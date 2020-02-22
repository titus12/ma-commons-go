package actor

import (
	"github.com/titus12/ma-commons-go/utils"
)

// 实现此接口的结构休都是一个Actor
type ActorHandler interface {
	Receive(c Context)
}

// 定义Actor的方法，为那些不希望使用对像的单独方法准备的
type ActorFunc func(c Context)

// 让ActorFunc实现Receive方法，以达到ActorFunc就是一个Actor
func (f ActorFunc) Receive(c Context) {
	//当actor处理发生异常时，不能就此让整个应用发生终结，这里必须捕获这个异常，并把调用栈打印出来
	defer utils.PrintPanicStack()
	f(c)
}
