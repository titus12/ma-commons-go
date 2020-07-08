package services

import "google.golang.org/grpc"

// 节点状态
const (
	ServiceStatusNone    = iota // 节点起动中，还未加入到集群，暂时不可提供服务，可以认为服务就是不存在
	ServiceStatusPending        // 节点已加入到服务，但是目前还是在等其他节点完成操作（比如数据牵移）
	ServiceStatusRunning        // 节点处于正常
	ServiceStatusStopping
)

const (
	TransferStatusNone = iota
	TransferStatusSucc
	TransferStatusFail
)

var StatusServiceName = map[int8]string{
	ServiceStatusNone:     "NONE",
	ServiceStatusPending:  "PENDING",
	ServiceStatusRunning:  "RUNNING",
	ServiceStatusStopping: "STOPPING",
}

type nodeData struct {
	addr   string `json:"addr"`
	status int8   `json:"status"`
}

// a single connection
type node struct {
	key      string
	conn     *grpc.ClientConn
	data     nodeData
	isLocal  bool
	transfer int32
}

func NewNode(name string, conn *grpc.ClientConn, data nodeData, isLocal bool, transfer int32) *node {
	return &node{name, conn, data, isLocal, transfer}
}
