package testconsole

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/titus12/ma-commons-go/setting"

	"github.com/titus12/ma-commons-go/actor"

	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"

	"github.com/titus12/ma-commons-go/testconsole/testmsg"
)

type server struct{}

func StartConsoleServer(s *grpc.Server) {
	testmsg.RegisterTestConsoleServer(s, &server{})
}


// 查询actor是否存在
func (s *server) QueryMsgRequest(ctx context.Context, request *testmsg.QueryMsg) (*testmsg.QueryMsgResponse, error) {
	ref := ActorSystem.Ref(request.TargetId)

	resp := &testmsg.QueryMsgResponse{
		ReplyId:              request.ReqId,
		TargetId:             request.TargetId,
	}

	if ref == nil {
		// 没有找到
		return resp, nil
	}

	resp.NodeName = setting.Key
	return resp, nil

}

func (s *server) RunMsgRequest(ctx context.Context, request *testmsg.RunMsg) (*testmsg.RunMsgResponse, error) {
	// 追加节点
	request.NodeKeys = fmt.Sprintf("%s,%s", request.NodeKeys, "console_"+setting.Key)
	logrus.Infof("RunMsgRequest 收到控制台请求消息: %v", request)

	resp, err := ActorSystem.Ask(actor.NoSenderId, request.TargetId, request)
	if err != nil {
		logrus.Errorf("RunMsgRequest err %v req %v", err, request)
		return nil, err
	}

	if response, ok := resp.(*testmsg.RunMsgResponse); ok {
		// 消息转换成功
		logrus.Infof("RunMsgRequest Successful execution req(%v) response(%v)", request, response)
		return response, nil

	}

	// 不能进行消息转换的
	err = errors.Errorf("not convert *testmsg.RunMsgResponse err %v (resp %v) (req %v)", err,resp, request)
	logrus.Errorf("RunMsgRequest err %v", err)

	return nil, err
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
