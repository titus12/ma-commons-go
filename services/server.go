package services

import (
	"context"
	"math"
	"net"

	log "github.com/sirupsen/logrus"
	gp "github.com/titus12/ma-commons-go/services/pb-grpc"
	"github.com/titus12/ma-commons-go/utils"
	"google.golang.org/grpc"
)

type server struct{}

type serverWrapper struct {
	listener *net.Listener
	gServer  *grpc.Server
}

// 接收牵移状态通知的，告知某节点数据牵移完成
func (s *server) Notify(cxt context.Context, node *gp.Node) (*gp.Result, error) {
	defer utils.PrintPanicStack()
	result := &gp.Result{ErrorCode: 0, Error: ""}
	if err := transfer(node.Name, node.Status); err != nil {
		result.ErrorCode = 1
		result.Error = err.Error()
	}
	return result, nil
}

func NewServerWrapper(listen string) (*serverWrapper, error) {
	serverWrapper := &serverWrapper{}
	// 监听
	lis, err := net.Listen("tcp", listen)
	if err != nil {
		return nil, err
	}
	log.Info("listening on ", lis.Addr())
	//注册服务
	s := grpc.NewServer(grpc.MaxConcurrentStreams(math.MaxInt32))
	serverWrapper.listener = &lis
	serverWrapper.gServer = s
	ins := &server{}
	gp.RegisterNodeServiceServer(s, ins)
	return serverWrapper, nil
}

// 启动服务器
func (s *serverWrapper) Start() error {
	err := s.gServer.Serve(*s.listener)
	log.Infof("开始监听服务")
	return err
}
