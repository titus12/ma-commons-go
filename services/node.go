package services

import "google.golang.org/grpc"

const (
	ServiceStatusNone = iota
	ServiceStatusPending
	ServiceStatusRunning
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
