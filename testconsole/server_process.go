package testconsole

import (
	"context"

	"github.com/titus12/ma-commons-go/actor"

	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"

	"github.com/titus12/ma-commons-go/testconsole/testmsg"
)

type server struct{}

func StartConsoleServer(s *grpc.Server) {
	testmsg.RegisterTestConsoleServer(s, &server{})
}

func (s *server) LocalRunRequest(ctx context.Context, request *testmsg.LocalRun) (*testmsg.LocalRunResponse, error) {
	logrus.Infof("LocalRun 收到控制台请求消息: %v", request)

	senderId := int64(actor.NoSenderId)
	if request.SenderId > 0 {
		senderId = request.SenderId
	}

	err := ActorSystem.Tell(senderId, request.TargetId, request)
	if err != nil {
		return nil, err
	}

	return &testmsg.LocalRunResponse{
		ReplyId: request.ReqId,
	}, nil
}

func (s *server) LocalRunPendingRequest(ctx context.Context, request *testmsg.LocalRunPending) (*testmsg.LocalRunResponse, error) {
	logrus.Infof("LocalRunPending 收到控制台请求消息: %v", request)

	senderId := int64(actor.NoSenderId)
	if request.SenderId > 0 {
		senderId = request.SenderId
	}

	err := ActorSystem.Tell(senderId, request.TargetId, request)
	if err != nil {
		return nil, err
	}

	return &testmsg.LocalRunResponse{
		ReplyId: request.ReqId,
	}, nil
}
