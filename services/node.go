package services

import (
	"context"
	"google.golang.org/grpc"
)

// 节点状态
const (
	ServiceStatusNone     = iota // 节点起动中，还未加入到集群，暂时不可提供服务，可以认为服务就是不存在
	ServiceStatusPending         // 节点已加入到服务，但是目前还是在等其他节点完成操作（比如数据牵移）
	ServiceStatusRunning         // 节点处于正常，可对外提供服务
	ServiceStatusStopping        // 节点正在关闭，节点可以被路由到，但不对外提供服务
)

// 牵移状态
const (
	TransferStatusNone = iota
	TransferStatusSucc
	TransferStatusFail
)

var StatusServiceName = map[int32]string{
	ServiceStatusNone:     "NONE",
	ServiceStatusPending:  "PENDING",
	ServiceStatusRunning:  "RUNNING",
	ServiceStatusStopping: "STOPPING",
}

// 节点数据
type NodeData struct {
	Addr   string `json:"addr"`   //ip:port
	Status int32  `json:"status"` // 节点状态
}

// 节点
type node struct {
	key      string // 可以认为是节点的唯一标记
	conn     *grpc.ClientConn
	data     NodeData
	isLocal  bool
	transfer int32
	ctx      context.Context
	cancel   context.CancelFunc
}

type optFn func(n *node)

func withContext() optFn {
	return func(n *node) {
		n.ctx, n.cancel = context.WithCancel(context.Background())
	}
}

func withClientConn(clientConn *grpc.ClientConn) optFn {
	return func(n *node) {
		n.conn = clientConn
	}
}

func NewNode(name string, nodeData NodeData, isLocal bool, transfer int32, opts ...optFn) *node {
	node := &node{key: name, data: nodeData, isLocal: isLocal, transfer: transfer}
	for _, opt := range opts {
		opt(node)
	}
	return node
}
