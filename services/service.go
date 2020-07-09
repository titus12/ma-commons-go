package services

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	cons "github.com/titus12/ma-commons-go/utils"
)

type serviceType int32

var (
	ErrNoNodes = errors.New("no nodes")
)

// 服务，一个服务下管理多个节点，这此节点为这个服务提供一致的服务，可以认为一服务是提供同一类型服务的集群

// bug: 改成大小的，包外可以访问类型(已改）
type Service struct {
	name               string //服务名
	serviceType        serviceType
	stableConsistent   *cons.Consistent // 稳定环，也称老环
	unstableConsistent *cons.Consistent // 不稳定环，也称新环
	nodes              []*node          // 服务拥有的节点
	mu                 sync.RWMutex
	idx                uint32

	// 由服务管理的actor系统设置进来的回调函数,用于触发actor的牵移操作
	callback func(nodeName string, nodeStatus int32) error
}

func newService(name string) *Service {
	service := &Service{name: name}
	service.stableConsistent = cons.NewConsistent()
	service.unstableConsistent = cons.NewConsistent()
	return service
}

// 是否完成，检查服务里所有节点是否正常牵移完数据
func (s *Service) isCompleted(exclude string) int32 {
	s.mu.RLock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key == exclude {
			continue
		}
		if v.transfer == TransferStatusFail {
			return TransferStatusFail
		} else if v.transfer == TransferStatusNone {
			return TransferStatusNone
		}
	}
	return TransferStatusSucc
}

// 给节点设置牵移状态
func (s *Service) transfer(nodeName string, status int32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key != nodeName {
			continue
		}
		v.transfer = status
		return true
	}
	return false
}

// 更新或者插入节点，如果节点存在，则更新，不存在则添加
// 操作都会checkStatus,checkStatus是对节点的状态进行检查，判断加入到新环还是老环
func (s *Service) upsertNode(node *node) error {
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
		err := s.checkStatus(nil, node)
		if err != nil {
			return err
		}
		s.nodes = append(s.nodes, node)
	} else {
		err := s.checkStatus(s.nodes[idx], node)
		if err != nil {
			return err
		}
		if node.data.addr != s.nodes[idx].data.addr {
			s.nodes[idx].conn = node.conn
		}
		//node.transfer = s.nodes[idx].transfer
		s.nodes[idx].data = node.data
	}
	return nil
}

// 添加节点，已经存在的节点不能添加
func (s *Service) addNode(node *node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.nodes {
		if v.key == node.key {
			return fmt.Errorf("upsertNode node %v already exist", node.key)
		}
	}
	err := s.checkStatus(nil, node)
	if err != nil {
		return err
	}
	s.nodes = append(s.nodes, node)
	return nil
}

// 更新节点，节点不存在不能更新
func (s *Service) updateNode(node *node) error {
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

// 检查节点状态,拿老的节点与新的节点进行比较，判断节点在环上的情况，并根据不同情况
// 作出对环的操作。（内部调用，都是在有锁的方法中调用，不用加锁）
func (s *Service) checkStatus(oldNode *node, newNode *node) error {
	var oldStatus int8 = ServiceStatusNone
	if oldNode != nil {
		oldStatus = oldNode.data.status
	}
	newStatus := newNode.data.status
	if oldStatus == newStatus {
		//return fmt.Errorf("checkStatus node %v status %v duplicated", newNode.key, StatusServiceName[newStatus])
		return errStatusDuplicated
	}

	switch newStatus {
	case ServiceStatusNone:
	case ServiceStatusPending:
		if oldStatus == ServiceStatusNone {
			//事件 - 不稳定环加节点
			s.unstableConsistent = s.stableConsistent.Clone()
			s.unstableConsistent.Add(cons.NewNodeKey(newNode.key, 1))
			return nil
		}
	case ServiceStatusRunning:
		if oldStatus == ServiceStatusPending {
			//事件 克隆 稳定环 = 不稳定环
			s.stableConsistent = s.unstableConsistent.Clone()
			return nil
		}
	case ServiceStatusStopping:
		//事件 - 不稳定环删节点
		if ok := s.unstableConsistent.Remove(newNode.key); !ok {
			return fmt.Errorf("checkStatus node %v %v unstableConsistent remove key not exist", newNode.key, StatusServiceName[newStatus])
		}
		return nil
	}
	return fmt.Errorf("checkStatus node %v no regulation %v -> %v ", newNode.key, StatusServiceName[oldStatus], StatusServiceName[newStatus])
}

// 删除节点
func (s *Service) delNode(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.nodes {
		if v.key == key { // deletion
			s.stableConsistent.Remove(key)
			s.unstableConsistent.Remove(key)
			s.nodes = append(s.nodes[:k], s.nodes[k+1:]...)
			v.conn.Close()
			log.Infof("Service removed: %v", key)
			return
		}
	}
}

// 获取节点
func (s *Service) getNode(id string) (node, error) {
	s.mu.RLock()
	defer s.mu.Unlock()
	for k := range s.nodes {
		if s.nodes[k].key == id {
			return *s.nodes[k], nil
		}
	}
	return node{}, fmt.Errorf("node %v id %v is not exist", s.name, id)
}

func (s *Service) getNodeWithRoundRobin() (node, error) {
	s.mu.RLock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	idx := int(atomic.AddUint32(&s.idx, 1)) % count
	return *s.nodes[idx], nil
}

func (s *Service) getNodeWithHash(hash int) (node, error) {
	s.mu.RLock()
	defer s.mu.Unlock()
	count := len(s.nodes)
	if count == 0 {
		return node{}, ErrNoNodes
	}
	idx := hash % len(s.nodes)
	return *s.nodes[idx], nil
}

// 在一致性哈希的环上获取节点，拿到的节点数据是一个copy，说明拿到的是当前时刻的节点，通过
// 这份数据进行逻缉操作时是不影响原环上的节点
func (s *Service) getNodeWithConsistentHash(id string, isStable bool) (node, error) {
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
			return *v, nil
		}
	}
	return node{}, fmt.Errorf("no found node id %v isStable %v", id, isStable)
}

func (s *Service) getNodes() []node {
	s.mu.RLock()
	defer s.mu.Unlock()
	nodes := make([]node, 0, len(s.nodes))
	for _, v := range s.nodes {
		nodes = append(nodes, *v)
	}
	//copy(nodes, s.nodes)
	return nodes
}

/////////////////////////////////////////////////////////////////
// 实现actor包提供的接口

func (s *Service) ServiceName() string {
	return s.name
}

func (s *Service) SetCallBack(fn func(nodeKey string, nodeStatus int32) error) {
	s.callback = fn
}

func (s *Service) IsLocalWithStableRing(id int64) (local bool, nodeKey string, nodeStatus int32, conn *grpc.ClientConn, err error) {
	node, err := s.getNodeWithConsistentHash(fmt.Sprintf("%d", id), true)
	if err != nil {
		return false, "", -1, nil, err
	}
	return node.isLocal, node.key, int32(node.data.status), node.conn, nil
}

func (s *Service) IsLocalWithUnstableRing(id int64) (local bool, nodeKey string, nodeStatus int32, conn *grpc.ClientConn, err error) {
	node, err := s.getNodeWithConsistentHash(fmt.Sprintf("%d", id), false)
	if err != nil {
		return false, "", -1, nil, err
	}
	return node.isLocal, node.key, int32(node.data.status), node.conn, nil
}
