package actor

import (
	"errors"
	"time"

	"github.com/titus12/ma-commons-go/actor/mailbox"
)

var (
	ErrPidNullOrNotLocal = errors.New("pid pointer is null or pid not local") //pid指针是空或者不在本地
	ErrCategoryNotExist  = errors.New("actor category not exist")             //类别不存在
	ErrNameAlreadyExist  = errors.New("actor name already exist")             //actor名字已经存在
	ErrPidLocalNotExist  = errors.New("local pid pointer not exist")          //本地pid指针不存在

	ErrReqRespALike = errors.New("req pid and resp pid the same") //请求actor与响应actor是同一个
	ErrTargetNull   = errors.New("target pid is null")            //消息发送的目标pid是空的
	ErrSenderNull   = errors.New("sender pid is null")            //消息发送者pid是空的
)

var (
	defaultDispatcher      = mailbox.NewDefaultDispatcher(300) //默认的分发器
	defaultMailboxProducer = mailbox.UnboundedLockfree()       // 默认的邮箱生产者
)

var (
	SystemRef *ActorRef // 系统引用
	SystemPid *PID      // 系统PID
)

// 开始一个actor系统
// serId: 传递给actor系统的服务器id，用于构建一个最初的actor，你可以认为这是一个系统actor
func StartActorSystem(serId uint32) {
	SystemRef = &ActorRef{
		Location: serId,
	}

	SystemPid = &PID{
		ActorRef: *SystemRef,
	}
}

// 注册Actor的类别，每一种类别都有接收消息接口（每一种类别的actor处理方式完全一样)
func RegisterCategory(f ActorFunc, category int) {
	if SystemRef == nil {
		panic("Actor System Not Start......")
	}
	if !addCategoryHandler(category, f) {
		panic("reg actorfunc fail, because already exists")
	}
}

// 创建actor,要给actor一个名字，以及一个类别
func CreateActor(name string, category int) (*PID, error) {
	if SystemRef == nil {
		panic("Actor System Not Start......")
	}

	if !isExistLocalCategory(category) {
		return nil, ErrCategoryNotExist
	}
	newPid := NewLocalPid(category, name)
	if isExistLocalActorWithPid(newPid) {
		return nil, ErrNameAlreadyExist
	}
	handler := getCategoryHandler(category) //获取一个类别的handler

	ctx := newDefaultContext(handler, newPid) //actor的上下文
	mb := defaultMailboxProducer()            //actor的邮箱
	dp := defaultDispatcher                   //actor的分发器
	proc := newActorProcess(mb)               //actor的模拟进程

	err := addLocalProcess(proc, newPid)
	if err != nil {
		return newPid, err
	}
	ctx.self = newPid
	mb.Start()
	mb.RegisterHandlers(ctx, dp)
	mb.PostSystemMessage(startedMessage)

	return newPid, nil
}

// 停止一个actor
func StopActor(pid *PID) error {
	p := getLocalProcessWithPid(pid)
	if p == nil {
		return ErrPidNullOrNotLocal
	}
	p.Stop(pid)
	return nil
}

// 向actor发送消息,sender发送者，target接收者，sender可以为空
func SendMsg(sender *PID, target *PID, message interface{}) {
	sendmsg(sender, target, 0, message)
}

// 发送消息,发送者与目标接收者都不能为空
// reqId: 表示此次发送的消息是一个请求消息
func sendmsg(sender *PID, target *PID, reqId int64, message interface{}) {
	if sender == nil || &sender.ActorRef == nil {
		panic(ErrSenderNull)
	}
	if target == nil || &target.ActorRef == nil {
		panic(ErrTargetNull)
	}
	//如果不是本地的，则说明是跨服务器进行消息通讯，这个时候消息必须是proto.Message
	if !target.IsLocal() {
		//这里向远端发送消息一定不是响应消息，但有可能是请求消息或普通消息
		remote.send(&sender.ActorRef, &target.ActorRef, reqId, false, message)
	} else {
		me := WrapEnvelope(message)
		me.Sender = sender
		me.FutureId = reqId
		target.sendUserMessage(me)
	}
}

// 根据类别和名称获取本地的actor引用,此方法已经破坏了接口设计的原则，但目前还没想到好方法
// 原意是不想让使用者能拿到PID,pid由使用者自行保存，但对于网络actor的消息发送不太友好
// 只能提供获取的方法,后继会考虑把接口去掉，系统中对于actor的实现只有一条途径，不提供其他扩展
// 思考: 不以接口做设计，提供标准的结构体，我们系统的实现实际上是不公有对接口扩展的可能性了
func GetLocalActorRef(category int, name string) *PID {
	ap, ok := getLocalProcess(category, name).(*ActorProcess)
	if !ok {
		return nil
	}
	if ap == nil {
		return nil
	}
	ctx, ok := ap.mailbox.Invoker().(*defaultContext)
	if !ok {
		return nil
	}
	if ctx == nil {
		return nil
	}
	return ctx.self
}

// actor发送请求，这个方法有别于SendMsg，他一定是有响应的
func RequestFuture(sender *PID, target *PID, message interface{}, timeout time.Duration) *Future {
	if sender == nil || &sender.ActorRef == nil {
		panic(ErrSenderNull)
	}
	if target == nil || &target.ActorRef == nil {
		panic(ErrTargetNull)
	}

	//请求者与响应者不能是同一个
	if sender.Equal(target) {
		panic(ErrReqRespALike)
	}

	f := NewFuture(timeout)

	if !target.IsLocal() {
		//如果不是本地，说明要把消息发送给远端，并且让远端消息标记为一个请求消息
		remote.send(&sender.ActorRef, &target.ActorRef, f.id, false, message)
	} else {
		me := WrapEnvelope(message)
		me.F = f
		me.FutureId = f.id
		me.Sender = sender
		target.sendUserMessage(me)
	}
	return f
}

// 向指定的Actor进行广播(暂未实现)
func Broadcast(pids []*PID, message interface{}) {
	//TODO:
}

//TODO:需要加入actor的监控，监控指标有待讨论
