package data

import (
	"github.com/golang/protobuf/proto"
)

type GCacheComponentData struct {
	key   string
	value interface{}
}

func NewGCacheComponentData() *GCacheComponentData {
	return &GCacheComponentData{}
}

func (c *GCacheComponentData) Init(key string, value interface{}) {
	c.key = key
	c.value = value
}

func (c *GCacheComponentData) GetValue() (string, interface{}) {
	return c.key, c.value
}

func (c *GCacheComponentData) Clone() (GCacheComponent, error) {
	clone := &GCacheComponentData{}
	if m, ok := c.value.(proto.Message); ok {
		clone.value = proto.Clone(m)
	} else {
		clone.value = c.value
	}
	clone.key = c.key
	return clone, nil
}

type GCacheComponent interface {
	Init(key string, value interface{})
	GetValue() (string, interface{})
	Clone() (GCacheComponent, error)
}
