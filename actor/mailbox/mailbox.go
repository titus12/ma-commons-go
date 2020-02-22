package mailbox

import (
	"runtime"
	"sync/atomic"

	"github.com/sirupsen/logrus"
)

//type SystemMessage interface {
//	actor.SystemMessage
//}

// 统计接口（TODO:还未起作用）
type Statistics interface {
	MailboxStarted()
	MessagePosted(message interface{})
	MessageReceived(message interface{})
	MailboxEmpty()
}

// 消息调用者结口，这里默认实现就是ctx
type MessageInvoker interface {
	InvokeSystemMessage(interface{})                         // 调用系统消息
	InvokeUserMessage(interface{})                           // 调用用户消息
	EscalateFailure(reason interface{}, message interface{}) //消息故障
}

// 邮箱接口
type Mailbox interface {
	Invoker() MessageInvoker                                        // 返回调用者
	PostUserMessage(message interface{})                            // 投递用户消息
	PostSystemMessage(message interface{})                          // 投递系统消息
	RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) //注册消息处理者
	Start()                                                         // 让邮箱开始工作
}

// 创建新mailbox的函数,工厂方法
type Producer func() Mailbox

// 下面是邮箱的排程状态，要么空闲，要么运行中。
const (
	idle    int32 = iota // 空闲
	running              // 运行
)

// 默认邮箱结构
type defaultMailbox struct {
	userMailbox     queue          // 用户消息邮箱
	systemMailbox   queue          // 系统消息邮箱
	schedulerStatus int32          // 排程状态
	userMessages    int32          // 用户消息数量
	sysMessages     int32          // 系统消息数量
	suspended       int32          // 邮箱是否挂起(1:挂起，0:正常)
	invoker         MessageInvoker // 这里实际会是ctx
	dispatcher      Dispatcher     // 调度器，这里默认会是goroutine
	mailboxStats    []Statistics   // 邮箱的统计，会有各类统计，也可自定定义注册进来（暂时没用）
}

// 投递用户消息
func (m *defaultMailbox) PostUserMessage(message interface{}) {
	for _, ms := range m.mailboxStats {
		ms.MessagePosted(message)
	}
	m.userMailbox.Push(message)
	atomic.AddInt32(&m.userMessages, 1)
	m.schedule()
}

// 投递系统消息
func (m *defaultMailbox) PostSystemMessage(message interface{}) {
	for _, ms := range m.mailboxStats {
		ms.MessagePosted(message)
	}

	m.systemMailbox.Push(message)
	atomic.AddInt32(&m.sysMessages, 1)
	m.schedule()
}

// 注册消息调用者
func (m *defaultMailbox) RegisterHandlers(invoker MessageInvoker, dispatcher Dispatcher) {
	m.invoker = invoker
	m.dispatcher = dispatcher
}

// 开始调度
func (m *defaultMailbox) schedule() {
	if atomic.CompareAndSwapInt32(&m.schedulerStatus, idle, running) {
		m.dispatcher.Schedule(m.processMessages)
	}
}

// 处理消息
func (m *defaultMailbox) processMessages() {
process:
	m.run()
	// run里处理完后，把邮箱状态设置为idle
	atomic.StoreInt32(&m.schedulerStatus, idle)
	sys := atomic.LoadInt32(&m.sysMessages)
	user := atomic.LoadInt32(&m.userMessages)

	// 再次检查是否还有消息要处理
	if sys > 0 || (atomic.LoadInt32(&m.suspended) == 0 && user > 0) {
		//试着再设置邮箱的的状态，从idle 到 running
		if atomic.CompareAndSwapInt32(&m.schedulerStatus, idle, running) {
			goto process
		}
	}

	for _, ms := range m.mailboxStats {
		ms.MailboxEmpty()
	}
}

func (m *defaultMailbox) run() {
	var msg interface{}
	defer func() {
		if r := recover(); r != nil {
			plog.WithFields(logrus.Fields{
				"actor":  m.invoker,
				"reason": r,
				"stack":  Stack(),
			}).Debug("[ACTOR] Recovering")
			m.invoker.EscalateFailure(r, msg)
		}
	}()

	i, t := 0, m.dispatcher.Throughput()
	for {
		// 下面代码可以看到调度器中的吞吐量了，由于go是一个非抢占式调试的程序，
		// 在没有发生时间片切换时，吞吐量到达了指定数量时，手动切换，以保证其他actor能正常处理消息
		if i > t {
			i = 0
			runtime.Gosched()
		}
		i++

		// 处理系统消息(系统消息全部处理完后，才轮到用户消息处理),这里要注意的事，当系统消息中有Stop时，则后续的用户消息都是无法
		// 处理的，系统消息和用户消息是不按顺序处理的，系统消息有序，用户消息有序，但并不代表，系统和用户消息一起有序。
		if msg = m.systemMailbox.Pop(); msg != nil {
			atomic.AddInt32(&m.sysMessages, -1)
			switch msg.(type) {
			case *SuspendMailbox:
				atomic.StoreInt32(&m.suspended, 1)
			case *ResumeMailbox:
				atomic.StoreInt32(&m.suspended, 0)
			default:
				m.invoker.InvokeSystemMessage(msg)
			}
			for _, ms := range m.mailboxStats {
				ms.MessageReceived(msg)
			}
			continue
		}

		// 邮箱处于挂起状态
		if atomic.LoadInt32(&m.suspended) == 1 {
			return
		}

		// 处理用户消息
		if msg = m.userMailbox.Pop(); msg != nil {
			atomic.AddInt32(&m.userMessages, -1)
			m.invoker.InvokeUserMessage(msg)
			for _, ms := range m.mailboxStats {
				ms.MessageReceived(msg)
			}
		} else {
			return
		}
	}
}

// 让邮箱开始工作
func (m *defaultMailbox) Start() {
	for _, ms := range m.mailboxStats {
		ms.MailboxStarted()
	}
}

// 返回消息调用者
func (m *defaultMailbox) Invoker() MessageInvoker {
	return m.invoker
}
