package data

import (
	log "github.com/sirupsen/logrus"
)

type GCacheTransaction struct {
	//update []*GCacheElement
}

func NewGCacheTransaction() *GCacheTransaction {
	return &GCacheTransaction{}
}

func (t *GCacheTransaction) Commit(elements []*GCacheElement) (err error) {
	var rollback bool
	update := make(map[string][]*GCacheElement)
	for _, elem := range elements {
		if elem.gCache == nil {
			continue
		}
		key := elem.gCache.name + elem.key.GetPrimary()
		if _, ok := update[key]; ok {
			update[key] = append(update[key], elem)
		} else {
			update[key] = []*GCacheElement{elem}
		}
	}

	for _, v := range update {
		if err = v[0].gCache.setElements(v); err != nil {
			rollback = true
			break
		}
	}
	//
	if rollback {
		for _, v := range update {
			if rErr := v[0].gCache.rollback(v); rErr != nil {
				log.Errorf("rollback err: %v", rErr)
				break
			}
		}
	}
	return
}
