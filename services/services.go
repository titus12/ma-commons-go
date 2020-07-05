package services

import (
	"encoding/json"
	"fmt"
	_ "fmt"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/titus12/ma-commons-go/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	etcdclient "github.com/coreos/etcd/clientv3"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	DefaultTimeout = 10 * time.Second
	DefaultRetries = 1440 // failed connection retries (for every ten seconds)
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
func Init(root string, hosts, serviceNames []string, selfServiceName, selfName string) {
	once.Do(func() {
		_retryMgr.init()
		_defaultPool.init(root, hosts, serviceNames, selfServiceName, selfName)
		timerStart()
	})
}

func (p *retryManager) init() {
	p.retries = make(map[string]int)
}

func joinPath(params ...string) string {
	return strings.Join(params, "/")
}

func getDir(path string) string {
	dir := filepath.Dir(path)
	return strings.ReplaceAll(dir, "\\", "/")
}

func getFileName(path string) string {
	dir := filepath.Base(path)
	return dir
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
	defer utils.PrintPanicStack()
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
	root            string
	selfNodeName    string
	selfServiceName string
	services        map[string]*service
	knownNames      map[string]bool
	namesProvided   bool
	client          *etcdclient.Client
	callbacks       sync.Map // service add callback notify
}

func (p *servicePool) init(root string, hosts, serviceNames []string, selfServiceName, selfNodeName string) {
	// init etcd node
	cfg := etcdclient.Config{
		Endpoints:   hosts,
		DialTimeout: DefaultTimeout,
	}
	c, err := etcdclient.New(cfg)
	if err != nil {
		log.Panic(err)
		os.Exit(-1)
	}
	p.client = c
	p.root = "/root" + root
	p.selfServiceName = selfServiceName
	p.selfNodeName = selfNodeName
	// init
	p.services = make(map[string]*service)
	p.knownNames = make(map[string]bool)

	if len(serviceNames) > 0 {
		p.namesProvided = true
	}

	log.Infof("all service serviceNames:%v", serviceNames)
	for _, v := range serviceNames {
		servicePath := joinPath(p.root, strings.TrimSpace(v))
		p.knownNames[servicePath] = true
		p.services[servicePath] = newService(v)
	}
}

type work struct {
	jobGroup    chan *mvccpb.KeyValue
	resultGroup chan string
	job         func()
	jobsNum     int
}

func (w *work) addJob(job *mvccpb.KeyValue) {
	w.jobGroup <- job
}

func (w *work) result() string {
	return <-w.resultGroup
}

func (w *work) start() {
	w.jobsNum = w.jobCount()
	worker := w.jobsNum/2 + 1
	for i := 0; i < worker; i++ {
		go w.job()
	}
}

func (w *work) wait(ctx context.Context) error {
	if w.jobsNum <= 0 {
		return nil
	}
	for {
		select {
		case key, ok := <-w.resultGroup:
			if !ok {
				return fmt.Errorf("start resultServiceJob close")
			}
			log.Infof("start add service node %v", key)
			if w.jobsNum--; w.jobsNum <= 0 {
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (w *work) jobCount() int {
	return len(w.jobGroup)
}

func (p *servicePool) makeWork(queueSize int) (*work, func()) {
	w := &work{
		jobGroup:    make(chan *mvccpb.KeyValue, queueSize),
		resultGroup: make(chan string, queueSize),
	}

	job := func() {
		for job := range w.jobGroup {
			key := string(job.Key)
			if p.addService(key, job.Value, false) {
				w.jobGroup <- job
				continue
			}
			w.resultGroup <- key
		}
	}

	w.job = job

	return w, func() {
		close(w.jobGroup)
		close(w.resultGroup)
	}
}

func (p *servicePool) startClient(ctx context.Context) error {
	w, cancel := p.makeWork(1024)
	defer cancel()
	for k, _ := range p.services {
		if k == p.selfServiceName {
			continue
		}
		go p.watcher(k)
		if err := p.initNodesOfService(k, w); err != nil {
			return err
		}
	}
	w.start()
	return w.wait(ctx)
}

func (p *servicePool) startServer(ctx context.Context, port int, startup func(*grpc.Server, *service) error) {
	sw, err := NewServerWrapper(fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("startServer %v err %v", port, err)
	}
	w, cancel := p.makeWork(256)
	defer cancel()

	servicePath := joinPath(p.root, strings.TrimSpace(p.selfServiceName))
	mtx, err := p.newMutex(p.selfServiceName)
	if err != nil {
		log.Fatalf("lock service err %v", err)
	}
	mtx.Lock(context.TODO())
	defer mtx.Unlock(context.TODO())

	if err != nil {
		log.Fatalf("startServer lock err:%v", err)
	}
	go p.watcher(servicePath)

	if err := p.initNodesOfService(servicePath, w); err != nil {
		log.Fatalf("startServer initNodesOfService err:%v", err)
	}

	service := p.services[servicePath]
	w.start()

	err = w.wait(ctx)
	if err != nil {
		log.WithError(err).Fatalf("startServer fail")
	}
	//startup func
	if err := startup(sw.gServer, service); err != nil {
		log.Fatalf("startup func err %v", err)
	}

	sw.Start()

	nodePath := joinPath(service.name, p.selfNodeName)
	if err := p.updateNode(nodePath, StatusServicePending); err != nil {
		log.Fatalf("updateNode %v %v err %v", nodePath, StatusServicePending, err)
	}

	//loop check to status StatusServiceRunning
	log.Infof("start to check whether nodes are ready")
	for {
		completed := service.isCompleted(p.selfNodeName)
		if completed {
			if err := p.updateNode(nodePath, StatusServiceRunning); err != nil {
				log.Fatalf("updateNode %v %v err %v", nodePath, StatusServiceRunning, err)
			}
		}
		time.Sleep(100)
	}
	log.Infof("node %v startup completed", p.selfNodeName)
}

func (p *servicePool) isCompleted() bool {
	dirPath := joinPath(p.root, strings.TrimSpace(p.selfServiceName))
	service := p.services[dirPath]
	return service.isCompleted(p.selfNodeName)
}

// watcher for data change in etcd directory
func (p *servicePool) watcher(serviceName string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		servicePath := joinPath(p.root, strings.TrimSpace(serviceName))
		wc := p.client.Watch(ctx, servicePath, etcdclient.WithPrefix())
		if wc == nil {
			log.Errorf("no watcher channel %v", servicePath)
			continue
		}
		for wresp := range wc {
			for _, ev := range wresp.Events {
				log.Debugf("watcher %v %v:%v", ev.Type, string(ev.Kv.Key), ev.Kv.Value)
				switch ev.Type {
				case etcdclient.EventTypePut:
					key := string(ev.Kv.Key)
					if ok := p.addService(key, ev.Kv.Value, true); !ok {
						addRetry(key)
					}
				case etcdclient.EventTypeDelete:
					key := string(ev.PrevKv.Key)
					p.removeNode(key)
					delRetry(key)
				default:
					log.Errorf("watch event %v kv %v", ev.Type, ev.Kv)
				}
			}
		}
	}
}

// connect to all services
func (p *servicePool) initNodesOfService(servicePath string, w *work) error {
	kAPI := etcdclient.NewKV(p.client)
	// get the keys under directory
	log.Infof("initNodesOfService service under:%v", servicePath)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	resp, err := kAPI.Get(ctx, servicePath, etcdclient.WithPrefix())
	cancel()
	if err != nil {
		log.Error(err)
		return err
	}

	for _, ev := range resp.Kvs {
		info := &nodeData{}
		err := json.Unmarshal(ev.Value, info)
		if err != nil {
			return fmt.Errorf("initNodesOfService nodeData Parse value:%v, err:%v", string(ev.Key), err)
		}
		if info.status == StatusServiceNone {
			continue
		}
		w.addJob(ev)
		//ch <- *ev
		/*path := string(ev.Key)
		if ok := p.addService(path, ev.Value); !ok {
			addRetry(path)
		}
		fmt.Printf("%s : %s\n", path, ev.Value)*/
	}
	log.Infof("initNodesOfService %v complete", servicePath)
	return nil
}

// add a service
func (p *servicePool) addService(key string, value []byte, isCallback bool) bool {
	// name check
	dirPath := getDir(key)
	if p.namesProvided && !p.knownNames[dirPath] {
		return true
	}

	info := &nodeData{}
	err := json.Unmarshal(value, info)
	if err != nil {
		log.Errorf("addService nodeData Parse value:%v, err:%v", value, err)
		return false
	}
	if info.status == StatusServiceNone {
		return true
	}
	nodeName := getFileName(key)
	// create service connection
	if nodeName == p.selfNodeName {
		service := p.services[dirPath]
		node := node{nodeName, nil, *info, true, false}
		err = service.upsertNode(node)
		if err != nil {
			log.Errorf("addService local %v - %v err %v", key, value, err)
			return false
		}
		log.Infof("addService local %v - %v", key, value)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		conn, err := grpc.DialContext(ctx, info.addr, grpc.WithBlock())
		cancel()
		if err != nil {
			log.Errorf("service connect %v - %v, Error: %v", key, value, err)
			return false
		}
		service := p.services[dirPath]
		node := node{nodeName, conn, *info, false, false}
		err = service.upsertNode(node)
		if err != nil {
			log.Errorf("addService remote %v - %v err %v", key, value, err)
			return false
		}
		log.Infof("addService remote %v - %v", key, value)
	}
	if isCallback {
		if callback, ok := p.callbacks.Load(nodeName); ok {
			call := callback.(func(key string, status int8))
			call(nodeName, info.status)
		}
	}
	return true
}

// remove a service
func (p *servicePool) removeNode(key string) {
	// name check
	dirPath := filepath.Dir(key)
	if p.namesProvided && !p.knownNames[dirPath] {
		return
	}

	// check service kind
	service := p.services[dirPath]
	if service == nil {
		log.Errorf("service not exists: %v", dirPath)
		return
	}
	nodeName := getFileName(key)
	// remove a node
	service.delNode(nodeName)
}

func (p *servicePool) updateNode(nodePath string, status int8) error {
	dirPath := filepath.Dir(nodePath)
	if p.namesProvided && !p.knownNames[dirPath] {
		return nil
	}
	nodeName := getFileName(nodePath)
	service := p.services[dirPath]
	node, err := service.getNode(nodeName)
	if err != nil {
		return fmt.Errorf("updateNode err %v", err)
	}
	data, err := json.Marshal(node.data)
	if err != nil {
		return fmt.Errorf("updateNode nodeData Marshal value:%v, err:%v", node.data, err)
	}
	kAPI := etcdclient.NewKV(p.client)
	// put the keys under directory
	log.Infof("updateNode under:%v", nodePath)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	_, err = kAPI.Put(ctx, nodePath, string(data))
	cancel()
	return err
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
	return service.getNode(id)
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
	kAPI := etcdclient.NewKV(p.client)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	resp, err := kAPI.Get(ctx, key)
	cancel()
	if err != nil || len(resp.Kvs) == 0 {
		del = true
		log.Error(err)
		return
	}

	del = p.addService(key, resp.Kvs[0].Value, true)
	return
}

func (p *servicePool) newMutex(serviceName string) (*concurrency.Mutex, error) {
	sess, err := concurrency.NewSession(_defaultPool.client)
	if err != nil {
		return nil, err
	}
	defer sess.Close()
	m1 := concurrency.NewMutex(sess, "/lock-"+serviceName)
	return m1, nil
}

func (p *servicePool) lockDo(serviceName string, f func(string)) error {
	sess, err := concurrency.NewSession(_defaultPool.client)
	if err != nil {
		return err
	}
	defer sess.Close()
	m1 := concurrency.NewMutex(sess, "/lock-"+serviceName)
	err = m1.Lock(context.TODO())
	if err != nil {
		return err
	}
	defer m1.Unlock(context.TODO())
	f(serviceName)
	return nil
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

func SyncStartClient(ctx context.Context) error {
	return _defaultPool.startClient(ctx)
}

func SyncStartService(ctx context.Context, port int, startup func(*grpc.Server, *service) error) {
	_defaultPool.startServer(ctx, port, startup)
}

func GetService(serviceName string) *service {
	return _defaultPool.services[serviceName]
}

func GetServiceWithConsistentHash(servieName string, key string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithConsistentHash(joinPath(_defaultPool.root, servieName), key)
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func getServiceWithRoundRobin(serviceName string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithRoundRobin(joinPath(_defaultPool.root, serviceName))
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func GetServiceWithId(serviceName string, id string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithId(joinPath(_defaultPool.root, serviceName), id)
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func GetServiceWithHash(serviceName string, hash int) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithHash(joinPath(_defaultPool.root, serviceName), hash)
	if err != nil {
		return false, nodeData{}, nil, err
	}
	return node.isLocal, node.data, node.conn, nil
}

func AllService(path string) ([]node, error) {
	return _defaultPool.getServices(joinPath(_defaultPool.root, path))
}

func RegisterCallback(path string, callback func(key string, status int8)) {
	_defaultPool.registerCallback(joinPath(_defaultPool.root, path), callback)
}
