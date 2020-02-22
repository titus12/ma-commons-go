package mailbox

// 调度器接口，下面会有二种调度器，一种是go程的，一种是同步的
// go程的，actor的消息处理是会在另一个go程里，可以认为一个actor会启动一个go程,但这个
// go程不会一直存在，当mailbox里没有消息处理时，这个go程就会消失，以保证当actor创建过
// 多时，而actor又处于空闲时，而挂起不必要的go程
// 另一种基本不会用，消息处理就在消息处理的go程里，不会另外起go程
type Dispatcher interface {
	Schedule(fn func())
	Throughput() int
}

type goroutineDispatcher int

// 进行调度(goroutine)
func (goroutineDispatcher) Schedule(fn func()) {
	go fn()
}

// 吞吐量(goroutine)，人为设定了调度器的吞吐量
func (d goroutineDispatcher) Throughput() int {
	return int(d)
}

// 构建一个默认的调度器，默认调度器是goroutine调度的
func NewDefaultDispatcher(throughput int) Dispatcher {
	return goroutineDispatcher(throughput)
}

type synchronizedDispatcher int

// 进行调度(sync)，没有goroutine参与
func (synchronizedDispatcher) Schedule(fn func()) {
	fn()
}

// 吞吐量(sync)，人为设定了调度器的吞吐量
func (d synchronizedDispatcher) Throughput() int {
	return int(d)
}

// 构建一个sync的调度器
func NewSynchronizedDispatcher(throughput int) Dispatcher {
	return synchronizedDispatcher(throughput)
}
