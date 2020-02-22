package queue

// 对队接口定义,goring与mpsc是其实现
type Queue interface {
	Push(interface{})
	Pop() interface{}
	Empty() bool
	Length() int64
}
