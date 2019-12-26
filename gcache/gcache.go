package gcache

import (
	"github.com/titus12/ma-commons-go/gcache/bigcache"
	"time"
	"unsafe"
)

type GCache struct {
	name   string
	cache  *bigcache.BigCache
	config *GCacheConfig
}

type IGCacheRWConfig interface {
	DebugMode() bool
}

type GCacheConfig struct {
	TimeToLiveSeconds time.Duration
	MaxEntries        int32
	MaxEntrySize      int32
	OnRemoveEvent     func(key string, entry []byte)
	RWConfig          IGCacheRWConfig
}

func NewCache(name string, config *GCacheConfig) *GCache {
	gCache := &GCache{}
	gCache.name = name
	gCache.config = config
	bigConfig := bigcache.Config{
		Shards:      1024,
		LifeWindow:  config.TimeToLiveSeconds,
		CleanWindow: 5 * time.Minute,
		// rps * lifeWindow, used only in initial memory allocation
		MaxEntriesInWindow: int(config.MaxEntrySize),
		// max entry size in bytes, used only in initial memory allocation
		MaxEntrySize: int(config.MaxEntries),
		// prints information about additional memory allocation
		Verbose: true,
		// cache will not allocate more memory than this limit, value in MB
		// if value is reached then the oldest entries can be overridden for the new ones
		// 0 value means no size limit
		HardMaxCacheSize: 0,
		// callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A bitmask representing the reason will be returned.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		OnRemove: config.OnRemoveEvent,
		// OnRemoveWithReason is a callback fired when the oldest entry is removed because of its expiration time or no space left
		// for the new entry, or because delete was called. A constant representing the reason will be passed through.
		// Default value is nil which means no callback and it prevents from unwrapping the oldest entry.
		// Ignored if OnRemove is specified.
		OnRemoveWithReason: nil,
	}
	gCache.cache, _ = bigcache.NewBigCache(bigConfig)
	return gCache
}

func (p *GCache) Queries(keys ...GCacheKey) (gCaches []*GCacheElement, err error) {
	for _, key := range keys {
		bytes, err := p.cache.Get(key.GetPrimary())
		if err != nil {
			return nil, err
		}
		var data *GCacheData
		if bytes == nil {
			if gCacheManager.rwHandler != nil {
				bytes = gCacheManager.rwHandler.ReadRawData(key, p.config.RWConfig)
				if err = p.cache.Set(key.GetPrimary(), bytes); err != nil {
					return
				}
				data = *(**GCacheData)(unsafe.Pointer(&bytes))
			}
		} else {
			data = *(**GCacheData)(unsafe.Pointer(&bytes))
		}
		element := &GCacheElement{p, key, *data}
		gCaches = append(gCaches, element)
	}
	return
}
