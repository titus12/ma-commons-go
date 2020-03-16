package discovery

const (
	defaultListenSize = 2000
)

// 发现服务接口
type NodeDiscoveryFace interface {
	// 开始监听，返回接收节点的通道，方法可以重复调用，第一次调用则是开启监听，以后则只是返回接收节点的通道
	Listen() <-chan *Node

	// 停止监听，返回停止等待信号
	Stop() <-chan struct{}

	//// 是否停止
	//IsStop() bool
}

func New() NodeDiscoveryFace {
	return newImpl()
}

//var (
//	defaultDiscover NodeDiscoveryFace
//)

//func init() {
//	defaultDiscover = New()
//	defaultDiscover.Listen()
//}

// 节点 (ip:port 肯定是唯一的)
type Node struct {
	ServiceName string //服务名称
	Uid         string //唯一id
	Ip          string //ip地址
	Port        int32
	Off         bool // 是否关闭（true: 关闭，false: 开启)
}

//func Listen() <-chan *Node {
//	return defaultDiscover.Listen()
//}
//
//func Stop() <-chan struct{} {
//	return defaultDiscover.Stop()
//}

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
