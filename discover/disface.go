package discover

const (
	defaultListenSize = 2000
)

// 发现服务接口
type DisServiceface interface {
	// 开始监听，返回接收节点的通道，方法可以重复调用，第一次调用则是开启监听，以后则只是返回接收节点的通道
	Listen() <-chan *Node

	// 停止监听，返回停止等待信号
	Stop() <-chan struct{}

	//// 是否停止
	//IsStop() bool
}

func New() DisServiceface {
	return newImpl()
}

var (
	defaultDiscover DisServiceface
)

func init() {
	defaultDiscover = New()
	defaultDiscover.Listen()
}

// 节点 (ip:port 肯定是唯一的)
type Node struct {
	ServiceName string //服务名称
	Uid         string //唯一id
	Ip          string //ip地址
	Port        int32
	Off         bool // 是否关闭（true: 添加服务，false: 关闭服务)
}

func Listen() <-chan *Node {
	return defaultDiscover.Listen()
}

func Stop() <-chan struct{} {
	return defaultDiscover.Stop()
}

//func IsStop() bool {
//	return defaultDiscover.IsStop()
//}
