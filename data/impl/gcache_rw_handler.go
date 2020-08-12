package impl

import (
	"github.com/titus12/ma-commons-go/data"
)

type DATA_SOURCE int32

const (
	SOURCE_LOCAL    DATA_SOURCE = 0
	SOURCE_REDIS    DATA_SOURCE = 1
	SOURCE_DATABASE DATA_SOURCE = 2
)

type GCacheRWHandler struct {
}

type GCacheRWConfig struct {
	DataSource DATA_SOURCE
	debugMode  bool
}

func (g *GCacheRWConfig) DebugMode() bool {
	return g.debugMode
}

func (g *GCacheRWHandler) ReadData(key interface{}, config data.IGCacheRWConfig) (data.GCacheData, error) {
	cfg := config.(*GCacheRWConfig)
	if cfg.DataSource == SOURCE_LOCAL {
		return nil, nil
	}
	switch cfg.DataSource {
	case SOURCE_REDIS:

	case SOURCE_DATABASE:

	}
	return nil, nil
}

func (g *GCacheRWHandler) ReadRawData(key interface{}, config data.IGCacheRWConfig) (data.GCacheData, error) {
	cfg := config.(*GCacheRWConfig)
	if cfg.DataSource == SOURCE_LOCAL {
		return nil, nil
	}
	switch cfg.DataSource {
	case SOURCE_REDIS:

	case SOURCE_DATABASE:

	}
	return nil, nil
}

func (g *GCacheRWHandler) WriteData(key interface{}, data data.GCacheData, config data.IGCacheRWConfig) error {

	return nil
}
