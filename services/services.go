package services

import (
	"encoding/json"
	"fmt"
	etcdclient "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/clientv3/concurrency"
	"github.com/coreos/etcd/mvcc/mvccpb"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)
import (
	gp "github.com/titus12/ma-commons-go/services/pb-grpc"
	"github.com/titus12/ma-commons-go/utils"
)

const (
	DefaultTimeout = 10 * time.Second
	DefaultRetries = 1440 // failed connection retries (for every ten seconds)
)

const (
	eventOnDestroy = "OnDestroy"
	eventOnFatal   = "OnFatal"
)

// a kind of Service

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

// 初始化，在使用前必须调用
// root: 在etcd的根路径， 这个路径一定要以 / 开始, 实际上这个根路径是在 etcd 中 /root下的，比如参数root=/abc,那么就是/root/abc
// etcdHosts: etcd地址.
// serviceNames: 本身初始化打算支持的服务有哪些
// selfServiceName: 自已是什么服务
// selfNodeName: 自已节点的名称
// selfNodeAddr: 自已节点的ip地址
func Init(root string, etcdHosts, serviceNames []string, selfServiceName, selfNodeName, selfNodeAddr string) {
	once.Do(func() {
		_retryMgr.init()
		_defaultPool.init(root, etcdHosts, serviceNames, selfServiceName, selfNodeName, selfNodeAddr)
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

// 服务池，容纳所有服务
type servicePool struct {
	root            string
	selfNodeName    string
	selfServiceName string
	selfNodeAddr    string
	services        map[string]*Service
	knownNames      map[string]bool // Service provided
	namesProvided   bool
	client          *etcdclient.Client
	event           sync.Map // Service event notify
}

func (p *servicePool) init(root string, etcdHosts, serviceNames []string, selfServiceName, selfNodeName, selfNodeAddr string) {
	// init etcd node
	cfg := etcdclient.Config{
		Endpoints:   etcdHosts,
		DialTimeout: DefaultTimeout,
	}
	c, err := etcdclient.New(cfg)
	if err != nil {
		log.Panic(err)
		os.Exit(-1)
	}
	p.client = c
	p.root = "/root" + root

	//todo: 下面三个是准备都是全名称，还是简称
	p.selfServiceName = selfServiceName
	p.selfNodeName = selfNodeName
	p.selfNodeAddr = selfNodeAddr
	// init
	p.services = make(map[string]*Service)
	p.knownNames = make(map[string]bool)

	if len(serviceNames) > 0 {
		p.namesProvided = true
	}

	log.Infof("all Service serviceNames:%v", serviceNames)
	for _, v := range serviceNames {
		servicePath := joinPath(p.root, strings.TrimSpace(v))
		p.knownNames[servicePath] = true
		p.services[servicePath] = newService(v)
	}
}

// 工作
type work struct {
	jobGroup    chan *mvccpb.KeyValue
	resultGroup chan string
	job         func()
	jobsNum     int
}

// 向工作中添加任务
func (w *work) addJob(job *mvccpb.KeyValue) {
	w.jobGroup <- job
}

// 返回工作结果
func (w *work) result() string {
	return <-w.resultGroup
}

// 开始工作
func (w *work) start() {
	w.jobsNum = w.jobCount()
	worker := w.jobsNum/2 + 1
	for i := 0; i < worker; i++ {
		go w.job()
	}
}

// 在工作上进行等待
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
			log.Infof("start add Service node %v", key)
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

// 构建一个工作
func (p *servicePool) makeWork(queueSize int) (*work, func()) {
	w := &work{
		jobGroup:    make(chan *mvccpb.KeyValue, queueSize),
		resultGroup: make(chan string, queueSize),
	}

	job := func() {
		for job := range w.jobGroup {
			key := string(job.Key)
			if p.upsertNode(key, job.Value) {
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

// 开启一个服务的代理客户端，本身不做为服务
func (p *servicePool) startClient(ctx context.Context) error {
	w, cancel := p.makeWork(1024)
	defer cancel()
	defer func() {
		servicePath := joinPath(p.root, strings.TrimSpace(p.selfServiceName))
		if p.services[servicePath] == nil {
			nodePath := joinPath(servicePath, p.selfNodeName)
			go func() {
				err := p.watcher(nodePath)
				if err != nil {
					log.Fatalf("startClient watcher err %v", err)
				}
			}()
			if err := p.updateNodeData(nodePath, &nodeData{p.selfNodeAddr, ServiceStatusRunning}); err != nil {
				log.Fatalf("startClient updateNodeData %v %v err %v", nodePath, ServiceStatusRunning, err)
			}
		}
	}()
	servicePath := joinPath(p.root, strings.TrimSpace(p.selfServiceName))
	for k, _ := range p.services {
		if k == servicePath {
			continue
		}
		go func() {
			err := p.watcher(k)
			if err != nil {
				log.Fatalf("startClient watcher err %v", err)
			}
		}()

		if err := p.initNodesOfService(k, w); err != nil {
			return err
		}
	}
	w.start()
	return w.wait(ctx)
}

// 开启一个服务的服务器
func (p *servicePool) startServer(ctx context.Context, port int, startup func(*grpc.Server, *Service) error) {
	sw, err := NewServerWrapper(fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("startServer %v err %v", port, err)
	}
	w, cancel := p.makeWork(256)
	defer cancel()

	servicePath := joinPath(p.root, strings.TrimSpace(p.selfServiceName))
	mtx, err := p.newMutex(p.selfServiceName)
	if err != nil {
		log.Fatalf("lock Service err %v", err)
	}
	mtx.Lock(context.TODO())
	defer mtx.Unlock(context.TODO())

	if err != nil {
		log.Fatalf("startServer lock err:%v", err)
	}

	go func() {
		err := p.watcher(servicePath)
		if err != nil {
			log.Fatalf("startServer watcher err %v", err)
		}
	}()

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

	nodePath := joinPath(servicePath, p.selfNodeName)
	node := NewNode(p.selfNodeName, nil, nodeData{p.selfNodeAddr, ServiceStatusPending}, true, TransferStatusSucc)
	if err := service.addNode(node); err != nil {
		log.Fatalf("startServer upsertNode %v %v err %v", node.key, StatusServiceName[node.data.Status], err)
	}

	if err := p.updateNodeData(nodePath, &node.data); err != nil {
		log.Fatalf("startServer updateNodeData %v %v err %v", nodePath, ServiceStatusPending, err)
	}

	//loop check to Status ServiceStatusRunning
	log.Infof("startServer start to check whether nodes are ready")
	status := node.data.Status
	for {
		transfer := service.isCompleted(p.selfNodeName)
		switch transfer {
		case TransferStatusFail:
			node := NewNode(p.selfNodeName, nil, nodeData{p.selfNodeAddr, ServiceStatusStopping}, true, TransferStatusFail)
			if err := p.stopNode(nodePath, node); err != nil {
				log.Error("startServer updateNode stop %v %v err %v", node.key, StatusServiceName[status], err)
			}
			log.Errorf("startServer %v startup failure", p.selfNodeName)
			os.Exit(0)
		case TransferStatusSucc:
			node := NewNode(p.selfNodeName, nil, nodeData{p.selfNodeAddr, ServiceStatusRunning}, true, TransferStatusSucc)
			if err := service.updateNode(node); err != nil {
				log.Error("startServer updateNode running %v %v err %v", node.key, StatusServiceName[status], err)
			}
			log.Info("startServer transfer success")
			for {
				if err := p.updateNodeData(nodePath, &node.data); err != nil {
					log.Error("startServer updateNodeData %v %v err %v", nodePath, StatusServiceName[status], err)
				} else {
					goto StartServerDone
				}
			}
		case TransferStatusNone:
			log.Debugf("startServer waiting for transfer")
		}
		time.Sleep(100)
	}
StartServerDone:
	log.Infof("node %v startup completed", p.selfNodeName)
	sw.Wait()
}

// 停止一个节点
func (p *servicePool) stopNode(nodePath string, node *node) error {
	servicePath := getDir(nodePath)
	if p.namesProvided && !p.knownNames[servicePath] {
		return nil
	}
	if p.selfNodeName != node.key {
		return nil
	}
	service := p.services[servicePath]
	if err := service.updateNode(node); err != nil {
		return fmt.Errorf("stopNode stopping err %v", err)
	}

	for {
		if err := p.updateNodeData(nodePath, &node.data); err != nil {
			log.Error("startServer updateNodeData %v %v err %v", nodePath, StatusServiceName[node.data.Status], err)
		} else {
			break
		}
	}

	err := service.callback(node.key, node.data.Status)
	if err != nil {
		log.Errorf("stopNode callback %v err %v", node.key, err)
	}
	for {
		if err := p.RemoveNodeData(nodePath); err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	return nil
}

// 变更一个节点的牵移状态
func (p *servicePool) transfer(key string, status int32) error {
	servicePath := getDir(key)
	service := p.services[servicePath]
	nodeName := getFileName(key)
	if !service.transfer(nodeName, status) {
		return fmt.Errorf("transfer node %v not exist", nodeName)
	}
	return nil
}

// 监控etcd，监控一个服务的事件发生
func (p *servicePool) watcher(servicePath string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan *etcdclient.Event, 256)
	defer close(events)
	wc := p.client.Watch(ctx, servicePath, etcdclient.WithPrefix())
	if wc == nil {
		return fmt.Errorf("watcher no channel %v", servicePath)
	}
	go func() {
		for v := range events {
			func() {
				defer utils.PrintPanicStack()
				key := string(v.Kv.Key)
				if ok := p.upsertNode(key, v.Kv.Value); !ok {
					addRetry(key)
				}
			}()
		}
	}()
	for wresp := range wc {
		for _, ev := range wresp.Events {
			func(event *etcdclient.Event) {
				log.Debugf("watcher %v %v:%v", event.Type, string(event.Kv.Key), event.Kv.Value)
				defer utils.PrintPanicStack()
				switch event.Type {
				case etcdclient.EventTypePut:
					events <- event
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
	return nil
}

// 初始化所有节点
func (p *servicePool) initNodesOfService(servicePath string, w *work) error {
	kAPI := etcdclient.NewKV(p.client)
	// get the keys under directory
	log.Infof("initNodesOfService Service under:%v", servicePath)
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	resp, err := kAPI.Get(ctx, servicePath, etcdclient.WithPrefix())
	cancel()
	if err != nil {
		log.Error(err)
		return err
	}

	for _, ev := range resp.Kvs {
		// todo: 下面这里要多注意，检查一下其他地方是否有类似的，这里主要是跳过作为目录的key
		// todo： 比如这里servicePath本身是是一个目录，这类型key不能解析，否则会出错。
		keyStr := utils.BytesToString(ev.Key)
		if keyStr == servicePath {
			continue
		}
		info := &nodeData{}
		err := json.Unmarshal(ev.Value, info)
		if err != nil {
			return fmt.Errorf("initNodesOfService nodeData Parse value:%v, err:%v", string(ev.Key), err)
		}
		if info.Status == ServiceStatusNone {
			continue
		}
		w.addJob(ev)
	}
	log.Infof("initNodesOfService %v complete", servicePath)
	return nil
}

// 添加或者更新一个节点
func (p *servicePool) upsertNode(key string, value []byte) bool {
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
	if info.Status == ServiceStatusNone {
		return true
	}
	nodeName := getFileName(key)
	service := p.services[servicePath]
	// create Service connection
	if nodeName == p.selfNodeName {
		node := NewNode(nodeName, nil, *info, true, TransferStatusSucc)
		err = service.upsertNode(node)
		if err != nil {
			log.Errorf("upsertNode local %v - %v err %v", key, value, err)
			return true
		}
		log.Infof("upsertNode local %v - %v", key, value)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
		conn, err := grpc.DialContext(ctx, info.Addr, grpc.WithBlock())
		cancel()
		if err != nil {
			log.Errorf("upsertNode Service connect %v - %v, Error: %v", key, value, err)
			return false
		}
		node := NewNode(nodeName, conn, *info, false, 0)
		err = service.upsertNode(node)
		if err != nil {
			log.Errorf("upsertNode remote %v - %v err %v", key, value, err)
			return false
		}
		log.Infof("upsertNode remote %v - %v", key, value)
		if node.data.Status == ServiceStatusPending {
			err := service.callback(nodeName, node.data.Status)
			sendNode := &gp.Node{Name: key, Status: TransferStatusSucc}
			if err != nil {
				sendNode.Status = TransferStatusFail
				log.Errorf("upsertNode remote callback %v - %v err %v", key, value, err)
			}
			for {
				ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
				cli := gp.NewNodeServiceClient(conn)
				result, err := cli.Notify(ctx, sendNode)
				cancel()
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

func (p *servicePool) eventOnDestroy(nodeName string) {
	event, ok := p.event.Load(eventOnDestroy)
	if ok {
		destroy := event.(func(key string, status int8))
		destroy(nodeName, ServiceStatusRunning)
	}
}

// 删除节点
func (p *servicePool) removeNode(key string) {
	// name check
	nodeName := getFileName(key)
	servicePath := filepath.Dir(key)

	// check Service kind
	service := p.services[servicePath]
	if service != nil {
		service.delNode(nodeName)
	}
	if nodeName == p.selfNodeName {
		p.eventOnDestroy(nodeName)
	}
}

// 向etcd更新节点数据
func (p *servicePool) updateNodeData(nodePath string, nodeData *nodeData) error {
	servicePath := filepath.Dir(nodePath)
	servicePath = strings.ReplaceAll(servicePath, `\`, `/`)

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

// 向etcd移除节点数据
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

// provide a specific key for a Service, eg:
// path:/backends/game, id:game001
//
// the full cannonical path for this Service is:
// /backends/game/game001 172.168.0.1:8000
func (p *servicePool) getServiceWithId(servicePath string, id string) (node, error) {
	// check existence
	service := p.services[servicePath]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", servicePath)
	}
	return service.getNode(id)
}

// get a Service in round-robin style
// especially useful for load-balance with vars-less services
func (p *servicePool) getServiceWithRoundRobin(servicePath string) (node, error) {
	// check existence
	service := p.services[servicePath]
	if service == nil {
		return node{}, fmt.Errorf("service %v is nil", servicePath)
	}
	// get a Service in round-robind style
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
	del = p.upsertNode(key, resp.Kvs[0].Value)
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

func SyncStartService(ctx context.Context, port int, startup func(*grpc.Server, *Service) error) {
	_defaultPool.startServer(ctx, port, startup)
}

func transfer(key string, status int32) error {
	return _defaultPool.transfer(key, status)
}

func GetService(serviceName string) *Service {
	return _defaultPool.services[serviceName]
}

func GetServiceWithConsistentHash(serviceName string, key string) (bool, nodeData, *grpc.ClientConn, error) {
	node, err := _defaultPool.getServiceWithConsistentHash(joinPath(_defaultPool.root, serviceName), key)
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
