package cluster

import (
	"container/list"
	"reflect"
	"sync"
	"time"

	"github.com/titus12/ma-commons-go/utils"

	"github.com/titus12/ma-commons-go/utils/diectrl"

	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/discovery"
)

const (
	ErrRetryNum       = 20              // 错误重试次数
	ErrDelay          = 2               // 错误延迟
	DialRetryNum      = 3               // GRPC拔号重试次数
	DialRetryInterval = 1 * time.Second // grpc拔号重试间隔
	DialTimeOut       = 3 * time.Second // grpc的拔号超时时间

	StopCloseRetryNum = 3 //集群停止时关闭节点失败后的重试次数
)

// 服务的配置
// 注意：Name保证服务是唯一的, CliFaceBuildFn 是grpc的一个New接口的方法，具体如下:
//       比如我们protobuf声明了 test.WaiterClient 的接口，那么在test包下一定会有
//       一个NewWaiterClient(*grpc.ClientConn) WaiterClient 的方法，我们传递给
//       CliFaceBuildFn的就是这个方法指针，这里go没有提供任何可以检查是否传递正确
//       的检测方法，使用都需要严格按规范使用
type ServiceConfig struct {
	Name           string      //服务名称
	CliFaceBuildFn interface{} //客户调用接口生成方法（grpc都会有一个New接口的方法)
}

// 错误节点信息, 每次监听到节点发生变化，并且变化是关闭节点时，如果关闭时发生错误，则
// 会形成错误信息，以便后续执行错误重试处理，这里的重试不会发生在节点变更调用的goroutine
// 上，而是会有一条专门的goroutine来处理，所谓的节点关闭重点是关闭grpc连接，释放grpc资
// 源
type errNodeInfo struct {
	nodeCli  *NodeClient //节点客户端
	retryNum int         // 重试次数
}

// 错误节点列表
type errNodes struct {
	*sync.Mutex
	*list.List
}

func (en *errNodes) init() {
	en.Mutex = &sync.Mutex{}
	en.List = list.New()
}

// 弹出错误信息
func (en *errNodes) poperr() []*errNodeInfo {
	en.Lock()
	defer en.Unlock()
	if en.List == nil {
		return nil
	}

	el := en.Front()
	if el != nil {
		en.Remove(el)
		if el.Value != nil {
			return el.Value.([]*errNodeInfo)
		}
	}
	return nil
}

// 放错误信息，延迟后会弹出，延迟最小计量单位秒
func (en *errNodes) pusherr(errInfo *errNodeInfo, delay int) {
	en.Lock()
	defer en.Unlock()

	if en.List == nil {
		return
	}

	el := en.Front()
	for i := 0; i < delay; i++ {
		if el == nil {
			el = en.PushBack(make([]*errNodeInfo, 0, 1))
		}
		if i == (delay - 1) {
			break
		} else {
			el = el.Next()
		}
	}
	arr := el.Value.([]*errNodeInfo)
	arr = append(arr, errInfo)
	el.Value = arr
}

func (en *errNodes) setEmpty() {
	en.Lock()
	defer en.Unlock()
	en.List = nil
}

// 集群管理器, 集群没有别的功能，就是服务发现，下面的服务映射表(serviceMap)不加锁, 后续所有操作只可能是只读
// 映射表中的服务都会在启动时初始化后，以后只是对服务中的节点进行加减
type Cluster struct {
	serviceMap map[string]*Service
	disCovery  discovery.NodeDiscoveryFace

	// 错误节点，监听到节点关闭后，无法正常关闭的节点
	// 这里是一个链表，链表下挂的是一个数组，会启动一个ticker每秒对链表头进行扫瞄
	//errNodes *list.List
	errNodes

	diectrl.Control
}

func New() *Cluster {
	cluster := &Cluster{
		serviceMap: map[string]*Service{},
		disCovery:  discovery.New(),
	}
	cluster.errNodes.init()

	// 集群会启动2个goroutine来进行操作 1.负责监控node的变化， 2.负责错误node的关闭
	cluster.Init(2)
	return cluster
}

// 根据服务配置初始化服务，服务初始化只执行一次，多次执行并无效果，建议都是在程序启动时
// 执行，目的是注册所有服务，没有初始化的服务是不会有节点监听的，即便是有也会跳过
// 调用者保证confArray数组中的参数不重复, 初始化代码不做重复检查，如果重复则会覆盖
// 如果初始化失败，直接panic, 服务初始化不能正常完成，给出错误返回也无实际意义。
func (cluster *Cluster) InitService(confArray []ServiceConfig) {
	if len(confArray) <= 0 {
		logrus.Panic("初始化服务，必需要存在服务.....目前1个服务都没有....")
	}

	for _, conf := range confArray {
		if len(conf.Name) <= 0 {
			logrus.Panic("初始化集群服务时，发现配置数组中有服务名称为空....")
		}

		if conf.CliFaceBuildFn == nil {
			logrus.Panicf("初始化 %s 服务时，发现配置中没有给出接口构建方法(CliFaceBuildFn=NIL)", conf.Name)
		}

		s := &Service{
			name:           conf.Name,
			cliFacebuildFn: reflect.ValueOf(conf.CliFaceBuildFn),
			nodeClientMap:  map[string]*NodeClient{},
		}
		cluster.serviceMap[s.name] = s
	}

	go cluster.nodeProcess()
	go cluster.errProcess()
}

// todo: 这里需要注意是否会产生panic, 产生panic时要恢复处理,
// todo: 这里还需要处理，正常退出时，要退出的协程
func (cluster *Cluster) nodeProcess() {
	nodeNotify := cluster.disCovery.Listen()
	for {
		select {
		case node := <-nodeNotify:
			cluster.nodeUpdate(node)
		case <-cluster.WaitDie():
			goto end
		}
	}
end:
	cluster.closeAllNodeCli()
	cluster.Done()
}

// 关闭所有节点
func (cluster *Cluster) closeAllNodeCli() {
	for _, service := range cluster.serviceMap {
		cliArray := service.NodeCliArray()
		for _, cli := range cliArray {
			err := utils.Retry(StopCloseRetryNum, time.Second, 1, func() error {
				return cli.Close()
			})
			if err != nil {
				logrus.WithError(err).Errorf("关服时节点关闭错误....service: %s, uid: %s, ip: %s",
					cli.serviceName, cli.uid, cli.ip)
				// todo: 加办法再记一次，至少可以发到某个地方提醒
			}
		}
	}
}

//todo: 关闭节点错误后的后续处理，这里需要注意是否会产生panic,产生panic时要恢复处理
//todo: 这里还需要处理，正常退出时，要退出的协程
func (cluster *Cluster) errProcess() {
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ticker.C:
			errInfoArray := cluster.poperr()
			if errInfoArray == nil {
				continue
			}
			for _, errInfo := range errInfoArray {
				errInfo.retryNum++
				err := errInfo.nodeCli.Close()
				if err != nil {
					if errInfo.retryNum >= ErrRetryNum {
						logrus.WithError(err).Errorf("关闭错误重试次数过多....service: %s, uid: %s, ip: %s",
							errInfo.nodeCli.serviceName, errInfo.nodeCli.uid, errInfo.nodeCli.ip)
						// todo: 除了正常日志打印，看还记录在哪里，之后能调出来看，并且向类似微信丁丁之类的进行报警
						continue
					}

					// 每次延迟都进行时间递增
					cluster.pusherr(errInfo, ErrDelay*errInfo.retryNum)
				}
			}
		case <-cluster.WaitDie():
			goto end
		}
	}
end:
	// 最后进行所有错误的清理
	cluster.finalCleanupErr()
	cluster.Done()
}

// 最终清理错误
func (cluster *Cluster) finalCleanupErr() {
	cloneList := cluster.errNodes

	cluster.errNodes.setEmpty()

	for el := cloneList.Front(); el != nil; el = el.Next() {
		arr := el.Value.([]*errNodeInfo)
		if arr == nil {
			continue
		}
		for _, errInfo := range arr {
			err := errInfo.nodeCli.Close()
			if err != nil {
				logrus.WithError(err).Errorf("节点最终关闭错误....")
				// todo: 除了正常日志打印，看还记录在哪里，之后能调出来看，并且向类似微信丁丁之类的进行报警
			}
		}
	}
}

// 最终返出去，让调用者决定是同步调用，还是异步调用
func (cluster *Cluster) Stop() <-chan struct{} {
	return cluster.CloseAndEnd(func() {
		<-cluster.disCovery.Stop()
	})
}

// 节点更新, 集群中服务的映射表操作不需要有锁，但服务下面的节点映射表需要考虑并发冲突
// 对于服务里节点的操作，都提供在服务结构体上的方法，这样不论后续是否应该加锁操作，都可以
// 只对方法进行修改
func (cluster *Cluster) nodeUpdate(node *discovery.Node) {
	if service := cluster.serviceMap[node.ServiceName]; service != nil {
		if node.Off { //节点关闭
			nodeCli, err := service.closeNode(node.Uid)
			// 关闭节点失败
			if err != nil {
				logrus.WithError(err).Errorf("节点关闭失败 service: %s, uid: %s, ip: %s",
					nodeCli.serviceName, nodeCli.uid, nodeCli.ip)
				cluster.pusherr(&errNodeInfo{nodeCli: nodeCli}, ErrDelay)
			}
		} else {
			logrus.Infof("准备打开节点.... service: %s, uid: %s, ip: %s, port: %d",
				node.ServiceName, node.Uid, node.Ip, node.Port)
			err := service.openNode(node)
			if err != nil {
				logrus.WithError(err).Errorf("节点打开失败... service: %s, uid: %s, ip: %s, port: %d",
					node.ServiceName, node.Uid, node.Ip, node.Port)
				// todo: 这里是否应该进行记录，或者要发送到某个对容器生命周期监控的容器进行容器的消亡???
			}
		}
	}
}

func (cluster *Cluster) ClientInterfaceWithUid(serviceName string, nodeUid string) interface{} {
	// todo:
	return nil
}

func (cluster *Cluster) ClientInterfaceWithMin(serviceName string) interface{} {
	// todo:
	return nil
}

func (cluster *Cluster) ClientInterfaceWithMax(serviceName string) interface{} {
	// todo:
	return nil
}

func (cluster *Cluster) MinNodeCli(serviceName string) *NodeClient {
	// todo:
	return nil
}

func (cluster *Cluster) MaxNodeCli(serviceName string) *NodeClient {
	// todo:
	return nil
}

func (cluster *Cluster) NodeCli(serviceName string, nodeUid string) *NodeClient {
	// todo:
	return nil
}
