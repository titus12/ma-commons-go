package services

import "google.golang.org/grpc"

type nodeData struct {
	addr   string `json:"addr"`
	status int8   `json:"status"`
}

// a single connection
type node struct {
	key     string
	conn    *grpc.ClientConn
	data    nodeData
	isLocal bool
	ready   bool
}
