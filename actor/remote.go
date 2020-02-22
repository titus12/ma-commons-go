package actor

import (
	"fmt"

	"github.com/titus12/ma-commons-go/utils"

	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/titus12/ma-commons-go/stsmq"
)

const (
	defaultMqNum = 4 //默认队列数量
	mq_format    = "actor_mq_%d_%d"
)

type defaultRemote struct {
	mqs []stsmq.SerMessageQueue
}

type defaultMsgProcesser struct {
	name string
}

var remote = &defaultRemote{}

//开启远程actor，开启远程actor后，actor就拥有跨进程访问actor的能力
func StartRemoteActor() {
	if SystemRef == nil {
		panic("Actor System Not Start......")
	}
	for i := 0; i < defaultMqNum; i++ {
		mq_name := fmt.Sprintf(mq_format, SystemRef.Location, i)
		mq := stsmq.New(mq_name)
		mq.SetProcessor(&defaultMsgProcesser{
			name: mq_name,
		})
		remote.mqs = append(remote.mqs, mq)
		plog.Info(fmt.Sprintf("Actor Recv MQ [%s]", mq_name))
	}
}

func (this *defaultRemote) newMsg(msg proto.Message) *RemoteMsg {
	name := proto.MessageName(msg)
	msgdata, err := proto.Marshal(msg)
	if err != nil {
		plog.WithError(err).Error("Build RemoteMsg Fail")
		return nil
	}
	return &RemoteMsg{
		MsgName: name,
		MsgData: msgdata,
	}
}

//发送远程消息(reqId > 0 && isResp == false：说明消息是请求消息,如果reqId <= 0:说明消息是正常消息，如果reqId > 0 && isResp==true
//说明消息是响应消息
func (this *defaultRemote) send(sender *ActorRef, target *ActorRef, reqId int64, isResp bool, msg interface{}) {
	if sender == nil {
		panic("sender ActorRef is Null")
	}
	if target == nil {
		panic("Target ActorRef is Null")
	}
	//目标不能为本地引用
	if target.IsLocal() {
		panic("Target Location is Local")
	}

	if !sender.IsLocal() {
		//发送者一定是本机的actor ,
		//TODO:这里是否要抛出异常要再看情况而定，理论上在开发阶阶应该尽可能早的发现错误，而这里如果在运营阶发现有问题
		//TODO:不能因为单个消息的错误，让整个程序崩溃
		panic("sender ActorRef Not Local")
	}

	//远程消息，msg一定是proto.Message，否则抛异常
	protoMsg, ok := msg.(proto.Message)
	if !ok {
		panic("Not protobuf message,remote msg must is one protobuf message")
	}
	remoteMsg := this.newMsg(protoMsg)
	remoteMsg.Sender = sender
	remoteMsg.Target = target
	remoteMsg.ReqId = reqId
	remoteMsg.IsRespond = isResp

	//目标队列索引
	idx := uint(utils.Hash(target.Name)) % uint(defaultMqNum)
	mq := this.mqs[idx]

	targetName := fmt.Sprintf(mq_format, target.Location, idx)   //得出需要发送到目标队列的名称
	innerMsg, err := stsmq.NewInnerSerMsg(targetName, remoteMsg) //构建一个InnerMsg
	if err != nil {
		plog.WithError(err).Error("send RemoteMsg fail...")
		return
	}
	mq.Push(innerMsg)
}

//收到消息后的分发处理
func (this *defaultMsgProcesser) Dispose(msg *stsmq.InnerSerMsg) {
	plog.Debug(fmt.Sprintf("recv mq msg mq_name: %s", this.name))

	data, err := msg.Body.Bytes()
	if err != nil {
		plog.WithError(err).Error("msg Dispose Err")
		return
	}
	rmsg := &RemoteMsg{}
	err = proto.Unmarshal(data, rmsg)
	if err != nil {
		plog.WithError(err).Error("msg Dispose Unmarshal Err")
		return
	}

	var targetPid *PID

	//如果目标是系统actor，则不需要到本地去找，直接赋值系统pid,理论上是不会出现目标为系统的，但如果是系统
	//发出的请求，然后又收回响应，此时target就会是系统actor
	if rmsg.Target.IsSystem() {
		//如果从队列里处理的消息里target检查为系统，则一定是本地系统，不可能是远端系统
		targetPid = SystemPid
	} else {
		targetPid = GetLocalActorRef(int(rmsg.Target.Category), rmsg.Target.Name)
	}
	if targetPid == nil {
		plog.Error(fmt.Sprintf("from remote msg, target actor not exist, sender=%v,target=%v", rmsg.Sender, rmsg.Target))
		return
	}

	//发送者pid
	senderPid := NewPidWithActorRef(rmsg.Sender)

	t := proto.MessageType(rmsg.MsgName)
	if t == nil {
		plog.Error(fmt.Sprintf("from remote msg, rmsg.MsgName is Null[%s]", rmsg.MsgName))
		return
	}

	protoMsg := reflect.New(t.Elem()).Interface()
	proto.Unmarshal(rmsg.MsgData, protoMsg.(proto.Message))
	if rmsg.IsRespond {
		//如果是响应，执行响应的逻缉，找到Future，并把结果放置到通道里
		future := FutureRegistry.Get(rmsg.ReqId)
		if future == nil {
			plog.Error(fmt.Sprintf("from remote resp fail, cause: future not find, reqId=%d", rmsg.ReqId))
			return
		} else {
			future.setResult(protoMsg)
		}
	} else {
		sendmsg(senderPid, targetPid, rmsg.ReqId, protoMsg)
	}
}
