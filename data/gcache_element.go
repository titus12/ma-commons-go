package data

import (
	"fmt"
	"github.com/golang/protobuf/proto"
)

type ACCESS_MODE int8

const (
	READONLY   ACCESS_MODE = 0
	READ_WRITE ACCESS_MODE = 1
)

type GCacheData interface {
	GetAttr(key string, in ...interface{}) (out interface{}, ok bool)
}

type GCollection interface {
	Clone() GCollection
}

type GCacheElement struct {
	cache *GCache
	key   GCacheKey
	data  GCacheData
}

func (g *GCacheElement) Get() (GCacheData, error) {
	if g.data == nil {
		return nil, nil
	}
	msg, ok := g.data.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("cast proto.message error")
	}
	clone := proto.Clone(msg).(GCacheData)
	return clone, nil
}

func (g *GCacheElement) GetCollection(mode ACCESS_MODE) (GCollection, error) {
	if g.data == nil {
		return nil, nil
	}
	col, ok := g.data.(GCollection)
	if !ok {
		return nil, fmt.Errorf("cast gcollection error")
	}
	clone := col.Clone()
	return clone, nil
}
