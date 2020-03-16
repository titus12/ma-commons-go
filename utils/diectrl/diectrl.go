// 死亡控制，需要组合到需要要的结构里去，死亡分成二步
// 1. 等待死亡
// 2. 最终死亡

package diectrl

import "sync"

type Control struct {
	waitDie  chan struct{} //等待死亡
	finalDie chan struct{} //最终死亡
	sync.WaitGroup
}

// 初始化
// goNum: 表示待控制的结构体会有几个goroutines进行操作
func (ctrl *Control) Init(goNum int) {
	ctrl.waitDie = make(chan struct{})
	ctrl.finalDie = make(chan struct{})

	if goNum > 0 {
		ctrl.Add(goNum)
	}
}

// 停止并且结束, 对于死亡控制的通常做法，调用即可，传递需要善尾的方法
// 这里只是一般做法，但有些结构的死亡，需要结体自行控制，结构体本身可能启动了协程
// 所以如果要细微控制，此方法就不必要调用，可以调用CloseWaitDie(),CloseFinalDie()
// 分别对死亡和死亡等待进行关闭
func (ctrl *Control) CloseAndEnd(fn func()) <-chan struct{} {
	go func() {
		close(ctrl.waitDie)
		ctrl.Wait()

		if fn != nil {
			// 进行结束后的收尾工作
			fn()
		}

		close(ctrl.finalDie)
	}()

	return ctrl.finalDie
}

// 返回等待死亡的控制通道
func (ctrl *Control) WaitDie() <-chan struct{} {
	return ctrl.waitDie
}

// 返回最终死亡的控制通道
func (ctrl *Control) FinalDie() <-chan struct{} {
	return ctrl.finalDie
}

// 关闭等待死亡通道(相当于开始死亡),并返回最终死亡通道
func (ctrl *Control) CloseWaitDie() {
	close(ctrl.waitDie)
}

// 关闭最终死亡通道
func (ctrl *Control) CloseFinalDie() {
	close(ctrl.finalDie)
}
