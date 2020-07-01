package services

import (
	"encoding/json"
	"fmt"
	_ "fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	etcdclient "github.com/coreos/etcd/client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	DefaultTimeout = 10 * time.Second
	DefaultRetries = 6 // failed connection retries (for every ten seconds)
)

// a kind of service

// retries
type retryManager struct {
	retries map[string]int // key ==> retry times
	mu      sync.RWMutex
}

var (
	_defaultPool servicePool
	_retryMgr    retryManager
	once         sync.Once
)

// Init() ***MUST*** be called before using
func Init(root string, hosts, names []string, self string) {
	once.Do(func() {
		_retryMgr.init()
		_defaultPool.init(root, hosts, names, self)
		timerStart()
	})
}

func (p *retryManager) init() {
	p.retries = make(map[string]int)
}

func pathJoin(params ...string) string {
	return strings.Join(params, "/")
}

func pathDir(path string) string {
	dir := filepath.Dir(path)
	return strings.ReplaceAll(dir, "\\", "/")
}

func (p *retryManager) addRetry(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retries[key] = DefaultRetries
	log.Debugf("Add connect retry:%v", key)
}

func (p *retryManager) delRetry(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.retries[key]
	if ok {
		delete(p.retries, key)
		log.Debugf("Del connect retry:%v", key)
	}
}

func (p *retryManager) cycleCheck() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, value := range p.retries {
		if value > 0 {
			log.Debugf("Trying connecting:%v ......", key)
			if del := retryConn(key); del == true {
				p.retries[key] = 0
				log.Infof("Retry connecting on %v successfully !", key)
			} else {
				p.retries[key]--
			}
		}

		if p.retries[key] == 0 {
			delete(p.retries, key)
			log.Debugf("Delete retry connect on %v", key)
		}
	}
}

// all services
type servicePool struct {
	root          string
	self          string
	services      map[string]*service
	knownNames    map[string]bool
	namesProvided bool
	client        etcdclient.Client
	callbacks     sync.Map // service add callback notify
}

func (p *servicePool) init(root string, hosts, serviceNames []string, self string) {
	// init etcd node
	cfg := etcdclient.Config{
		Endpoints:               hosts,
		Transport:               etcdclient.DefaultTransport,
		HeaderTimeoutPerRequest: time.Second,
	}
	c, err := etcdclient.New(cfg)
	if err != nil {
		log.Panic(err)
		os.Exit(-1)
	}
	p.client = c
	p.root = root
	p.self = self
	// init
	p.services = make(map[string]*service)
	p.knownNames = make(map[string]bool)

	if len(serviceNames) > 0 {
		p.namesProvided = true
	}

	log.Infof("all service serviceNames:%v", serviceNames)
	for _, v := range serviceNames {
		p.knownNames[pathJoin(p.root, strings.TrimSpace(v))] = true
		p.services[v] = newService()
		//fmt.Println("init:" ,p.root+"/"+strings.TrimSpace(v))
	}

	// start connection
	p.connectAll(p.root)
}

// get stored service name
func (p *servicePool) loadNames(filepath string) []string {
	kAPI := etcdclient.NewKeysAPI(p.client)
	// get the keys under directory
	log.Infof("reading names:%v", filepath)
	resp, err := kAPI.Get(context.Background(), filepath, nil)
	if err != nil {
		log.Error(err)
		return nil
	}

	// validation check
	if resp.Node.Dir {
		log.Error("names is not a file")
		return nil
	}

	// split names
	return strings.Split(resp.Node.Value, "\n")
}

// connect to all services
func (p *servicePool) connectAll(directory string) {
	kAPI := etcdclient.NewKeysAPI(p.client)
	// get the keys under directory
	log.Infof("connecting services under:%v", directory)
	ctx, _ := context.WithTimeout(context.Background(), DefaultTimeout)
	resp, err := kAPI.Get(ctx, directory, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		log.Error(err)
		return
	}

	// validation check
	if !resp.Node.Dir {
		log.Errorf("node %v not a directory", directory)
		return
	}

	// do not need to wait for exists connections complete
	go p.watcher()

	for _, node := range resp.Node.Nodes {
		if node.Dir { // service directory
			for _, service := range node.Nodes {
				if ok := p.addService(service.Key, service.Value); !ok {
					addRetry(service.Key)
				}
			}
		}
	}
	log.Info("services add complete")
}

// watcher for data change in etcd directory
func (p *servicePool) watcher() {
	kAPI := etcdclient.NewKeysAPI(p.client)
	w := kAPI.Watcher(p.root, &etcdclient.WatcherOptions{Recursive: true})
	for {
		resp, err := w.Next(context.Background())
		if err != nil {
			log.Error(err)
			continue
		}
		if resp.Node.Dir {
			continue
		}

		//log.Debugf("Watcher: %v %v %v", resp.Action, resp.Node.Key, resp.Node.Value)
		switch resp.Action {
		case "set", "create", "update", "compareAndSwap":
			if ok := p.addService(resp.Node.Key, resp.Node.Value); !ok {
				addRetry(resp.Node.Key)
			}
		case "delete":
			key := resp.PrevNode.Key
			p.removeNode(key)
			delRetry(key)
		}
	}
}

// add a service
func (p *servicePool) addService(key, value string) bool {
	// name check
	serviceName := pathDir(key)
	if p.namesProvided && !p.knownNames[serviceName] {
		return true
	}

	info := &nodeData{}
	err := json.Unmarshal([]byte(value), info)
	if err != nil {
		log.Errorf("addService nodeData Parse value:%v, err:%v", value, err)
		return false
	}
	// create service connection
	if key == p.self {
		service := p.services[serviceName]
		node := node{key, nil, *info, true}
		service.addNode(node)
		if callback, ok := p.callbacks.Load(key); ok {
			call := callback.(func(key string, status int8))
			call(key, info.status)
		}
		log.Infof("local service added %v(%v)", key, value)
		return true
	} else {
		ctx, _ := context.WithTimeout(context.Background(), DefaultTimeout)
		if conn, err := grpc.DialContext(ctx, value, grpc.WithBlock()); err == nil {
			service := p.services[serviceName]
			node := node{key, conn, *info, false}
			service.addNode(node)
			if callback, ok := p.callbacks.Load(key); ok {
				call := callback.(func(key string, status int8))
				call(key, info.status)
			}
			log.Infof("service added %v(%v)", key, value)
			return true
		} else {
			log.Errorf("service connect %v(%v), Error: %v", key, value, err)
		}
	}

	return false
}

// remove a service
func (p *servicePool) removeNode(key string) {
	// name check
	serviceName := filepath.Dir(key)
	if p.namesProvided && !p.knownNames[serviceName] {
		return
	}

	// check service kind
	service := p.services[serviceName]
	if service == nil {
		log.Errorf("service not exists: %v", serviceName)
		return
	}

	// remove a node
	service.delNode(key)
}

// provide a specific key for a service, eg:
// path:/backends/game, id:game001
//
// the full cannonical path for this service is:
// /backends/game/game001 172.168.0.1:8000
func (p *servicePool) getServiceWithId(path string, id string) (node, error) {
	// check existence
	service := p.services[path]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", path)
	}
	return service.getNode(path, id)
}

// get a service in round-robin style
// especially useful for load-balance with vars-less services
func (p *servicePool) getServiceWithRoundRobin(path string) (node, error) {
	// check existence
	service := p.services[path]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", path)
	}
	// get a service in round-robind style,
	return service.getNodeWithRoundRobin(path)
}

func (p *servicePool) getServiceWithHash(path string, hash int) (node, error) {
	service := p.services[path]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", path)
	}
	return service.getNodeWithHash(path, hash)
}

func (p *servicePool) getServiceWithConsistentHash(path string, key string) (node, error) {
	service := p.services[path]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", path)
	}
	return service.getNodeWithConsistentHash(path, key)
}

func (p *servicePool) getServices(path string) ([]node, error) {
	service := p.services[path]
	if service == nil {
		return nil, fmt.Errorf("service %v is nil", path)
	}
	return service.getNodes(path), nil
}

func (p *servicePool) registerCallback(path string, callback func(key string, status int8)) {
	_, ok := p.callbacks.LoadOrStore(path, callback)
	if ok {
		log.Errorf("register callback on: %v duplicated", path)
		return
	}
	log.Infof("register callback on: %v", path)
}

func (p *servicePool) retryConn(key string) (del bool) {
	kAPI := etcdclient.NewKeysAPI(p.client)
	resp, err := kAPI.Get(context.Background(), key, nil)
	if err != nil {
		del = true
		log.Error(err)
		return
	}

	if resp.Node.Dir {
		del = true
		log.Errorf("%v is not a node", key)
		return
	}

	del = p.addService(key, resp.Node.Value)
	return
}

/////////////////////////////////////////////////////////////////
func addRetry(key string) {
	_retryMgr.addRetry(key)
}

func delRetry(key string) {
	_retryMgr.delRetry(key)
}

func retryConn(key string) bool {
	return _defaultPool.retryConn(key)
}

func timerStart() {
	go func() {
		timer := time.NewTicker(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				_retryMgr.cycleCheck()
			}
		}
	}()
}

/////////////////////////////////////////////////////////////////
// Wrappers

func GetServiceWithConsistentHash(servieName string, key string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithConsistentHash(pathJoin(_defaultPool.root, servieName), key)
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func getServiceWithRoundRobin(servieName string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithRoundRobin(pathJoin(_defaultPool.root, servieName))
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func GetServiceWithId(path string, id string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithId(pathJoin(_defaultPool.root, path), id)
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func GetServiceWithHash(path string, hash int) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithHash(pathJoin(_defaultPool.root, path), hash)
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func AllService(path string) ([]node, error) {
	return _defaultPool.getServices(pathJoin(_defaultPool.root, path))
}

func RegisterCallback(path string, callback func(key string, status int8)) {
	_defaultPool.registerCallback(pathJoin(_defaultPool.root, path), callback)
}
