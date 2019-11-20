package net

import "sync"

type ConnectCount struct {
	count int32
	sync.RWMutex
}

var (
	_default_connect_count ConnectCount
)

func init() {
	_default_connect_count.Init()
}

func (p *ConnectCount) Init() {
	//nothing to do
}

func (p *ConnectCount) Count() (count int32) {
	p.Lock()
	count = p.count
	p.Unlock()
	return
}

func (p *ConnectCount) AddConn(num int32) {
	p.Lock()
	p.count += num
	p.Unlock()
}

func (p *ConnectCount) SubConn(num int32) {
	p.Lock()
	p.count -= num
	p.Unlock()
}

func ConnCount() int32 {
	return _default_connect_count.Count()
}

func AddConn(num int32) {
	_default_connect_count.AddConn(num)
}

func SubConn(num int32) {
	_default_connect_count.SubConn(num)
}
