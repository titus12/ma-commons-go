package mailbox

// 邮箱的内部队列接口
type queue interface {
	Push(interface{})
	Pop() interface{}
}
