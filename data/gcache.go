package data

import (
	"fmt"
	caching "github.com/titus12/gcache"
	"github.com/titus12/gcache/cache"
	"time"
)

type GCache struct {
	name   string
	cache  *caching.GCache
	config *GCacheConfig
}

type IGCacheRWConfig interface {
	DebugMode() bool
}

type GCacheConfig struct {
	Shards            uint32
	TimeToLiveSeconds time.Duration
	CleanInterval     time.Duration
	MaxEntrySize      int32
	EvictType         string
	OnRemoveEvent     func(key interface{}, value interface{}, reason cache.RemoveReason)
	RWConfig          IGCacheRWConfig
}

func NewCache(name string, config *GCacheConfig) *GCache {
	gCache := &GCache{}
	gCache.name = name
	gCache.config = config
	gconfig := caching.Config{
		Shards:        int(config.Shards),
		Expiration:    config.TimeToLiveSeconds,
		CleanInterval: config.CleanInterval,
		MaxEntrySize:  int(config.MaxEntrySize),
		EvictType:     config.EvictType,
		OnRemoveFunc:  config.OnRemoveEvent,
		Logger:        caching.DefaultLogger(),
	}
	gCache.cache, _ = caching.NewGCache(gconfig)
	return gCache
}

func (p *GCache) Queries(keys ...GCacheKey) (gCaches []*GCacheElement, err error) {
	for _, key := range keys {
		obj, ok := p.cache.Get(key.GetPrimary())
		var gCacheData GCacheData
		if !ok {
			if gCacheManager.rwHandler != nil {
				gCacheData, err = gCacheManager.rwHandler.ReadData(key, p.config.RWConfig)
				if err != nil {
					return nil, err
				}
				if ok = p.cache.Set(key.GetPrimary(), obj); !ok {
					return nil, fmt.Errorf("queries object, set object to data failed")
				}
				//data = *(**GCacheData)(unsafe.Pointer(&bytes))
			}
		} else {
			gCacheData = obj.(GCacheData)
		}
		element := &GCacheElement{p, key, gCacheData}
		gCaches = append(gCaches, element)
	}
	return
}
