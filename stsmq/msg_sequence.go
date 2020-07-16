package stsmq

// 消息序列
// 队列发送消息，如果发送的go只有一个，通常情况下是能保证发送的有序进行的，但很多情况下我们的发送go可能不只一个
// 尤其在多核情况下，如果在不特意变更发送go的情况下，会存在多个发送go，如果对消息的发送序列有严格要求时，则调用
// 者通过SerMessageQueue的方法构建出一个序列，通过序列进行消息的发送，可以保证消息的发送是有序的。
type MsgSequence struct {
	msgs []*InnerSerMsg
	q    SerMessageQueue
}

func (this *MsgSequence) Push(msg *InnerSerMsg) *MsgSequence {
	this.msgs = append(this.msgs, msg)
	return this
}

// 执行消息序列
func (this *MsgSequence) Exec() {
	for _, msg := range this.msgs {
		switch impl_q := this.q.(type) {
		case *redis_queue:
			box := impl_q.hitbox()
			impl_q.pushtobox(box, msg)
		}
		this.q.Push(msg)
	}
}
