package actor

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	stateNone  int32 = iota //初始状态
	stateAlive              //激活
	//stateRestarting
	stateStopping //停止中
	stateStopped  //已停止
)

// 每一个actor人处理，一定会伴随着一个上下文
type Context interface {
	Sender() *PID                                                            //发送者
	Send(target *PID, message interface{})                                   //发送消息,每次发送，收消息的发送都就转变了当前操作的actor
	Tell(message interface{})                                                //给自已发送消息，发送者和自已都变成同一个了
	Request(target *PID, message interface{}, timeout time.Duration) *Future //请求消息，必然有响应
	Respond(response interface{}) bool                                       //响应消息(成功响应则返回true,否则返回false, false:说明收到的消息不是一个请求消息

	//转发消息，把当前消息转到目标,发送者不发生变化，无论转发多少次，发送者一直为第一个发送者
	//此方法可以提供做一些过滤操作，比如消息最终到达点，中间要经过一些其他actor，并做出相应的
	//过滤操作, //TODO:后续考虑提供整个链路的调用线保存
	Forward(target *PID)

	Stop()

	Self() *PID           //自已
	Actor() ActorHandler  //代表消息处理的地方
	Message() interface{} //消息
	MessageHeader() ReadonlyMessageHeader
}

// 默认的context实现
type defaultContext struct {
	actor             ActorHandler
	self              *PID
	messageOrEnvelope interface{}
	receiveTimeout    time.Duration
	state             int32
}

// 构建一个新的默认上下文
func newDefaultContext(actor ActorHandler, pid *PID) *defaultContext {
	ctx := &defaultContext{
		actor: actor,
		self:  pid,
	}
	atomic.StoreInt32(&ctx.state, stateAlive)
	return ctx
}

func (ctx *defaultContext) defaultReceive() {
	//if _, ok := ctx.Message().(*PoisonPill); ok {
	//	ctx.Close(ctx.self)
	//	return
	//}

	//if ctx.props.contextDecoratorChain != nil {
	//	ctx.actor.Receive(ctx.ensureExtras().context)
	//	return
	//}
	ctx.actor.Receive(ctx)
}

func (ctx *defaultContext) processMessage(m interface{}) {
	ctx.messageOrEnvelope = m

	// 不直接调用ctx.actor.Receive,是考虑到，在actor处理消息前可能有一些其他事务要处理
	// 不过暂时还未有事务
	ctx.defaultReceive()
	ctx.messageOrEnvelope = nil
}

//func (ctx *defaultContext) sendUserMessage(target *PID, message interface{}) {
//	//TODO:
//}

func (ctx *defaultContext) handleStop(msg *Stop) {
	if atomic.LoadInt32(&ctx.state) >= stateStopping {
		//已经在停止中或已经停止了
		return
	}
	atomic.StoreInt32(&ctx.state, stateStopping)
	ctx.InvokeUserMessage(stoppingMessage)
	ctx.finalizeStop()
}

func (ctx *defaultContext) handleFailure(msg *Failure) {
	//TODO:
}

// 最终停止actor
func (ctx *defaultContext) finalizeStop() {
	if atomic.LoadInt32(&ctx.state) == stateStopping {
		removeLocalProcess(ctx.self)
		ctx.InvokeUserMessage(stoppedMessage) //最后一个递交的消息，actor已经停止
		atomic.StoreInt32(&ctx.state, stateStopped)
	}
}

// =====下面是实现 MessageInvoker 接口======
func (ctx *defaultContext) InvokeSystemMessage(message interface{}) {
	switch msg := message.(type) {
	case *Started: //目前只实现了一个开始消息
		ctx.InvokeUserMessage(msg)
	case *Stop:
		ctx.handleStop(msg)
	case *Failure:
		ctx.handleFailure(msg)
	default:
		//TODO:重点打印，不知道的系统消息
		plog.WithFields(logrus.Fields{
			"message": msg,
		}).Error("unknown system message")
	}
}

func (ctx *defaultContext) InvokeUserMessage(message interface{}) {
	if atomic.LoadInt32(&ctx.state) == stateStopped {
		// actor已经停止了,不需要进行处理了
		return
	}
	ctx.processMessage(message)
}

func (ctx *defaultContext) EscalateFailure(reason interface{}, message interface{}) {
	//TODO
}

//=======下面是Context的实现===============
func (ctx *defaultContext) Sender() *PID {
	return UnwrapEnvelopeSender(ctx.messageOrEnvelope)
}

func (ctx *defaultContext) Send(target *PID, message interface{}) {
	ctx.self.Send(target, message)
}

func (ctx *defaultContext) Tell(message interface{}) {
	ctx.self.Tell(message)
}

func (ctx *defaultContext) Self() *PID {
	return ctx.self
}

func (ctx *defaultContext) Actor() ActorHandler {
	return ctx.actor
}

func (ctx *defaultContext) Message() interface{} {
	return UnwrapEnvelopeMessage(ctx.messageOrEnvelope)
}

func (ctx *defaultContext) MessageHeader() ReadonlyMessageHeader {
	return UnwrapEnvelopeHeader(ctx.messageOrEnvelope)
}

func (ctx *defaultContext) Request(target *PID, message interface{}, timeout time.Duration) *Future {
	f := ctx.self.RequestFuture(target, message, timeout)
	return f
}

// 响应消息给发送者，前题是这个处理的消息本身是请求消息，否则返回false不成功
func (ctx *defaultContext) Respond(response interface{}) bool {
	f, fid := UnwrapEnvelopeFutureAndFutureId(ctx.messageOrEnvelope)
	if fid <= 0 {
		return false
	}
	if f != nil {
		//如果本地能响应，则直接响应
		f.setResult(response)
		return true
	} else {
		sender := ctx.Sender()
		//否则这个响应应该是响应给远程actor的，下面的逻缉是检查远程actor是否合法，并发送消息给消息队列
		if sender == nil {
			plog.Error(fmt.Sprintf("not sender cannot response msg targetRef=%v", ctx.self.ActorRef))
			return false
		}

		//说明发送者是本机的，那么说明响应超时了
		if sender.IsLocal() {
			plog.Error(fmt.Sprintf("response timeout sender=%v target=%v", sender.ActorRef, ctx.self.ActorRef))
			return false
		}

		remote.send(&ctx.self.ActorRef, &ctx.Sender().ActorRef, fid, true, response)
		return true
	}
	return false
}

func (ctx *defaultContext) Forward(target *PID) {
	if msg, ok := ctx.messageOrEnvelope.(SystemMessage); ok {
		//系统消息不能跟随
		plog.WithFields(logrus.Fields{
			"MsgType": msg,
		}).Error("System Msg Not Forward")
		return
	}
	SendMsg(ctx.Sender(), target, ctx.messageOrEnvelope)
}

func (ctx *defaultContext) Stop() {
	StopActor(ctx.self)
}
