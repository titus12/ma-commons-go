package actor

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc"

	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/actor/pb"
)

type remoteServiceImpl struct{}

// todo: actor之前接收远程消息的地方，这里的处理担心会陷入死循环....
func (service *remoteServiceImpl) Request(ctx context.Context, req *pb.RequestMsg) (resp *pb.ResponseMsg, err error) {

	logrus.Infof("REMOTE: 开始执行请求......%v", req)

	sender := &Pid{id: req.Sender.Id, systemName: req.Sender.System}
	target := &Pid{id: req.Target.Id, systemName: req.Target.System}

	system := target.System()

	// actor系统不存在
	if system == nil {
		err = fmt.Errorf("actor system not exist, %v", req.Target)
		return
	}

	msg, err := req.Data.UnPack()
	if err != nil {
		logrus.WithError(err).WithField("TAG", "ACTOR").
			WithField("sender", req.Sender).
			WithField("target", req.Target).
			WithField("MsgName", req.Data.MsgType).Error("not find msgtype")
		return
	}

	var (
		respMsg interface{}
	)
	if req.IsRespond {
		// 重定向信息中，告知目前节点是处于不稳定状态，在这样的状态下，不要再发生路由了
		// 直接判定是否能执行，不能就返回错误，以避免陷入死循环
		if req.Redirect.NodeStatus > nodeStatusRunning {
			respMsg, err = system.redirectFinalWithAsk(sender.id, target.id, msg)
		} else {
			respMsg, err = system.Ask(sender.id, target.id, msg)
		}
	} else {
		if req.Redirect.NodeStatus > 0 {
			err = system.redirectFinalWithTell(sender.id, target.id, msg)
		} else {
			err = system.Tell(sender.id, target.id, msg)
		}
	}

	if err != nil {
		return
	}

	if req.IsRespond {
		protoRespMsg, ok := respMsg.(proto.Message)
		if !ok {
			err = fmt.Errorf("resp msg not proto.message[actor.id=%d,system=%s] req=%v, resp=%v", target.id, target.systemName, req, respMsg)
			return
		}

		wrapMsg, err := pb.NewWrapMsg(protoRespMsg)
		if err != nil {
			return
		}

		resp = &pb.ResponseMsg{
			Resper: req.Target,
			ReqId:  req.ReqId,
			Data:   wrapMsg,
		}
	} else {
		resp = &pb.ResponseMsg{
			Resper: req.Target,
			ReqId:  req.ReqId,
		}
	}
	return
}

func startRemoteServer(grpcServer *grpc.Server) {
	pb.RegisterRemoteServiceServer(grpcServer, &remoteServiceImpl{})

	//reflection.Register(grpcServer)
	//
	//err = grpcServer.Serve(lis)
	//if err != nil {
	//	return err
	//}
	//
	//logrus.Infof("开始监听远程Actor....%s", address)
	//return nil
}
