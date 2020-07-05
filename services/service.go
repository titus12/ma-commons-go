package services

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"sync"
	"sync/atomic"
)

import (
	cons "github.com/titus12/ma-commons-go/utils"
)

var (
	ErrNoNodes = errors.New("no nodes")
)

const (
	StatusServiceNone = iota
	StatusServicePending
	StatusServiceRunning
	StatusServiceStopping
)

type service struct {
	name               string
	stableConsistent   *cons.Consistent
	unstableConsistent *cons.Consistent
	nodes              []node
	mu                 sync.RWMutex
	idx                uint32
}

func newService(name string) *service {
	service := &service{name: name}
	service.stableConsistent = cons.NewConsistent()
	service.unstableConsistent = cons.NewConsistent()
	return service
}

func (s *service) isCompleted(exclude string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key == exclude {
			continue
		}
		if !v.ready {
			return false
		}
	}
	return true
}

func (s *service) upsertNode(node node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key == node.key {
			if node.data.status == StatusServiceRunning && v.data.status != StatusServicePending {
				return fmt.Errorf("UpsertNode old.data %v -> new.data %v", v, node)
			}
			if v.data.status == StatusServicePending && node.data.status == StatusServiceRunning {
				s.stableConsistent.Add(cons.NewNodeKey(node.key, 1))
				log.Debugf("upsertNode stableConsistent add key %v", node.key)
			}
			v.data = node.data
			v.conn = node.conn
			return nil
		}
	}
	s.nodes = append(s.nodes, node)
	if node.data.status == StatusServicePending {
		s.unstableConsistent.Add(cons.NewNodeKey(node.key, 1))
		log.Debugf("upsertNode unstableConsistent add key %v", node.key)
	} else if node.data.status == StatusServiceStopping {
		s.unstableConsistent.Remove(node.key)
		log.Debugf("upsertNode unstableConsistent remove key %v", node.key)
	}
	return nil
}

func (s *service) delNode(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.nodes {
		if v.key == key { // deletion
			s.stableConsistent.Remove(key)
			s.unstableConsistent.Remove(key)
			s.nodes = append(s.nodes[:k], s.nodes[k+1:]...)
			v.conn.Close()
			log.Infof("service removed: %v", key)
			return
		}
	}
}

func (s *service) getNode(id string) (node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k := range s.nodes {
		if s.nodes[k].key == id {
			return s.nodes[k], nil
		}
	}
	return node{}, fmt.Errorf("node %v id %v is not exist", s.name, id)
}

func (s *service) getNodeWithRoundRobin(path string) (node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	idx := int(atomic.AddUint32(&s.idx, 1)) % count
	return s.nodes[idx], nil
}

func (s *service) getNodeWithHash(path string, hash int) (node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	idx := hash % len(s.nodes)
	return s.nodes[idx], nil
}

func (s *service) getNodeWithConsistentHash(path string, id string) (node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	nodeKey, err := s.stableConsistent.Get(id)
	if err != nil {
		return node{}, fmt.Errorf("consistent err %v", err)
	}
	for _, v := range s.nodes {
		if v.key == nodeKey.Key() {
			return v, nil
		}
	}
	return node{}, fmt.Errorf("no found node")
}

func (s *service) getNodes(path string) (nodes []node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy(nodes, s.nodes)
	return nodes
}
