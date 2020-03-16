// k8s 的服务发现
// +build k8s

package discover

import (
	"fmt"
	"time"

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

type k8simpl struct {
	clientset      *kubernetes.Clientset
	stopController chan struct{}
	stopWaitNotify chan struct{}

	controller cache.Controller
	nodeNofity chan *Node
}

func newImpl() *k8simpl {
	config, err := rest.InClusterConfig()
	if err != nil {
		// todo: 抛出去时，打印点什么，
		logrus.Panic(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		// todo: 抛出去时，打印点什么，
		logrus.Panic(err)
	}

	k8s := &k8simpl{
		clientset:      clientset,
		stopController: make(chan struct{}),
		stopWaitNotify: make(chan struct{}),
		nodeNofity:     make(chan *Node, defaultListenSize),
	}
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
		go controller.Run(k8s.stopController)
	}
	return k8s.nodeNofity
}

func (k8s *k8simpl) Stop() <-chan struct{} {
	go func() {
		close(k8s.stopController)

		// todo: 进行一些收尾工作，暂时没有东西要处理

		close(k8s.stopWaitNotify)
	}()
	return k8s.stopWaitNotify
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
		logrus.Info("容器没有定义任何端口，说明不对外开放服务.....")
		return
	}

	off := checkOff(pod)

	// 每一个对外端口都是开放的一个节点
	for _, portInfo := range pod.Spec.Containers[0].Ports {
		if portInfo.Protocol != v1.ProtocolTCP {
			continue
		}

		node := &Node{
			Uid:         buildUid(pod.Status.ContainerStatuses[0].ContainerID, portInfo.ContainerPort),
			ServiceName: pod.Status.ContainerStatuses[0].Name,
			Ip:          pod.Status.PodIP,
			Port:        portInfo.ContainerPort,
			Off:         off,
		}

		k8s.nodeNofity <- node
	}
}

// 构建一个节点的唯一id，取容器id的前12位
func buildUid(containerID string, port int32) string {
	return fmt.Sprintf("%s_%d", containerID[9:9+12], port)
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
