package services

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
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

var StatusServiceName = map[int8]string{
	StatusServiceNone:     "NONE",
	StatusServicePending:  "PENDING",
	StatusServiceRunning:  "RUNNING",
	StatusServiceStopping: "STOPPING",
}

type service struct {
	name               string
	stableConsistent   *cons.Consistent
	unstableConsistent *cons.Consistent
	nodes              []*node
	mu                 sync.RWMutex
	idx                uint32
	callback           func(nodeName string, nodeStatus int32) error
}

func newService(name string) *service {
	service := &service{name: name}
	service.stableConsistent = cons.NewConsistent()
	service.unstableConsistent = cons.NewConsistent()
	return service
}

func (s *service) isCompleted(exclude string) bool {
	s.mu.RLock()
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
	add, err := checkAddNode(&node)
	if add {
		s.unstableConsistent.Add(cons.NewNodeKey(node.key, 1))
		s.nodes = append(s.nodes, node)
	}

	if node.data.status == StatusServicePending {
		s.unstableConsistent.Add(cons.NewNodeKey(node.key, 1))
		log.Debugf("upsertNode unstableConsistent add key %v", node.key)
	} else if node.data.status == StatusServiceStopping {
		s.unstableConsistent.Remove(node.key)
		log.Debugf("upsertNode unstableConsistent remove key %v", node.key)
	}
	return nil
}

func (s *service) addNode(node *node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key == node.key {
			return fmt.Errorf("addNode node %v already exist", node.key)
		}
	}
	err := s.checkStatus(nil, node)
	if err != nil {
		return err
	}
	s.nodes = append(s.nodes, node)
	return nil
}

func (s *service) updateNode(node *node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := -1
	for i, v := range s.nodes {
		if v.key == node.key {
			idx = i
			break
		}
	}
	if idx == -1 {
		return fmt.Errorf("updateNode node %v is not exist", node.key)
	}
	err := s.checkStatus(s.nodes[idx], node)
	if err != nil {
		return err
	}
	s.nodes[idx] = node
	return nil
}

func (s *service) checkStatus(oldNode *node, newNode *node) error {
	var oldStatus int8 = StatusServiceNone
	if oldNode != nil {
		oldStatus = oldNode.data.status
	}
	newStatus := newNode.data.status
	if oldStatus == newStatus {
		return fmt.Errorf("updateNode node %v status %v duplicated", newNode.key, StatusServiceName[newStatus])
	}

	switch newStatus {
	case StatusServiceNone:
		if oldStatus == StatusServiceNone {
			return nil
		}
	case StatusServicePending:
		if oldStatus == StatusServiceNone {
			//事件 - 不稳定环加节点
			s.unstableConsistent.Add(cons.NewNodeKey(newNode.key, 1))
			return nil
		}
	case StatusServiceRunning:
		if oldStatus == StatusServicePending {
			//事件 克隆 稳定环 = 不稳定环
			s.stableConsistent = s.unstableConsistent.Clone()
			return nil
		}
	case StatusServiceStopping:
		//事件 - 不稳定环删节点
		s.unstableConsistent.Remove(newNode.key)
		return nil
	}
	return fmt.Errorf("updateNode node %v no regulation %v -> %v ", newNode.key, StatusServiceName[oldStatus], StatusServiceName[newStatus])
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
	s.mu.RLock()
	defer s.mu.Unlock()
	for k := range s.nodes {
		if s.nodes[k].key == id {
			return s.nodes[k], nil
		}
	}
	return node{}, fmt.Errorf("node %v id %v is not exist", s.name, id)
}

func (s *service) getNodeWithRoundRobin() (node, error) {
	s.mu.RLock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	idx := int(atomic.AddUint32(&s.idx, 1)) % count
	return s.nodes[idx], nil
}

func (s *service) getNodeWithHash(hash int) (node, error) {
	s.mu.RLock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	idx := hash % len(s.nodes)
	return s.nodes[idx], nil
}

func (s *service) getNodeWithConsistentHash(id string, isStable bool) (node, error) {
	s.mu.RLock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	var nodeKey *cons.NodeKey
	var err error
	if isStable {
		nodeKey, err = s.stableConsistent.Get(id)
	} else {
		nodeKey, err = s.unstableConsistent.Get(id)
	}

	if err != nil {
		return node{}, fmt.Errorf("consistent id %v isStable %v err %v", id, isStable, err)
	}
	for _, v := range s.nodes {
		if v.key == nodeKey.Key() {
			return v, nil
		}
	}
	return node{}, fmt.Errorf("no found node id %v isStable %v", id, isStable)
}

func (s *service) getNodes() []node {
	s.mu.RLock()
	defer s.mu.Unlock()
	len := len(s.nodes)
	nodes := make([]node, len, len)
	copy(nodes, s.nodes)
	return nodes
}

/////////////////////////////////////////////////////////////////
// Wrappers

func (s *service) ServiceName() string {
	return s.name
}

func (s *service) SetCallback(fn func(nodeName string, nodeStatus int32) error) {
	s.callback = fn
}

func (s *service) IsLocalWithStableRing(id int64) (local bool, nodeName string, nodeStatus int8, conn *grpc.ClientConn, err error) {
	node, err := s.getNodeWithConsistentHash(fmt.Sprintf("%d", id), true)
	if err != nil {
		return false, "", -1, nil, err
	}
	return node.isLocal, node.key, node.data.status, node.conn, nil
}

func (s *service) IsLocalWithUnstableRing(id int64) (local bool, nodeName string, nodeStatus int8, conn *grpc.ClientConn, err error) {
	node, err := s.getNodeWithConsistentHash(fmt.Sprintf("%d", id), false)
	if err != nil {
		return false, "", -1, nil, err
	}
	return node.isLocal, node.key, node.data.status, node.conn, nil
}
