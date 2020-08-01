package data

import (
	"github.com/golang/protobuf/proto"
)

type GCacheComponentData struct {
	Value interface{}
}

func NewGCacheComponentData() *GCacheComponentData {
	return &GCacheComponentData{}
}

func (c *GCacheComponentData) Init(key string, value interface{}) {
	c.Value = value
}

func (c *GCacheComponentData) GetValue() interface{} {
	return c.Value
}

func (c *GCacheComponentData) Clone() (GCacheComponent, error) {
	clone := &GCacheComponentData{}
	if m, ok := c.Value.(proto.Message); ok {
		clone.Value = proto.Clone(m)
	} else {
		clone.Value = c.Value
	}
	return clone, nil
}

type GCacheComponent interface {
	Init(key string, value interface{})
	GetValue() interface{}
	Clone() (GCacheComponent, error)
}
