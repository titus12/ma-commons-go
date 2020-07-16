package actor

import "google.golang.org/grpc"

// 节点状态  todo:
const (
	nodeStatusNone     = iota
	nodeStatusPending  //加入中
	nodeStatusRunning  //运行中
	nodeStatusStopping //停止中
)

var nodeStatusName = map[int32]string{
	nodeStatusNone:     "NONE",
	nodeStatusPending:  "PENDING",
	nodeStatusRunning:  "RUNNING",
	nodeStatusStopping: "STOPPING",
}

// 缓存托管接口，在某些时候我们希望有地方可以管理进程中的内存，避免内存无限增长带来
// 的问题, cache必须实现sync.Map的接口，也就是下面的接口，规范也是按sync.Map来，必
// 须是线程安全的，之前所是按sync.Map的接口来，这样sync.Map可以直接使用，而且go sdk
// 有多很集合都有实现这类方法
type CacheTrust interface {
	Load(interface{}) (interface{}, bool)
	Store(key, value interface{})
	LoadOrStore(key, value interface{}) (actual interface{}, loaded bool)
	Delete(interface{})
	Range(func(key, value interface{}) (shouldContinue bool))
}

type ClusterInfo interface {
	// 给集群设置回调
	SetCallBack(fn func(nodeKey string, nodeStatus int32) error)

	// 获取集群的服务名称
	ServiceName() string

	// 在新的哈希环中是否在本地
	// local: true:在本地，false:不在本地
	// nodeKey: 命中的节点位置
	// nodeStatus: 命中节点当前的状态
	// conn: 命中节点的grpc连接
	IsLocalWithStableRing(id int64) (local bool, nodeKey string, nodeStatus int32, conn *grpc.ClientConn, err error)

	// 在旧的哈希环中是否在本地
	IsLocalWithUnstableRing(id int64) (local bool, nodeKey string, nodeStatus int32, conn *grpc.ClientConn, err error)
}
