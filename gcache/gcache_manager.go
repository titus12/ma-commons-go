package gcache

import (
	_ "github.com/titus12/ma-commons-go/gcache/impl"
)

type IGCacheRWHandler interface {
	ReadData(key interface{}, config IGCacheRWConfig) (GCacheData, error)
	WriteData(key interface{}, data GCacheData, config IGCacheRWConfig) error
}

var gCacheManager *GCacheManager

/*var once sync.Once
func GetInstance() *GCacheManager {
	once.Do(func() {
		gCacheManager = &GCacheManager{}
	})
	return gCacheManager
}*/

type GCacheManager struct {
	rwHandler IGCacheRWHandler
}

func init() {
	gCacheManager = &GCacheManager{}
}

func (g *GCacheManager) init(rwHandler IGCacheRWHandler) {
	g.rwHandler = rwHandler
}

func GetInstance() *GCacheManager {
	return gCacheManager
}

func Init(rwHandler IGCacheRWHandler) {
	gCacheManager.init(rwHandler)
}
