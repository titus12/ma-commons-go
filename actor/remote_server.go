package actor

import (
	"context"
	"fmt"
	"github.com/titus12/ma-commons-go/actor/pb"
	"github.com/titus12/ma-commons-go/setting"

	"github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"
)

type remoteServiceImpl struct{}

// todo: actor之前接收远程消息的地方，这里的处理担心会陷入死循环....
func (service *remoteServiceImpl) Request(ctx context.Context, req *pb.RequestMsg) (resp *pb.ResponseMsg, err error) {
	//todo: defer utils.PrintPanicStack()

	logrus.Debugf("REMOTE: 当前节点: %s, 开始执行请求......%v", setting.Key, req)

	var senderId, targetId int64
	var sender, target *Pid

	if req.Sender != nil {
		sender = &Pid{id: req.Sender.Id, systemName: req.Sender.System}
		senderId = sender.id
	} else {
		sender = NoSender
		senderId = NoSenderId
	}

	target = &Pid{id: req.Target.Id, systemName: req.Target.System}
	targetId = target.id

	system := target.System()

	// actor系统不存在
	if system == nil {
		err = fmt.Errorf("actor system not exist, %v", req.Target)
		return
	}

	msg, err := req.Data.UnPack()
	if err != nil {
		logrus.Errorf("Request req.Data.UnPack() req(%v) err %v", req, err)
		return
	}

	var (
		respMsg interface{}
	)
	if req.IsRespond {
		// 重定向信息中，告知目前节点是处于不稳定状态，在这样的状态下，不要再发生路由了
		// 直接判定是否能执行，不能就返回错误，以避免陷入死循环
		if req.Redirect.NodeStatus != nodeStatusRunning {
			respMsg, err = system.redirectFinalWithAsk(senderId, targetId, msg)
		} else {
			respMsg, err = system.Ask(senderId, targetId, msg)
		}
	} else {
		if req.Redirect.NodeStatus != nodeStatusRunning {
			err = system.redirectFinalWithTell(senderId, targetId, msg)
		} else {
			err = system.Tell(senderId, targetId, msg)
		}
	}

	if err != nil {
		logrus.Errorf("Request Tell msg req(%v) err %v", req, err)
		return
	}

	if req.IsRespond {
		protoRespMsg, ok := respMsg.(proto.Message)
		if !ok {
			err = fmt.Errorf("resp msg not proto.message[actor.id=%d,system=%s] req=%v, resp=%v", target.id, target.systemName, req, respMsg)
			logrus.Errorf("Request ask msg receive err %v", err)
			return
		}

		var wrapMsg *pb.WrapMsg
		wrapMsg, err = pb.NewWrapMsg(protoRespMsg)
		if err != nil {
			logrus.Errorf("Request wrap msg err %v req(%v)", err, req)
			return nil, err
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

	logrus.Debugf("Request respmsg(%v)", resp)

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
