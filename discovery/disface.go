package discovery

import (
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	defaultListenSize = 5000

	defaultTcpDialTimeOut = 5 * time.Second
)

// 发现服务接口
type NodeDiscoveryFace interface {
	// 开始监听，返回接收节点的通道，方法可以重复调用，第一次调用则是开启监听，以后则只是返回接收节点的通道
	Listen() <-chan *Node

	// 停止监听，返回停止等待信号
	Stop() <-chan struct{}
}

func New() NodeDiscoveryFace {
	return newImpl()
}

// 节点 (uid肯定是唯一的)
type Node struct {
	ServiceName string //服务名称
	Uid         string //唯一id
	Ip          string //ip地址
	Port        int32
	Off         bool // 是否关闭（true: 关闭，false: 开启)
}

func (node *Node) String() string {
	str := fmt.Sprintf("{Service: %s, Uid: %s %s:%d, Off: %t}",
		node.ServiceName, node.Uid, node.Ip, node.Port, node.Off)
	return str
}

func (node *Node) Address() string {
	return fmt.Sprintf("%s:%d", node.Ip, node.Port)
}

// 拿到机器自已的内网ip地址
//func selfIpArray() (iparr []string, err error) {
//	netInterfaces, err := net.Interfaces()
//	if err != nil {
//		logrus.Errorf("net.Interfaces failed, err: %v", err)
//		return
//	}
//
//	for i := 0; i < len(netInterfaces); i++ {
//		if netInterfaces[i].Flags&net.FlagUp != 0 {
//			addrs, _ := netInterfaces[i].Addrs()
//
//			for _, address := range addrs {
//				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
//					if ipnet.IP.To4() != nil {
//
//						fmt.Println(ipnet.IP.String())
//					}
//				}
//			}
//		}
//	}
//
//}

// 检查节点表示的网络是否畅通
func checkNet(n *Node) bool {
	conn, err := net.DialTimeout("tcp", n.Address(), defaultTcpDialTimeOut)
	if err != nil {
		logrus.WithError(err).Errorf("探测连接失败...%s", n.Address())
		return false
	}

	defer conn.Close()
	return true
}
