package services

import (
	context2 "context"
	_ "fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/titus12/ma-commons-go/utils/ctxfunc"

	etcdclient "github.com/coreos/etcd/client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	DEFAULT_TIMEOUT = 5 * time.Second
	DEFAULT_RETRIES = 6 // failed connection retries (for every ten seconds)
)

// a single connection
type client struct {
	key  string
	conn *grpc.ClientConn
}

// a kind of service
type service struct {
	clients []client
	idx     uint32 // for round-robin purpose
}

// all services
type servicePool struct {
	root           string
	services       map[string]*service
	known_names    map[string]bool
	names_provided bool
	client         etcdclient.Client
	callbacks      map[string][]chan string // service add callback notify
	mu             sync.RWMutex
}

// retries
type retryManager struct {
	retries map[string]int // key ==> retry times
	mu      sync.RWMutex
}

var (
	defaultPool servicePool
	retryMgr    retryManager
	once        sync.Once
)

// Init() ***MUST*** be called before using
func Init(root string, hosts, names []string) {
	once.Do(func() {
		retryMgr.init()
		defaultPool.init(root, hosts, names)
		timerStart()
	})
}

func (p *retryManager) init() {
	p.retries = make(map[string]int)
}

func (p *retryManager) addRetry(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.retries[key] = DEFAULT_RETRIES
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

func (p *servicePool) init(root string, hosts, names []string) {
	// init etcd client
	cfg := etcdclient.Config{
		Endpoints: hosts,
		Transport: etcdclient.DefaultTransport,
	}
	c, err := etcdclient.New(cfg)
	if err != nil {
		log.Panic(err)
		os.Exit(-1)
	}
	p.client = c
	p.root = root

	// init
	p.services = make(map[string]*service)
	p.known_names = make(map[string]bool)

	if len(names) > 0 {
		p.names_provided = true
	}

	log.Infof("all service names:%v", names)
	for _, v := range names {
		p.known_names[p.root+"/"+strings.TrimSpace(v)] = true
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

	var (
		resp *etcdclient.Response
		err  error
	)

	ctxfunc.Timeout1m(func(ctx context2.Context) {
		resp, err = kAPI.Get(ctx, directory, &etcdclient.GetOptions{Recursive: true})
	})
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
				p.addService(service.Key, service.Value)
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
			p.removeService(key)
			delRetry(key)
		}
	}
}

// add a service
func (p *servicePool) addService(key, value string) bool {
	// name check
	service_name := filepath.Dir(key)
	service_name = strings.ReplaceAll(service_name, "\\", "/")
	//fmt.Println("add:" , service_name)
	if p.names_provided && !p.known_names[service_name] {
		return true
	}

	// try new service kind init
	p.mu.Lock()
	if p.services[service_name] == nil {
		p.services[service_name] = &service{}
	}
	p.mu.Unlock()

	// create service connection
	if conn, err := grpc.Dial(value, grpc.WithBlock(), grpc.WithInsecure(), grpc.WithTimeout(DEFAULT_TIMEOUT)); err == nil {
		p.mu.Lock()
		defer p.mu.Unlock()
		service := p.services[service_name]
		service.clients = append(service.clients, client{key, conn})
		for k := range p.callbacks[service_name] {
			select {
			case p.callbacks[service_name][k] <- key:
			default:
			}
		}
		log.Infof("service added %v(%v)", key, value)
		return true
	} else {
		log.Errorf("service connect %v(%v), Error: %v", key, value, err)
	}

	return false
}

// remove a service
func (p *servicePool) removeService(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// name check
	service_name := filepath.Dir(key)
	if p.names_provided && !p.known_names[service_name] {
		return
	}

	// check service kind
	service := p.services[service_name]
	if service == nil {
		log.Errorf("service not exists: %v", service_name)
		return
	}

	// remove a service
	for k := range service.clients {
		if service.clients[k].key == key { // deletion
			service.clients[k].conn.Close()
			service.clients = append(service.clients[:k], service.clients[k+1:]...)
			log.Infof("service removed: %v", key)
			return
		}
	}
}

// provide a specific key for a service, eg:
// path:/backends/snowflake, id:s1
//
// the full cannonical path for this service is:
// 			/backends/snowflake/s1
func (p *servicePool) getServiceWithId(path string, id string) *grpc.ClientConn {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// check existence
	service := p.services[path]
	if service == nil {
		return nil
	}
	if len(service.clients) == 0 {
		return nil
	}

	// loop find a service with id
	fullpath := string(path) + "/" + id
	for k := range service.clients {
		if service.clients[k].key == fullpath {
			return service.clients[k].conn
		}
	}

	return nil
}

// get a service in round-robin style
// especially useful for load-balance with state-less services
func (p *servicePool) getService(path string) (conn *grpc.ClientConn, key string) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	// check existence
	service := p.services[path]
	if service == nil {
		return nil, ""
	}

	if len(service.clients) == 0 {
		return nil, ""
	}

	// get a service in round-robind style,
	idx := int(atomic.AddUint32(&service.idx, 1)) % len(service.clients)
	return service.clients[idx].conn, service.clients[idx].key
}

func (p *servicePool) getAllService(path string) (conns map[string]*grpc.ClientConn) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	service := p.services[path]
	if service == nil {
		return
	}

	if len(service.clients) == 0 {
		return
	}

	conns = make(map[string]*grpc.ClientConn)
	for _, v := range service.clients {
		conns[v.key] = v.conn
	}

	return
}

func (p *servicePool) registerCallback(path string, callback chan string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.callbacks == nil {
		p.callbacks = make(map[string][]chan string)
	}

	p.callbacks[path] = append(p.callbacks[path], callback)
	if s, ok := p.services[path]; ok {
		for k := range s.clients {
			callback <- s.clients[k].key
		}
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
	retryMgr.addRetry(key)
}

func delRetry(key string) {
	retryMgr.delRetry(key)
}

func retryConn(key string) bool {
	return defaultPool.retryConn(key)
}

func timerStart() {
	go func() {
		timer := time.NewTicker(10 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				retryMgr.cycleCheck()
			}
		}
	}()
}

/////////////////////////////////////////////////////////////////
// Wrappers

func GetService(path string) (*grpc.ClientConn, string) {
	conn, key := defaultPool.getService(defaultPool.root + "/" + path)
	return conn, key
}

func GetServiceWithId(path string, id string) *grpc.ClientConn {
	return defaultPool.getServiceWithId(defaultPool.root+"/"+path, id)
}

func AllService(path string) map[string]*grpc.ClientConn {
	return defaultPool.getAllService(defaultPool.root + "/" + path)
}

func RegisterCallback(path string, callback chan string) {
	defaultPool.registerCallback(defaultPool.root+"/"+path, callback)
}
