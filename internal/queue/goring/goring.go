// 传统环状队列
package goring

import (
	"sync"
	"sync/atomic"
)

type ringBuffer struct {
	buffer []interface{}
	head   int64
	tail   int64
	mod    int64
}

type Queue struct {
	len     int64
	content *ringBuffer
	lock    sync.Mutex
}

func New(initialSize int64) *Queue {
	return &Queue{
		content: &ringBuffer{
			buffer: make([]interface{}, initialSize),
			head:   0,
			tail:   0,
			mod:    initialSize,
		},
		len: 0,
	}
}

func (q *Queue) Push(item interface{}) {
	q.lock.Lock()
	defer q.lock.Unlock()

	c := q.content
	c.tail = (c.tail + 1) % c.mod
	if c.tail == c.head {
		var fillFactor int64 = 2
		newLen := c.mod * fillFactor
		newBuff := make([]interface{}, newLen)

		for i := int64(0); i < c.mod; i++ {
			buffIndex := (c.tail + i) % c.mod
			newBuff[i] = c.buffer[buffIndex]
		}

		newContent := &ringBuffer{
			buffer: newBuff,
			head:   0,
			tail:   c.mod,
			mod:    newLen,
		}
		q.content = newContent
	}
	atomic.AddInt64(&q.len, 1)
	q.content.buffer[q.content.tail] = item
}

func (q *Queue) Length() int64 {
	return atomic.LoadInt64(&q.len)
}

func (q *Queue) Empty() bool {
	return q.Length() == 0
}

//func (q *Queue) Pop() (interface{}, bool) {
//	if q.Empty() {
//		return nil, false
//	}
//
//	q.lock.Lock()
//	defer q.lock.Unlock()
//
//	c := q.content
//	c.head = (c.head + 1) % c.mod
//	res := c.buffer[c.head]
//	c.buffer[c.head] = nil
//	atomic.AddInt64(&q.len, -1)
//	return res, true
//}

func (q *Queue) Pop() interface{} {
	if q.Empty() {
		return nil
	}

	q.lock.Lock()
	defer q.lock.Unlock()

	c := q.content
	c.head = (c.head + 1) % c.mod
	res := c.buffer[c.head]
	c.buffer[c.head] = nil
	atomic.AddInt64(&q.len, -1)
	return res
}
