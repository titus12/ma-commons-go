package actor

import (
	"sync"
	"sync/atomic"
)

// Future的登记结构
type FutureRegistryValue struct {
	sequenceID int64
	items      sync.Map
}

// Future的管理者对像，每个产生的Future都会在这里登记
var FutureRegistry = &FutureRegistryValue{}

// 根据id获取到一个Future
func (fr *FutureRegistryValue) Get(key int64) *Future {
	value, ok := fr.items.Load(key)
	if !ok {
		return nil
	}
	return value.(*Future)
}

func (fr *FutureRegistryValue) SetIfAbsent(key int64, value *Future) bool {
	_, ok := fr.items.LoadOrStore(key, value)
	return ok
}

func (fr *FutureRegistryValue) Remove(key int64) {
	fr.items.Delete(key)
}

func (fr *FutureRegistryValue) NextId() int64 {
	counter := atomic.AddInt64(&fr.sequenceID, 1)
	return counter
}
