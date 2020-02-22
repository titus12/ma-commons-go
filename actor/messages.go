package actor

import "github.com/titus12/ma-commons-go/actor/mailbox"

// 定义系统消息接口，实现此接口的结构都视为系统消息
type SystemMessage interface {
	SystemMessage()
}

////凡实现此接口的消息，actor将自动处理
//type AutoReceiveMessage interface {
//	AutoReceiveMessage()
//}

// 接收超时消息(TODO:暂时没有任何处理，先定义)
type ReceiveTimeout struct{}

// 停止消息
type Stop struct{}

// 停止一个actor前的发送消息(说明在停止中)
type Stopping struct{}

// 已经停止消息，actor停止后最后一个收到的消息
type Stopped struct{}

// 启动消息，actor创建完后的第一个消息（TODO:是否需要，还有待讨论)
type Started struct{}

// 失败消息
type Failure struct {
	Who     *PID
	Reason  interface{}
	Message interface{}
}

func (*Stopping) SystemMessage() {} //已用
func (*Stopped) SystemMessage()  {} //已用
func (*Started) SystemMessage()  {} //已用
func (*Stop) SystemMessage()     {} //已用
func (*Failure) SystemMessage()  {}

//func (*mailbox.ResumeMailbox) SystemMessage()      {}

var (
	//restartingMessage     interface{} = &Restarting{}
	stoppingMessage       interface{} = &Stopping{}
	stoppedMessage        interface{} = &Stopped{}
	receiveTimeoutMessage interface{} = &ReceiveTimeout{}
	startedMessage        interface{} = &Started{}
	stopMessage           interface{} = &Stop{}
	resumeMailboxMessage  interface{} = &mailbox.ResumeMailbox{}
	suspendMailboxMessage interface{} = &mailbox.SuspendMailbox{}
)
