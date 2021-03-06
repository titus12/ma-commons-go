package data

import (
	"fmt"
	caching "github.com/titus12/gcache"
	"github.com/titus12/gcache/cache"
	"time"
)

type GCache struct {
	name         string
	cache        *caching.GCache
	config       *GCacheConfig
	sourceReader func(key string) (map[string]GCacheComponent, error)
	sourceWriter func(key string, components []GCacheComponent) error
}

type GCacheConfig struct {
	Shards            uint32
	TimeToLiveSeconds time.Duration
	CleanInterval     time.Duration
	MaxEntrySize      int32
	EvictType         string
	OnRemoveEvent     func(key interface{}, value interface{}, reason cache.RemoveReason)
}

type OptFn func(cache *GCache)

func NewGCache(name string, config *GCacheConfig, opts ...OptFn) (*GCache, error) {
	gCache := &GCache{}
	gCache.name = name
	gCache.config = config
	gConfig := caching.Config{
		Shards:        int(config.Shards),
		Expiration:    config.TimeToLiveSeconds,
		CleanInterval: config.CleanInterval,
		MaxEntrySize:  int(config.MaxEntrySize),
		EvictType:     config.EvictType,
		OnRemoveFunc:  config.OnRemoveEvent,
		Logger:        caching.DefaultLogger(),
	}
	var err error
	gCache.cache, err = caching.NewGCache(gConfig)
	for _, opt := range opts {
		opt(gCache)
	}
	return gCache, err
}

func WithSourceReader(reader func(key string) (map[string]GCacheComponent, error)) OptFn {
	return func(cache *GCache) {
		cache.sourceReader = reader
	}
}

func WithSourceWriter(writer func(key string, components []GCacheComponent) error) OptFn {
	return func(cache *GCache) {
		cache.sourceWriter = writer
	}
}

func (p *GCache) GetElements(keys ...GCacheKey) (elements []*GCacheElement, err error) {
	if len(keys) <= 0 {
		return nil, fmt.Errorf("keys len is zeor")
	}
	obj, ok := p.cache.Get(keys[0].GetPrimary())
	var gCacheCmpMap map[string]GCacheComponent
	if !ok {
		if p.sourceReader != nil {
			gCacheCmpMap, err = p.sourceReader(keys[0].GetPrimary())
			if err != nil {
				return nil, err
			}
			if ok = p.cache.Set(keys[0].GetPrimary(), gCacheCmpMap); !ok {
				return nil, fmt.Errorf("queries object, set object to data failed")
			}
			//data = *(**GCacheData)(unsafe.Pointer(&bytes))
		}
	} else {
		gCacheCmpMap = obj.(map[string]GCacheComponent)
	}
	for _, key := range keys {
		primaryKey, _ := key.GetElementPrimary().(string)
		element, err := newGCacheElement(p, gCacheCmpMap, key, gCacheCmpMap[primaryKey])
		if err != nil {
			return nil, err
		}
		elements = append(elements, element)
	}
	return
}

func (p *GCache) setElements(elements []*GCacheElement) error {
	update := make([]GCacheComponent, 0, len(elements))
	var key string
	for _, elem := range elements {
		key = elem.key.GetPrimary()
		elemPrimary := elem.key.GetElementPrimary().(string)
		elem.cmpMap[elemPrimary] = elem.new
		update = append(update, elem.new)
	}
	return p.sourceWriter(key, update)
}

func (p *GCache) rollback(elements []*GCacheElement) error {
	update := make([]GCacheComponent, 0, len(elements))
	var key string
	for _, elem := range elements {
		key = elem.key.GetPrimary()
		elemPrimary := elem.key.GetElementPrimary().(string)
		elem.cmpMap[elemPrimary] = elem.old
		update = append(update, elem.new)
	}
	return p.sourceWriter(key, update)
}
