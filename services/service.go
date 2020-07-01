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

type service struct {
	stableConsistent   *cons.Consistent
	unstableConsistent *cons.Consistent
	nodes              []node
	mu                 sync.RWMutex
	idx                uint32
}

func newService() *service {
	service := &service{}
	service.stableConsistent = cons.NewConsistent()
	service.unstableConsistent = cons.NewConsistent()
	return service
}

func (s *service) addNode(node node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key == node.key {
			v.data = node.data
			v.conn = node.conn
			return
		}
	}
	s.nodes = append(s.nodes, node)
	s.stableConsistent.Add(&cons.NodeKey{node.key, 1})
}

func (s *service) delNode(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.nodes {
		if v.key == key { // deletion
			s.stableConsistent.Remove(key)
			s.nodes = append(s.nodes[:k], s.nodes[k+1:]...)
			v.conn.Close()
			log.Infof("service removed: %v", key)
			return
		}
	}
}

func (s *service) getNode(path string, id string) (node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fullPath := pathJoin(path, id)
	for k := range s.nodes {
		if s.nodes[k].key == fullPath {
			return s.nodes[k], nil
		}
	}
	return node{}, fmt.Errorf("node %v id %v is not exist", path, id)
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
