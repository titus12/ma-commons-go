package actor

import (
	"sync/atomic"

	"github.com/titus12/ma-commons-go/actor/mailbox"
)

// 一个actor的处理过程，每一个生成的actor都会有一个，实现Process
type ActorProcess struct {
	mailbox mailbox.Mailbox
	dead    int32
}

func newActorProcess(mailbox mailbox.Mailbox) *ActorProcess {
	return &ActorProcess{mailbox: mailbox}
}

// 发送用户消息
func (ref *ActorProcess) SendUserMessage(pid *PID, message interface{}) {
	ref.mailbox.PostUserMessage(message)
}

// 发送系统消息
func (ref *ActorProcess) SendSystemMessage(pid *PID, message interface{}) {
	ref.mailbox.PostSystemMessage(message)
}

// 停止actor
func (ref *ActorProcess) Stop(pid *PID) {
	atomic.StoreInt32(&ref.dead, 1)
	ref.SendSystemMessage(pid, stopMessage)
}
