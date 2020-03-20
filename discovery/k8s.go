// k8s 的服务发现, 这里监控的是pod然后找出pod中第0个容器的状态，另外这里监控的是k8s的默认名字空间
// 所以我们后面所有服都应该是在默认名称空间下， 另外端口也是pod中定义的第0个端口，原则上一个pod不
// 能提供2个端口来进行grpc
// +build k8s

package discovery

import (
	"time"

	"github.com/titus12/ma-commons-go/utils/diectrl"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/tools/cache"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	defaultCondSize = 4 //默认有4个条件
)

// 节点发现，用k8s的api直接发现pod中的容器，规定，除开initcontainer,pod中的所有容器
// 在设定的中，第0个要设定成节点对外提供grpc端口的容器，最好的设定是是pod中只有一个容
// 器
type k8simpl struct {
	clientset         *kubernetes.Clientset // k8s的客户端访问集合
	diectrl.ControlV1                       //死亡控制器
	controller        cache.Controller      // k8s的用于节点发现的controller
	nodeNofity        chan *Node            // 节点通知，节点发现后会放到这个通道
}

// 构建k8s实现
func newImpl() *k8simpl {
	// 拿到集群内配置，这里要注意,k8s集群要绑定cluster-role的admin
	config, err := rest.InClusterConfig()

	// 错误返回无意义了，无法正常启动，则直接让服务器关停
	if err != nil {
		logrus.WithError(err).Panicln("k8s集群配置错误")
		//TODO: 这里最好可以把错误发送到丁丁、微信或手机短信以便通知错误
	}

	clientset, err := kubernetes.NewForConfig(config)

	// 错误返回无意义了，无法正常启动，则直接让服务器关停
	if err != nil {
		logrus.WithError(err).Panicln("构建k8s访问器错误")
		//TODO: 这里最好可以把错误发送到丁丁、微信或手机短信以便通知错误
	}

	k8s := &k8simpl{
		clientset:  clientset,
		nodeNofity: make(chan *Node, defaultListenSize),
	}

	// 死亡控制器初始化，在内部没有启动goroutine来等待完成某些事情
	k8s.Init(0)
	return k8s
}

func (k8s *k8simpl) Listen() <-chan *Node {
	if k8s.controller == nil {
		watchlist := cache.NewListWatchFromClient(k8s.clientset.RESTClient(), string(v1.ResourcePods), v1.NamespaceDefault, fields.Everything())
		_, controller := cache.NewInformer(watchlist, &v1.Pod{}, time.Second*0, cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				k8s.buildNode(obj.(*v1.Pod))
			},
			DeleteFunc: func(obj interface{}) {
				k8s.buildNode(obj.(*v1.Pod))
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				k8s.buildNode(newObj.(*v1.Pod))
			},
		})
		k8s.controller = controller
		go controller.Run(k8s.WaitDie())
	}
	return k8s.nodeNofity
}

func (k8s *k8simpl) Stop() <-chan struct{} {
	return k8s.CloseAndEnd(func() {
		//close(k8s.nodeNofity)  -- 不能关闭（不清楚controller.Run(k8s.WaitDie())的执行顺序)
	})
}

// 构建节点，发送给通知, pod在定义时，首个容器一定是对外提供服务的
func (k8s *k8simpl) buildNode(pod *v1.Pod) {
	// 条件还不完整
	if len(pod.Status.Conditions) < defaultCondSize {
		return
	}
	// 容器还未启动
	if len(pod.Status.ContainerStatuses) <= 0 {
		return
	}

	// 容器还未启动完成
	if len(pod.Status.ContainerStatuses[0].ContainerID) <= 0 {
		return
	}

	if len(pod.Spec.Containers[0].Ports) <= 0 {
		logrus.Infoln("容器没有定义任何端口，说明不对外开放服务.....")
		return
	}

	if pod.Spec.Containers[0].Ports[0].Protocol != v1.ProtocolTCP {
		logrus.Infoln("容器定义的端口不是TCP协议.....")
		return
	}

	off := checkOff(pod)

	node := &Node{
		Uid: buildUid(pod.Status.ContainerStatuses[0].ContainerID),
		//Uid:         buildUid(pod.Status.ContainerStatuses[0].ContainerID, portInfo.ContainerPort),
		ServiceName: pod.Status.ContainerStatuses[0].Name,
		Ip:          pod.Status.PodIP,
		Port:        pod.Spec.Containers[0].Ports[0].ContainerPort,
		Off:         off,
	}

	k8s.nodeNofity <- node

	// 每一个对外端口都是开放的一个节点
	//for _, portInfo := range pod.Spec.Containers[0].Ports {
	//	if portInfo.Protocol != v1.ProtocolTCP {
	//		continue
	//	}
	//
	//	node := &Node{
	//		Uid: buildUid(pod.Status.ContainerStatuses[0].ContainerID),
	//		//Uid:         buildUid(pod.Status.ContainerStatuses[0].ContainerID, portInfo.ContainerPort),
	//		ServiceName: pod.Status.ContainerStatuses[0].Name,
	//		Ip:          pod.Status.PodIP,
	//		Port:        portInfo.ContainerPort,
	//		Off:         off,
	//	}
	//
	//	k8s.nodeNofity <- node
	//}
}

// 构建一个节点的唯一id，取容器id的前12位
//func buildUid(containerID string, port int32) string {
//	return fmt.Sprintf("%s_%d", containerID[9:9+12], port)
//}

// 通过容器id，构建一个节点uid
func buildUid(containerID string) string {
	return containerID[9 : 9+12]
	//return fmt.Sprintf("%s", containerID[9:9+12])
}

// 检查是否关闭
func checkOff(pod *v1.Pod) bool {
	if pod.Status.Phase != v1.PodRunning {
		return true
	}

	// 检查条件
	for _, cond := range pod.Status.Conditions {
		if cond.Status != v1.ConditionTrue {
			return true
		}
	}

	// 检查容器状态
	if !pod.Status.ContainerStatuses[0].Ready {
		return true
	}

	// 容器不处于运行状态
	if pod.Status.ContainerStatuses[0].State.Running == nil {
		return true
	}
	return false
}
