package actor

import (
	"errors"
	"time"
)

var ErrFutureTimeout = errors.New("future: timeout")        //在规定时间内没有结果返回
var ErrFutureChanClose = errors.New("future: result close") //结果通道不明原因半闭

// 每次发送请求（注：不是发送消息，这个是有区别的，请求一定会有响应）都会产生一个Future用于追踪响应
type Future struct {
	id     int64            //futureId
	err    error            //是否发生错误
	result chan interface{} //等待收到的结果
	t      *time.Timer      //超时保护定时器
	//pipes        []*PID
	//completions  []func(res interface{}, err errors)
}

// 构建一个新的Future,传入一个超时时间，超时后会自行处理
func NewFuture(d time.Duration) *Future {
	id := FutureRegistry.NextId()
	f := &Future{
		id: id,
		//结果只等待一个，等待时是阻塞，但给出结构的时不应该有等待，所以是有缓存通道，并且是1
		result: make(chan interface{}, 1),
	}
	FutureRegistry.SetIfAbsent(id, f)

	if d > 0 {
		f.t = time.AfterFunc(d, func() {
			f.err = ErrFutureTimeout
			f.close()
		})
	}
	return f
}

func (f *Future) Id() int64 {
	return f.id
}

// 等待结果回来，如果发生错误一定是等待超时了
func (f *Future) Result() (interface{}, error) {
	defer f.close()

	if f.err != nil {
		return nil, f.err
	}
	r, ok := <-f.result
	if ok {
		return r, nil
	} else {
		if f.err == nil {
			f.err = ErrFutureChanClose
		}
		return r, f.err
	}
}

// 设置结构，内部调用，不暴露
func (f *Future) setResult(res interface{}) {
	defer func() {
		recover()
		//x := recover()
		//if x != nil {
		//	fmt.Println("结果写入通道时，通道已经关闭")
		//}
		f.close()
	}()
	f.result <- res
}

func (f *Future) close() {
	defer func() {
		recover() //捕捉方法的异常，因为关闭很有可能通道已经关闭了
	}()
	FutureRegistry.Remove(f.id)
	if f.t != nil {
		f.t.Stop()
		f.t = nil
	}
	if f.result != nil {
		close(f.result)
	}
}
