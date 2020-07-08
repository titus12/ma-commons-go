package services

import (
	"encoding/json"
	"fmt"
	_ "fmt"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/pkg/errors"
	"github.com/titus12/ma-commons-go/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	etcdclient "github.com/coreos/etcd/clientv3"
	log "github.com/sirupsen/logrus"
	gp "github.com/titus12/ma-commons-go/services/pb-grpc"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

const (
	DefaultTimeout = 10 * time.Second
	DefaultRetries = 1440 // failed connection retries (for every ten seconds)
)

const (
	eventOnDestroy = "OnDestroy"
	eventOnFatal   = "OnFatal"
)

var errStatusDuplicated = errors.New("checkStatus node status duplicated")

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
func Init(root string, hosts, serviceNames []string, selfServiceName, selfNodeName, selfNodeAddr string) {
	once.Do(func() {
		_retryMgr.init()
		_defaultPool.init(root, hosts, serviceNames, selfServiceName, selfNodeName, selfNodeAddr)
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
	selfNodeAddr    string
	services        map[string]*service
	knownNames      map[string]bool
	namesProvided   bool
	client          *etcdclient.Client
	event           sync.Map // service add callback notify
}

func (p *servicePool) init(root string, hosts, serviceNames []string, selfServiceName, selfNodeName, selfNodeAddr string) {
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
	p.selfNodeAddr = selfNodeAddr
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
			if p.upsertNode(key, job.Value, false) {
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
		log.Fatalf("startServer fail")
	}
	//startup func
	if err := startup(sw.gServer, service); err != nil {
		log.Fatalf("startup func err %v", err)
	}

	sw.Start()

	nodePath := joinPath(service.name, p.selfNodeName)
	node := NewNode(p.selfNodeName, nil, nodeData{p.selfNodeAddr, StatusServicePending}, true, StatusTransferSucc)
	if err := service.addNode(node); err != nil {
		log.Fatalf("startServer upsertNode %v %v err %v", node.key, StatusServiceName[node.data.status], err)
	}

	if err := p.updateNodeData(nodePath, &node.data); err != nil {
		log.Fatalf("startServer updateNodeData %v %v err %v", nodePath, StatusServicePending, err)
	}

	//loop check to status StatusServiceRunning
	log.Infof("startServer start to check whether nodes are ready")
	status := node.data.status
	for {
		switch status {
		case StatusServicePending:
			transfer := service.isCompleted(p.selfNodeName)
			if transfer == StatusTransferFail {
				status = StatusServiceStopping
				node := NewNode(p.selfNodeName, nil, nodeData{p.selfNodeAddr, StatusServiceStopping}, true, StatusTransferFail)
				if err := p.stopNode(nodePath, node); err != nil {
					log.Error("startServer updateNode stopping %v %v err %v", node.key, StatusServiceName[status], err)
				}
				log.Errorf("startServer transfer failed err %v", err)
			} else if transfer == StatusTransferSucc {
				status = StatusServiceRunning
				node := NewNode(p.selfNodeName, nil, nodeData{p.selfNodeAddr, StatusServiceRunning}, true, StatusTransferSucc)
				if err := service.updateNode(node); err != nil {
					log.Error("startServer updateNode running %v %v err %v", node.key, StatusServiceName[status], err)
				}
				log.Info("startServer transfer success")
			}
		case StatusServiceRunning, StatusServiceStopping:
			if err := p.updateNodeData(nodePath, &node.data); err != nil {
				log.Error("startServer updateNodeData %v %v err %v", nodePath, StatusServiceName[status], err)
			} else {
				goto StartServerDone
			}
		}
		time.Sleep(100)
	}
StartServerDone:
	log.Infof("node %v startup completed %v ", p.selfNodeName, StatusServiceName[status])
}

func (p *servicePool) stopNode(key string, node *node) error {
	servicePath := getDir(key)
	if p.namesProvided && !p.knownNames[servicePath] {
		return nil
	}
	service := p.services[servicePath]
	if err := service.updateNode(node); err != nil {
		return fmt.Errorf("stopNode stopping err %v", err)
	}
	if node.data.status == StatusServiceStopping {
		err := service.callback(node.key, int32(node.data.status))
		if err != nil {
			log.Errorf("stopNode callback %v err %v", node.key, err)
		}
		for {
			if err := p.RemoveNodeData(key); err == nil {
				event, ok := p.event.Load(eventOnDestroy)
				if ok {
					destroy := event.(func(key string, status int8))
					destroy(node.key, node.data.status)
				}
				os.Exit(0)
				break
			}
			time.Sleep(time.Second)
		}
	}
	return nil
}

func (p *servicePool) transfer(key string, status int32) error {
	servicePath := getDir(key)
	service := p.services[servicePath]
	nodeName := getFileName(key)
	if !service.transfer(nodeName, status) {
		return fmt.Errorf("transfer node %v not exist", nodeName)
	}
	return nil
}

// watcher for data change in etcd directory
func (p *servicePool) watcher(serviceName string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for {
		servicePath := joinPath(p.root, strings.TrimSpace(serviceName))
		wc := p.client.Watch(ctx, servicePath, etcdclient.WithPrefix())
		if wc == nil {
			log.Errorf("watcher no channel %v", servicePath)
			continue
		}
		for wresp := range wc {
			for _, ev := range wresp.Events {
				func(event *etcdclient.Event) {
					log.Debugf("watcher %v %v:%v", event.Type, string(event.Kv.Key), event.Kv.Value)
					utils.PrintPanicStack()
					switch event.Type {
					case etcdclient.EventTypePut:
						key := string(event.Kv.Key)
						if ok := p.upsertNode(key, event.Kv.Value, true); !ok {
							addRetry(key)
						}
					case etcdclient.EventTypeDelete:
						key := string(event.PrevKv.Key)
						p.removeNode(key)
						delRetry(key)
					default:
						log.Errorf("watcher event %v kv %v", event.Type, event.Kv)
					}
				}(ev)
			}
		}
	}
}

// connect to all nodes of the service
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
	}
	log.Infof("initNodesOfService %v complete", servicePath)
	return nil
}

// add a node
func (p *servicePool) upsertNode(key string, value []byte, isCallback bool) bool {
	servicePath := getDir(key)
	if p.namesProvided && !p.knownNames[servicePath] {
		return true
	}

	info := &nodeData{}
	err := json.Unmarshal(value, info)
	if err != nil {
		log.Errorf("upsertNode nodeData Parse value:%v, err:%v", value, err)
		return false
	}
	if info.status == StatusServiceNone {
		return true
	}
	nodeName := getFileName(key)
	service := p.services[servicePath]
	// create service connection
	if nodeName == p.selfNodeName {
		node := NewNode(nodeName, nil, *info, true, StatusTransferSucc)
		err = service.upsertNode(node)
		if err == errStatusDuplicated {
			return true
		}
		if err != nil {
			log.Errorf("upsertNode local %v - %v err %v", key, value, err)
			return true
		}
		log.Infof("upsertNode local %v - %v", key, value)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		conn, err := grpc.DialContext(ctx, info.addr, grpc.WithBlock())
		cancel()
		if err != nil {
			log.Errorf("upsertNode service connect %v - %v, Error: %v", key, value, err)
			return false
		}
		node := NewNode(nodeName, conn, *info, false, 0)
		err = service.upsertNode(node)
		if err != nil {
			log.Errorf("upsertNode remote %v - %v err %v", key, value, err)
			return false
		}
		log.Infof("upsertNode remote %v - %v", key, value)
		if node.data.status == StatusServicePending {
			err := service.callback(nodeName, int32(node.data.status))
			sendNode := &gp.Node{Name: key, Status: StatusTransferSucc}
			if err != nil {
				sendNode.Status = StatusTransferFail
				log.Errorf("upsertNode remote callback %v - %v err %v", key, value, err)
			}
			for {
				cli := gp.NewNodeServiceClient(conn)
				result, err := cli.Notify(context.Background(), sendNode)
				if err != nil {
					log.Errorf("upsertNode remote Notify %v - %v err %v", key, value, err)
				} else {
					if result.ErrorCode == 0 {
						log.Infof("upsertNode remote Notify %v - %v succ", key, value)
						break
					} else {
						log.Errorf("upsertNode remote Notify %v - %v receive result %v", key, value, result)
					}
				}
				time.Sleep(time.Second * 1)
			}
		}
	}
	return true
}

// remove a node
func (p *servicePool) removeNode(key string) {
	// name check
	servicePath := filepath.Dir(key)
	if p.namesProvided && !p.knownNames[servicePath] {
		return
	}

	// check service kind
	service := p.services[servicePath]
	if service == nil {
		log.Errorf("removeNode service not exists: %v", servicePath)
		return
	}
	nodeName := getFileName(key)
	// remove a node
	service.delNode(nodeName)
}

func (p *servicePool) updateNodeData(nodePath string, nodeData *nodeData) error {
	servicePath := filepath.Dir(nodePath)
	if p.namesProvided && !p.knownNames[servicePath] {
		return nil
	}
	data, err := json.Marshal(nodeData)
	if err != nil {
		return fmt.Errorf("updateNodeData nodeData Marshal value:%v, err:%v", data, err)
	}
	kAPI := etcdclient.NewKV(p.client)
	// put the keys under directory
	log.Infof("updateNodeData under %v %v", nodePath, nodeData)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	_, err = kAPI.Put(ctx, nodePath, string(data))
	cancel()
	return err
}

func (p *servicePool) RemoveNodeData(nodePath string) error {
	servicePath := filepath.Dir(nodePath)
	if p.namesProvided && !p.knownNames[servicePath] {
		return nil
	}
	kAPI := etcdclient.NewKV(p.client)
	// put the keys under directory
	log.Infof("RemoveNodeData under %v", nodePath)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	_, err := kAPI.Delete(ctx, nodePath)
	cancel()
	return err
}

// provide a specific key for a service, eg:
// path:/backends/game, id:game001
//
// the full cannonical path for this service is:
// /backends/game/game001 172.168.0.1:8000
func (p *servicePool) getServiceWithId(servicePath string, id string) (node, error) {
	// check existence
	service := p.services[servicePath]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", servicePath)
	}
	return service.getNode(id)
}

// get a service in round-robin style
// especially useful for load-balance with vars-less services
func (p *servicePool) getServiceWithRoundRobin(servicePath string) (node, error) {
	// check existence
	service := p.services[servicePath]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", servicePath)
	}
	// get a service in round-robind style
	return service.getNodeWithRoundRobin()
}

func (p *servicePool) getServiceWithHash(servicePath string, hash int) (node, error) {
	service := p.services[servicePath]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", servicePath)
	}
	return service.getNodeWithHash(hash)
}

func (p *servicePool) getServiceWithConsistentHash(servicePath string, key string) (node, error) {
	service := p.services[servicePath]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", servicePath)
	}
	return service.getNodeWithConsistentHash(key, true)
}

func (p *servicePool) getServices(servicePath string) ([]node, error) {
	service := p.services[servicePath]
	if service == nil {
		return nil, fmt.Errorf("service %v is nil", servicePath)
	}
	return service.getNodes(), nil
}

func (p *servicePool) registerCallback(path string, callback func(key string, status int8)) {
	_, ok := p.event.LoadOrStore(path, callback)
	if ok {
		log.Errorf("register event callback on: %v duplicated", path)
		return
	}
	log.Infof("register event callback on: %v", path)
}

func (p *servicePool) retryConn(key string) (del bool) {
	kAPI := etcdclient.NewKV(p.client)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	resp, err := kAPI.Get(ctx, key)
	cancel()
	if err != nil || len(resp.Kvs) == 0 {
		del = true
		log.Errorf("retryConn err %v", err)
		return
	}
	del = p.upsertNode(key, resp.Kvs[0].Value, true)
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

func transfer(key string, status int32) error {
	return _defaultPool.transfer(key, status)
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

func GetServices(path string) ([]node, error) {
	return _defaultPool.getServices(joinPath(_defaultPool.root, path))
}

func RegisterDestroyEvent(callback func(key string, status int8)) {
	_defaultPool.registerCallback(eventOnDestroy, callback)
}
func RegisterFatalEvent(callback func(key string, status int8)) {
	_defaultPool.registerCallback(eventOnFatal, callback)
}
