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
	update := make([]*GCacheElement, 0, len(elements))
	for _, elem := range elements {
		if elem.cache == nil {
			continue
		}
		if err = elem.cache.setElements(elem); err == nil {
			update = append(update, elem)
		} else {
			rollback = true
			break
		}
	}
	//
	if rollback {
		for _, elem := range update {
			if rErr := elem.cache.setElements(elem); rErr != nil {
				log.Errorf("rollback err: %v", rErr)
				break
			}
		}
	}
	return
}
