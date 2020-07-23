package actor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
)

// actor的一个引用表示，actor本身的对像可能存在数据，但数据对于actor来说都应该隐藏
// 所有变动都是通过引用传递消息来驱动的
type Ref struct {
	id int64 //actor id, 在一个系统里唯一标识一个actor的,如果为0表示是一个系统actor，常驻

	mu sync.RWMutex

	// actor归属哪个系统
	owner *System

	// 状态, 状态的操作都是原子操作
	status int32

	// 邮箱，可以理解为是actor的消息队列，在actor中都是把这个称为邮箱的
	mailbox chan interface{}

	// 消息处理者，我们把一个带有数据的结构体实现了这引接口，那么这个带数据的结构体就是
	// 一个有效的actor，但数据对于actor本身是不暴露的，所以用Ref来指向
	// 比如一个玩家Player{},里面有玩家的所有数据，这个结构体实现Handler后，我们就可以通
	// 过Ref发消息来驱动Player
	handler Handler

	lastStartTime time.Time //最近一次消息处理开始时间
	lastEndTime   time.Time //最近一次消息处理结束时间

	burningTime int64 //总共处理时长，按纳秒计算(所有消息加起来一共消耗多少处理时间)
	msgCount    int64 //总共处理了多少消息

	pingCount int64 //ping计数

	ctx    context.Context
	cancel context.CancelFunc

	// 最近处理的消息（可以认为是上一个处理的消息）
	lastMsg interface{}

	// 超时处理取消（消息开始处理，超时还未到就要把超时取消掉)
	outtimeCancel CancelFunc

	// ping的取消方法
	pingCancel CancelFunc
}

// 在规定时间内等待actor直到满足某个条件
func (ref *Ref) wait(condStatus int32, timeout time.Duration) error {
	if atomic.LoadInt32(&ref.status) >= condStatus {
		return nil
	}

	// 等待通道
	waitCha := make(chan struct{}, 1)

	// 开启循环定时器，每秒查看一下状态是否到达条件
	cancel := startTimer(time.Second, time.Second, func() {
		if atomic.LoadInt32(&ref.status) >= condStatus {
			waitCha <- struct{}{}
		}
	})

	ctx, ctxCancel := context.WithTimeout(context.Background(), timeout)
	select {
	case <-waitCha:
		cancel()
		ctxCancel()
		return nil
	case <-ctx.Done():
		cancel()
		return ctx.Err()
	}
}

// 等待摧毁
func (ref *Ref) WaitDestroyed(timeout time.Duration) error {
	return ref.wait(Destroyed, timeout)
}

func (ref *Ref) Get() (value interface{}, ok bool) {
	if ref.owner.cache == nil {
		logrus.Panicf("[actor]id=%d,CacheTrust Not Supper....", ref.id)
	}
	return ref.owner.cache.Load(ref.id)
}

func (ref *Ref) Set(value interface{}) {
	if ref.owner.cache == nil {
		logrus.Panicf("[actor]id=%d,CacheTrust Not Supper....", ref.id)
	}
	ref.owner.cache.Store(ref.id, value)
}

// 是否为系统actor, 系统actor是常驻的，不能stop，只有停止掉整个系统或体系才能停止
func (ref *Ref) IsSystemRef() bool {
	if ref.id == 0 {
		return true
	}
	return false
}

// 停止掉一个actor
func (ref *Ref) Stop() error {
	if ref.IsSystemRef() {
		return errors.New("actor is systemActor")
	}
	return ref.stop()
}

// 内部的停止方法
func (ref *Ref) stop() error {
	ref.mu.Lock()
	defer ref.mu.Unlock()

	// Runnable 与 Running 的情况下都有可能发生 stop
	if atomic.CompareAndSwapInt32(&ref.status, Running, Destroying) || atomic.CompareAndSwapInt32(&ref.status, Runnable, Destroying) {
		// 如果存在ping，停止掉ping
		if ref.pingCancel != nil {
			ref.pingCancel()
		}
		ref.mailbox <- stopMessage
		return nil
	}
	status := atomic.LoadInt32(&ref.status)
	return errors.New(fmt.Sprintf("stop actor status error(running => destroying), status: %d", status))
}

// 传递消息给actor，不能传递系统消息
func (ref *Ref) Push(msgOrCtx interface{}) error {
	_, ok := msgOrCtx.(SystemMessage)
	if ok {
		return errors.New("system message not push")
	}

	return ref.pushmsg(msgOrCtx)
}

// 内部的传递消息方法
func (ref *Ref) pushmsg(msgOrCtx interface{}) error {
	ref.mu.RLock()
	defer ref.mu.RUnlock()

	if status := atomic.LoadInt32(&ref.status); status >= Destroying {
		return errors.New(fmt.Sprintf("push msg actor status is destroy, currstatus: %d", status))
	}
	ref.mailbox <- msgOrCtx
	return nil
}

func (ref *Ref) ToPid() *Pid {
	return &Pid{
		systemName: ref.owner.name,
		id:         ref.id,
	}
}

func (ref *Ref) start() {
	go func() {
		defer printPanicStack()
		// 发起超时保护(设置超时时间，并且不是系统actor，则有超时保护)
		if ref.owner.otime > 0 && !ref.IsSystemRef() {
			ref.outtimeCancel = SendOnce(ref.owner.otime, ref, timeoutMessage)
		}

		if ref.owner.pingInterval > 0 {
			ref.pingCancel = SendRepeatedly(ref.owner.pingInterval, ref.owner.pingInterval, ref, pingMessage)
		}

		for {
			select {
			case msg := <-ref.mailbox:
				if ref.owner.otime > 0 && !ref.IsSystemRef() && ref.outtimeCancel != nil {
					ref.outtimeCancel()
					ref.outtimeCancel = nil
				}

				ref.lastStartTime = time.Now()

				// 把执行放到一个变量，然后再执行变量，不能明确执行的方法不会出问题
				// 出现问题后可以继续进行，因为后有关于超时的处理

				func() {
					defer printPanicStack()
					switch msg.(type) {
					case SystemMessage: //系统消息
						processSystemMessage(ref, msg)
					case ProcessMsgFunc: //方法消息，直接处理了
						fn := msg.(ProcessMsgFunc)
						fn(ref)
					case Context: //信封消息(会包装消息之外的其他参数)
						//envel := msg.(*Envelope)
						ref.handler.OnProcess(msg.(Context))
					default:
						selfPid := ref.owner.NewPid(ref.id)
						ctx := newDefaultContext(NoSender, selfPid, ref.owner, msg, nil)
						ref.handler.OnProcess(ctx)
					}
				}()

				ref.lastEndTime = time.Now()
				d := ref.lastEndTime.Sub(ref.lastStartTime)
				ref.burningTime += d.Nanoseconds()
				ref.lastMsg = msg

				if ref.owner.otime > 0 && !ref.IsSystemRef() {
					delay := ref.owner.otime - d

					// 处理超时
					if atomic.LoadInt32(&ref.status) <= Running {
						if delay <= 0 {
							ref.mailbox <- timeoutMessage
						} else {
							ref.outtimeCancel = SendOnce(delay, ref, timeoutMessage)
						}
					}
				}

				if atomic.LoadInt32(&ref.status) == Destroyed {
					logrus.Infof("actor is destroyed exit gorun[id=%d]", ref.id)
					return
				}

			case <-ref.ctx.Done():
				err := ref.stop()
				if err != nil {
					logrus.WithError(err).Errorf("actor recv ctx.Done Call ref.stop() fail")
					// 从容器中移除actor
					//ref.owner.container.Delete(ref.id)
					//if ref.owner.cache != nil {
					//	ref.owner.cache.Delete(ref.id)
					//}
					//return
				}
			}
		}
	}()
	ref.mailbox <- startMessage
}

// 一个新的actor
func newActor(ctx context.Context, cancel context.CancelFunc, owner *System, id int64, handler Handler) *Ref {
	ref := &Ref{
		id:      id,
		owner:   owner,
		status:  Runnable,
		mailbox: make(chan interface{}, DefaultQueueSize),
		handler: handler,
		ctx:     ctx,
		cancel:  cancel,
	}
	return ref
}

// 返回 bool 告诉协程是否要退出了，true: 退出， false 不退出
func processSystemMessage(ref *Ref, msg interface{}) {
	switch msg.(type) {
	case *start:
		// 执行start时可能发生问题，但发生问题后，还是要继续 todo: 这里需要讨论一下，因为是开始，是否考虑直接让进程崩溃
		func() {
			defer printPanicStack()
			ref.handler.OnStarted(ref)
		}()

		// 执行完后检查状态并变更状态....由于是串行不可能发生状态错误，发生错误说明程序写得有问题，立即终止程序修复
		if !atomic.CompareAndSwapInt32(&ref.status, Runnable, Running) {
			//status := atomic.LoadInt32(&ref.status)
			//
			//if status >= Destroying
			//
			//panic(fmt.Sprintf("actor[id=%d] processSystemMessage status error OnStarted finish[%d => %d], curr: %d",
			//	ref.id, Runnable, Running, status))
		}

	case *timeout:
		// 执行outtime时可能发生问题，但发生问题后，还是要继续发送stop
		func() {
			defer printPanicStack()
			ref.handler.OnTimeout(ref)
		}()
		err := ref.stop()
		if err != nil {
			status := atomic.LoadInt32(&ref.status)
			logrus.WithError(err).Errorf("actor[id=%d]TimeOut Call stop Fail currstatus: %d", ref.id, status)
		}
	case *stop:
		// 检查后面是否还有消息,保证stopMessage是最后一条消息
		if len(ref.mailbox) > 0 {
			ref.mailbox <- stopMessage
		} else {
			// todo: 摧毁方法的执行是否要保证进程不崩溃
			func() {
				defer printPanicStack()
				ref.handler.OnDestroy(ref)
			}()

			// 无论是否成功，都要改变成Destoryed
			if !atomic.CompareAndSwapInt32(&ref.status, Destroying, Destroyed) {
				status := atomic.LoadInt32(&ref.status)
				logrus.Errorf("[id: %d]actor destroying to destroyed fail, curr status: %d", status)
			}

			// 无论是否成功，都要改变成Destoryed
			atomic.StoreInt32(&ref.status, Destroyed)

			// 从容器中移除actor
			ref.owner.container.Delete(ref.id)
			if ref.owner.cache != nil {
				ref.owner.cache.Delete(ref.id)
			}
		}
	case *ping:
		if ref.IsSystemRef() {
			func() {
				defer printPanicStack()
				ref.handler.OnPing(ref, ref.pingCount)
			}()
			ref.pingCount++
		}
	default:
		panic(fmt.Sprintf("actor[id=%d]Not Supper System Message", ref.id))
	}

	return
}

//// 在相同的actor系统中进行请求
func (ref *Ref) RequestWithSameSystem(target int64, msg interface{}) (interface{}, error) {
	return ref.owner.Ask(ref.id, target, msg)
}
