package utils

import (
	"github.com/pkg/errors"
	"sync/atomic"
)

const (
	stateCanUse = 0
	stateDone = 1
)


type SafeChan struct {
	status int32
	c chan interface{}
}

func NewSafeChan(queueLen int) *SafeChan {
	return &SafeChan{status:stateCanUse,c:make(chan interface{}, queueLen)}
}

func (safe *SafeChan) Chan() <- chan interface{} {
	return safe.c
}

func (safe *SafeChan) Close() {
	defer PrintPanicStack()
	if atomic.CompareAndSwapInt32(&safe.status, stateCanUse, stateDone) {
		close(safe.c)
	}
}

func (safe *SafeChan) Write(m interface{}) error {
	defer PrintPanicStack()
	if atomic.LoadInt32(&safe.status) == stateDone {
		return errors.New("already close")
	}
	safe.c <- m
	return nil
}
