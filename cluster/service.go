package cluster

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/discovery"
	"github.com/titus12/ma-commons-go/utils"
	"github.com/titus12/ma-commons-go/utils/ctxfunc"
	"google.golang.org/grpc"
)

// 这里把map声名成一个类型是为了方便在service结构体上操作map,这里的map会涉及到读写锁
// 如果在操作上总是lock,unlock,极为不方便，而且容易忘记unlock发生错误,而且定义类弄从
// 代码上一下子就能看出是什么操作，但这个类型不开放，仅限在包内使用
type nodeClientMap map[string]*NodeClient

// 添加节点客户
func (cm nodeClientMap) addcli(cli *NodeClient, mu *sync.RWMutex) {
	mu.Lock()
	defer mu.Unlock()
	cm[cli.uid] = cli

}

// 删除节点客户
func (cm nodeClientMap) delcli(uid string, mu *sync.RWMutex) *NodeClient {
	mu.Lock()
	defer mu.Unlock()
	nodeCli, ok := cm[uid]
	if ok {
		delete(cm, uid)
	}
	return nodeCli
}

// 获取节点客户
func (cm nodeClientMap) cli(uid string, mu *sync.RWMutex) *NodeClient {
	mu.RLock()
	defer mu.RUnlock()

	nodeCli := cm[uid]
	return nodeCli
}

// 所有节点客户
func (cm nodeClientMap) all(mu *sync.RWMutex) []*NodeClient {
	cliArray := make([]*NodeClient, 0, len(cm))
	mu.RLock()
	defer mu.RUnlock()

	for _, val := range cm {
		cliArray = append(cliArray, val)
	}
	return cliArray
}

// 服务这里的接口生成会采用反射，虽然go的反射性能不好，但由于服务发现执行次数较少，统一处理
// 必然采用反射，另外由于go不提供模板，所以统一返回 interface{}, cliFacebuildFn反射的是grpc
// 对客户端接口New的方法，返回具体接口，需要调用者自已强转，调用者肯定清楚是哪个服务的接口.
// 注意：这里会有人统一grpc的接口，但个人认为这样的设计不好，虽然代码编写上会很方便，但对于使
//       用的人很不友好
type Service struct {
	mu sync.RWMutex

	name           string        //服务名称
	cliFacebuildFn reflect.Value //客户端接口生成方法（grpc都会有一个New接口的方法)
	nodeClientMap                //服务所连接的地址
}

// 关闭节点, 不论是否正常关闭都从map中删除掉，删除失败的处理是上一层集群管理的
func (service *Service) closeNode(uid string) (nodeCli *NodeClient, err error) {
	nodeCli = service.delcli(uid, &service.mu)
	if nodeCli != nil {
		err = nodeCli.Close()
	}
	return
}

// 打开节点失败，怎么处理，（不希望和k8s进行强关系) -- todo: 注意有可能产生pain,这里能有pain ?? - 代码风险
func (service *Service) openNode(node *discovery.Node) error {
	targetStr := fmt.Sprintf("%s:%d", node.Ip, node.Port)

	nodeCli := service.cli(node.Uid, &service.mu)
	if nodeCli != nil {
		logrus.Infof("节点已经存在....serviceName: %s, uid: %s, ip: %s",
			nodeCli.serviceName, nodeCli.uid, nodeCli.ip)
		return nil
	}

	return utils.Retry(DialRetryNum, DialRetryInterval, 2, func() error {
		var (
			conn *grpc.ClientConn
			err  error
		)
		ctxfunc.Timeout(DialTimeOut, func(ctx context.Context) {
			conn, err = grpc.DialContext(ctx, targetStr, grpc.WithBlock(), grpc.WithInsecure())
		})

		// 打开时可能超时了
		if err != nil {
			return err
		}

		resultArray := service.cliFacebuildFn.Call(utils.ReflectArguments(conn))
		nodeCli := newNodeClient(node, conn, resultArray[0].Interface())
		service.addcli(nodeCli, &service.mu)

		logrus.Infof("节点打开成功....serviceName: %s, uid: %s, ip: %s",
			nodeCli.serviceName, nodeCli.uid, nodeCli.ip)
		return nil
	})
}

// 获取节点客户端
func (service *Service) NodeCli(uid string) *NodeClient {
	return service.cli(uid, &service.mu)
}

// 获取所有节点客户端
func (service *Service) NodeCliArray() []*NodeClient {
	return service.all(&service.mu)
}
