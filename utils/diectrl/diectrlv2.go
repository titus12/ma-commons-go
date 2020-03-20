package diectrl

import (
	"context"
	"sync"
)

type ControlV2 struct {
	ctx    context.Context
	cancel context.CancelFunc

	finalDie chan struct{} //最终死亡
	sync.WaitGroup
}

func (ctrl *ControlV2) Init(goNum int) {
	ctx, cancel := context.WithCancel(context.Background())

	ctrl.ctx = ctx
	ctrl.cancel = cancel

	ctrl.finalDie = make(chan struct{})

	if goNum > 0 {
		ctrl.Add(goNum)
	}
}

func (ctrl *ControlV2) Ctx() context.Context {
	return ctrl.ctx
}

// 返回等待死亡的控制通道
func (ctrl *ControlV2) WaitDie() <-chan struct{} {
	return ctrl.ctx.Done()
}

// 返回最终死亡的控制通道
func (ctrl *ControlV2) FinalDie() <-chan struct{} {
	return ctrl.finalDie
}

func (ctrl *ControlV2) CloseAndEnd(fn func()) <-chan struct{} {
	go func() {
		ctrl.cancel()
		ctrl.Wait()

		if fn != nil {
			// 进行结束后的收尾工作
			fn()
		}

		close(ctrl.finalDie)
	}()
	return ctrl.finalDie
}

// 运行生命周期.....
func (ctrl *ControlV2) GoLoopFun(run func(), end func()) {
	go func() {
		for {
			select {
			case <-ctrl.ctx.Done():
				end()
				goto xx
			default:
				run()
			}
		}
	xx:
		ctrl.Done()
	}()
}
