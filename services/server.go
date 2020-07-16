package services

import (
	"context"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"math"
	"net"
	"sync"
	"time"
)

import (
	gp "github.com/titus12/ma-commons-go/services/pb-grpc"
	"github.com/titus12/ma-commons-go/utils"
)

const defaultTcpDialTimeOut = 2 * time.Second

type server struct{}

type serverWrapper struct {
	listener *net.Listener
	gServer  *grpc.Server
	wg       sync.WaitGroup

	address string
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
	serverWrapper.address = listen
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
func (s *serverWrapper) Start() {
	s.wg.Add(1)
	ch := make(chan struct{})
	defer close(ch)
	go func() {
		<-ch
		err := s.gServer.Serve(*s.listener)
		if err != nil {
			log.Fatalf("start tcp server err %v", err)
		}
		log.Infof("start tcp server listen")
	}()
	ch <- struct{}{}
	//runtime.Gosched()
}

func (s *serverWrapper) Done() {
	s.wg.Done()
}

func (s *serverWrapper) Wait() {
	s.wg.Wait()
}

// 检查节点表示的网络是否畅通
func (s *serverWrapper) checkNet() bool {
	conn, err := net.DialTimeout("tcp", s.address, defaultTcpDialTimeOut)
	if err != nil {
		log.Errorf("checkNet %s err %v", s.address, err)
		return false
	}
	defer conn.Close()
	return true
}
