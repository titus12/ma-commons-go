package data

import (
	"fmt"
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
	gCache *GCache
	key    GCacheKey
	cmpMap map[string]GCacheComponent
	old    GCacheComponent
	new    GCacheComponent
}

func newGCacheElement(cache *GCache, cmpMap map[string]GCacheComponent, key GCacheKey, data GCacheComponent) (*GCacheElement, error) {
	element := &GCacheElement{gCache: cache, cmpMap: cmpMap, key: key, old: data}
	clone, err := element.old.Clone()
	if err != nil {
		return nil, err
	}
	element.new = clone
	return element, nil
}

func (g *GCacheElement) GetComponent() GCacheComponent {
	return g.new
}

func (g *GCacheElement) GetCollection(mode ACCESS_MODE) (GCollection, error) {
	if g.old == nil {
		return nil, nil
	}
	col, ok := g.old.(GCollection)
	if !ok {
		return nil, fmt.Errorf("cast gcollection errors")
	}
	clone := col.Clone()
	return clone, nil
}
