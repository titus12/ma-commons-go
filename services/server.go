package services

import (
	"context"
	log "github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/utils"
	"google.golang.org/grpc"
	"math"
	"net"
	gp "github.com/titus12/ma-commons-go/services/pb-grpc
)

type server struct{}

func (s *server) Ready(context.Context, *gp.Node) (*gp.Result, error) {
	defer utils.PrintPanicStack()
	return nil,nil
}

func startServer(listen string)(*server,error) {
	// 监听
	lis, err := net.Listen("tcp", listen)
	if err != nil {
		return nil ,err
	}
	log.Info("listening on ", lis.Addr())

	//注册服务
	s := grpc.NewServer(grpc.MaxConcurrentStreams(math.MaxInt32))
	ins := &server{}
	gp.RegisterServiceServer(s, ins)
	log.Infof("开始监听service %s", listen)
	return ins,nil
}
