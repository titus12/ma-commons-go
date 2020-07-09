package actor

// 定义系统消息, 实现此接口则为系统消息
type SystemMessage interface {
	SystemMessage()
}

// 超时消息
type timeout struct{}

// 启动消息
type start struct{}

// 停止消息
type stop struct{}

// ping 消息
type ping struct{}

func (*timeout) SystemMessage() {}
func (*start) SystemMessage()   {}

func (*stop) SystemMessage() {}
func (*ping) SystemMessage() {}

var (
	startMessage   interface{} = &start{}
	stopMessage    interface{} = &stop{}
	timeoutMessage interface{} = &timeout{}
	pingMessage    interface{} = &ping{}
)
