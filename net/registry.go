package net

import (
	"sync"
)

type Registry struct {
	records map[uint32]interface{} // id -> v
	sync.RWMutex
}

var (
	_default_registry Registry
)

func init() {
	_default_registry.init()
}

func (r *Registry) init() {
	r.records = make(map[uint32]interface{})
}

// register a user
func (r *Registry) Register(id uint32, v interface{}) {
	r.Lock()
	// exists
	_, ok := r.records[id]
	r.records[id] = v
	r.Unlock()

	if !ok {
		AddConn(1)
	}
}

// unregister a user
func (r *Registry) Unregister(id uint32) {
	r.Lock()
	// exists
	_, ok := r.records[id]
	delete(r.records, id)
	r.Unlock()

	if ok {
		SubConn(1)
	}
}

// query a user
func (r *Registry) Query(id uint32) (x interface{}) {
	r.RLock()
	x = r.records[id]
	r.RUnlock()
	return
}

// return count of online users
func (r *Registry) Count() (count int) {
	r.RLock()
	count = len(r.records)
	r.RUnlock()
	return
}

func Register(id uint32, v interface{}) {
	_default_registry.Register(id, v)
}

func Unregister(id uint32) {
	_default_registry.Unregister(id)
}

func Query(id uint32) interface{} {
	return _default_registry.Query(id)
}

func Count() int {
	return _default_registry.Count()
}
