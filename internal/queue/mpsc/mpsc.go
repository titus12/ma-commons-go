// 提供多生产者，单消费者的无锁队列
package mpsc

import (
	"sync/atomic"
	"unsafe"
)

//节点
type node struct {
	next *node
	val  interface{}
}

type Queue struct {
	head *node
	tail *node
	len  int64
}

func New() *Queue {
	q := &Queue{}
	stub := &node{}
	q.head = stub
	q.tail = stub
	return q
}

func (q *Queue) Push(x interface{}) {
	n := new(node)
	n.val = x

	prev := (*node)(atomic.SwapPointer((*unsafe.Pointer)(unsafe.Pointer(&q.head)), unsafe.Pointer(n)))
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&prev.next)), unsafe.Pointer(n))
	atomic.AddInt64(&q.len, 1)
}

func (q *Queue) Pop() interface{} {
	tail := q.tail
	next := (*node)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&tail.next))))
	if next != nil {
		q.tail = next
		v := next.val
		next.val = nil

		atomic.AddInt64(&q.len, -1)
		return v
	}
	return nil
}

func (q *Queue) Length() int64 {
	return atomic.LoadInt64(&q.len)
}

func (q *Queue) Empty() bool {
	tail := q.tail
	next := (*node)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&tail.next))))
	return next == nil
}
