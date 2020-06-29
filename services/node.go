package services

import "google.golang.org/grpc"

type nodeData struct {
	addr   string `json:"addr"`
	status int32  `json:"status"`
}

// a single connection
type node struct {
	key     string
	conn    *grpc.ClientConn
	data    *nodeData
	isLocal bool
}
