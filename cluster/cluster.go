package cluster

import (
	"container/list"
	"time"

	"github.com/titus12/ma-commons-go/utils/diectrl"

	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/discovery"
	"google.golang.org/grpc"
)

const ERR_RETRY_NUM = 20 // 错误重试次数
const ERR_DELAY = 2      //错误延迟

const DIAL_RETRY_NUM = 10 //拔号重试次数
const DIAL_INTERVAL = 2   //拔号间隔

// 错误节点信息
type errNodeInfo struct {
	nodeCli *NodeClient //节点客户端
	num     int         // 关闭次数
}

// 集群管理器, 集群没有别的功能，就是服务发现，下面的服务映射表不加锁, 后续所有操作只可能是只读
// 映射表中的服务都会在启动时初始化后，以后只是对服国中的节点进行加减
type Cluster struct {
	serviceMap map[string]*Service
	disCovery  discovery.NodeDiscoveryFace

	// 错误节点，监听到节点关闭后，无法正常关闭的节点
	// 这里是一个链表，链表下挂的是一个数组，会启动一个ticker每秒对链表表进行扫瞄
	errNodes *list.List

	diectrl.Control
}

func New() *Cluster {
	cluster := &Cluster{
		serviceMap: map[string]*Service{},
		disCovery:  discovery.New(),
		errNodes:   list.New(),
	}

	// 集群会启动2个goroutines来进行操作 1.负责监控node的变化， 2.负责错识node的关闭
	cluster.Init(2)
	return cluster
}

// 服务的配置
type ServiceConfig struct {
	Name           string      //服务名称
	CliFaceBuildFn interface{} //客户调用接口生成方法（grpc都会有一个New接口的方法)
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
			cliFacebuildFn: conf.CliFaceBuildFn,
			nodeClientMap:  map[string]*NodeClient{},
		}

		cluster.serviceMap[s.name] = s
	}
	cluster.disCovery = discovery.New()

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
	// todo: 把所有节点进行关闭

	cluster.Done()
}

//todo: 关闭节点错误后的后续处理，这里需要注意是否会产生panic,产生panic时要恢复处理
//todo: 这里还需要处理，正常退出时，要退出的协程
func (cluster *Cluster) errProcess() {
	ticker := time.NewTicker(time.Second)

	for {
		select {
		case <-ticker.C:
			errInfoArray := cluster.poperr()
			for _, errInfo := range errInfoArray {
				errInfo.num++
				err := errInfo.nodeCli.grpc.Close()
				if err != nil {
					if errInfo.num >= ERR_RETRY_NUM {
						logrus.Error(err)
						// todo: 除了正常日志打印，看还记录在哪里，之后能调出来看，并且向类似微信丁丁之类的进行报警
						continue
					}

					// 每次延迟都进行时间递增
					cluster.pusherr(errInfo, ERR_DELAY*errInfo.num)
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
	cluster.errNodes = nil

	for el := cloneList.Front(); el != nil; el = el.Next() {
		arr := el.Value.([]*errNodeInfo)
		for _, errInfo := range arr {
			err := errInfo.nodeCli.grpc.Close()
			if err != nil {
				logrus.WithError(err).Errorf("节点最终关闭错误....")
				// todo: 除了正常日志打印，看还记录在哪里，之后能调出来看，并且向类似微信丁丁之类的进行报警
			}
		}
	}
}

// 弹出错误信息
func (cluster *Cluster) poperr() []*errNodeInfo {

	if cluster.errNodes == nil {
		return nil
	}

	el := cluster.errNodes.Front() // 第一个元素
	cluster.errNodes.Remove(el)
	if el != nil && el.Value != nil {
		return el.Value.([]*errNodeInfo)
	}
	return nil
}

// 放错误信息，一定延迟后会弹出，延迟最小计量单位秒
func (cluster *Cluster) pusherr(errInfo *errNodeInfo, delay int) {
	if cluster.errNodes == nil {
		return
	}

	el := cluster.errNodes.Front()
	for i := 0; i < delay; i++ {
		if el == nil {
			el = cluster.errNodes.PushBack(make([]*errNodeInfo, 0, 1))
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

// 最终返出去，让调用者决定是同步调用，还是异步调用
func (cluster *Cluster) Stop() <-chan struct{} {
	return cluster.CloseAndEnd(func() {
		cluster.disCovery.Stop()
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
				logrus.WithError(err).Errorf("节点关闭失败 uid: %s, ip ")
				cluster.pusherr(&errNodeInfo{nodeCli: nodeCli}, ERR_DELAY)
			}
		} else {

		}
	}
}

// 服务这里的接口生成会采用反射，虽然go的反射性能不好，但由于服务发现执行次数较少，统一处理
// 还是采用反射，另外由于go不提供模板，所以统一返回 interface{}, cliFacebuildFn反射的是grpc
// 对客户端接口New的方法，返回具体接口，需要调用者自已强转，调用者肯定清楚是哪个服务的接口.
// 注意：这里会有人统一grpc的接口，但个人认为这样的设计不好，虽然代码编写上会很方便，但对于使
//       用的人很不友好
type Service struct {
	name           string                 //服务名称
	cliFacebuildFn interface{}            //客户端接口生成方法（grpc都会有一个New接口的方法)
	nodeClientMap  map[string]*NodeClient //服务所连接的地址
}

// 关闭节点(对节点的操作都建立在服务结构的方法中，方便以后变更操作方式，比如现在没有进行加锁，
// 后面很大可能加锁
func (service *Service) closeNode(uid string) (nodeCli *NodeClient, err error) {
	if nodeCli := service.nodeClientMap[uid]; nodeCli != nil {
		delete(service.nodeClientMap, uid)
		err = nodeCli.grpc.Close()
		return
	}
	return
}

// 打开节点失败，怎么处理，（不希望和k8s进行强关系)
func (service *Service) openNode(node *discovery.Node) {
	// todo:
}

type NodeClient struct {
	grpc *grpc.ClientConn //grpc的连接

	//uid  string
	//ip   string
	//port int

	weight  int         //权重
	cliFace interface{} //客户端调用接口
}

// 返回客户端调用接口
func (cli *NodeClient) ClientInterface() interface{} {
	return cli.cliFace
}

//func (s *Service) WeightSelect() *NodeClient {
//	// todo: 权重选择，选择权重最小的
//	return nil
//}
