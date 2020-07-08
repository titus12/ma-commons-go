package utils

import (
	"errors"
	"fmt"
	"hash/crc32"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
)

const DefaultReplicas = 150

type uints []uint32

func (x uints) Len() int           { return len(x) }
func (x uints) Less(i, j int) bool { return x[i] < x[j] }
func (x uints) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// 不存在任何节点，进行Get操作返回错误
var ErrEmptyCircle = errors.New("empty circle")

type NodeKey struct {
	key    string
	weight int //权重
}

func (node NodeKey) Clone() *NodeKey {
	return &node
}

func (node *NodeKey) Weight() int {
	return node.weight
}

// 节点的id就是ip:port
func (node *NodeKey) Key() string {
	return node.key
}

func (node *NodeKey) String() string {
	return fmt.Sprintf("key: %s, weight: %d", node.key, node.weight)
}

// 构建一个节点出来
func NewNodeKey(key string, weight int) *NodeKey {
	if weight <= 0 {
		weight = 1
	}
	return &NodeKey{
		key:    key,
		weight: weight,
	}
}

type Consistent struct {
	circle           map[uint32]*NodeKey //节点在环上的位置
	members          map[string]*NodeKey //真实节点，有多少真实节点
	sortedHashes     uints               //给环上的hash排序
	NumberOfReplicas int                 //每个节点产生多少虚拟节点
	count            int64               //有多少节点
	UseFnv           bool
	sync.RWMutex
}

// 构建一致性哈希对像
func NewConsistent() *Consistent {
	c := new(Consistent)
	c.NumberOfReplicas = DefaultReplicas
	c.circle = make(map[uint32]*NodeKey)
	c.members = make(map[string]*NodeKey)
	return c
}

func (c *Consistent) Clone() *Consistent {
	c.Lock()
	defer c.Unlock()

	clone := NewConsistent()
	//clone.sortedHashes = c.sortedHashes
	clone.NumberOfReplicas = c.NumberOfReplicas
	clone.count = c.count
	clone.UseFnv = c.UseFnv
	if c.sortedHashes != nil {
		clone.sortedHashes = make(uints, len(c.sortedHashes), cap(c.sortedHashes))
		copy(clone.sortedHashes, c.sortedHashes)
	}

	// 克隆成员
	for k, v := range c.members {
		clone.members[k] = v.Clone()
	}

	for k, v := range c.circle {
		node := clone.members[v.key]
		clone.circle[k] = node
	}
	return clone
}

// 根据索引生成代表虚拟节点的字符串
func (c *Consistent) eltKey(elt string, idx int) string {
	return strconv.Itoa(idx) + elt
}

// 添加节点
func (c *Consistent) Add(node *NodeKey) bool {
	c.Lock()
	defer c.Unlock()

	// 已经加入过的就不需要加入了
	if _, ok := c.members[node.key]; ok {
		return false
	}

	count := c.NumberOfReplicas * node.Weight()
	for i := 0; i < count; i++ {
		c.circle[c.hashKey(c.eltKey(node.key, i))] = node
	}
	c.members[node.key] = node
	c.updateSortedHashes()
	c.count++

	return true
}

// 移除节点，节点id=ipAndPort
func (c *Consistent) Remove(key string) bool {
	c.Lock()
	defer c.Unlock()

	// 如果不存在节点，直接返回
	if _, ok := c.members[key]; !ok {
		return ok
	}

	node := c.members[key]
	count := c.NumberOfReplicas * node.Weight()
	for i := 0; i < count; i++ {
		delete(c.circle, c.hashKey(c.eltKey(key, i)))
	}
	delete(c.members, key)
	c.updateSortedHashes()
	c.count--
	return true
}

// 所有节点
func (c *Consistent) Members() []*NodeKey {
	c.RLock()
	defer c.RUnlock()
	var m []*NodeKey
	for _, v := range c.members {
		m = append(m, v)
	}
	return m
}

// 获取所在的节点
func (c *Consistent) Get(name string) (*NodeKey, error) {
	c.RLock()
	defer c.RUnlock()
	if len(c.circle) == 0 {
		return nil, ErrEmptyCircle
	}
	key := c.hashKey(name)
	i := c.search(key)
	return c.circle[c.sortedHashes[i]], nil
}

func (c *Consistent) search(key uint32) (i int) {
	f := func(x int) bool {
		return c.sortedHashes[x] > key
	}
	i = sort.Search(len(c.sortedHashes), f)
	if i >= len(c.sortedHashes) {
		i = 0
	}
	return
}

// 获取相邻二个节点
// todo: 小心使用，未测试
func (c *Consistent) GetTwo(name string) (*NodeKey, *NodeKey, error) {
	c.RLock()
	defer c.RUnlock()
	if len(c.circle) == 0 {
		return nil, nil, ErrEmptyCircle
	}
	key := c.hashKey(name)
	i := c.search(key)
	a := c.circle[c.sortedHashes[i]]

	if c.count == 1 {
		return a, nil, nil
	}

	start := i
	var b *NodeKey
	for i = start + 1; i != start; i++ {
		if i >= len(c.sortedHashes) {
			i = 0
		}
		b = c.circle[c.sortedHashes[i]]
		if b != a {
			break
		}
	}
	return a, b, nil
}

// 获取相领N个节点
// TODO: 小心使用,未测试
func (c *Consistent) GetN(name string, n int) ([]*NodeKey, error) {
	c.RLock()
	defer c.RUnlock()

	if len(c.circle) == 0 {
		return nil, ErrEmptyCircle
	}

	if c.count < int64(n) {
		n = int(c.count)
	}

	var (
		key   = c.hashKey(name)
		i     = c.search(key)
		start = i
		res   = make([]*NodeKey, 0, n)
		elem  = c.circle[c.sortedHashes[i]]
	)

	res = append(res, elem)

	if len(res) == n {
		return res, nil
	}

	for i = start + 1; i != start; i++ {
		if i >= len(c.sortedHashes) {
			i = 0
		}
		elem = c.circle[c.sortedHashes[i]]
		if !sliceContainsMember(res, elem) {
			res = append(res, elem)
		}
		if len(res) == n {
			break
		}
	}

	return res, nil
}

func (c *Consistent) hashKey(key string) uint32 {
	if c.UseFnv {
		return c.hashKeyFnv(key)
	}
	return c.hashKeyCRC32(key)
}

func (c *Consistent) hashKeyCRC32(key string) uint32 {
	if len(key) < 64 {
		var scratch [64]byte
		copy(scratch[:], key)
		return crc32.ChecksumIEEE(scratch[:len(key)])
	}
	return crc32.ChecksumIEEE([]byte(key))
}

func (c *Consistent) hashKeyFnv(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

func (c *Consistent) updateSortedHashes() {
	hashes := c.sortedHashes[:0]
	//reallocate if we're holding on to too much (1/4th)
	if cap(c.sortedHashes)/(c.NumberOfReplicas*4) > len(c.circle) {
		hashes = nil
	}
	for k := range c.circle {
		hashes = append(hashes, k)
	}
	sort.Sort(hashes)
	c.sortedHashes = hashes
}

func sliceContainsMember(set []*NodeKey, member *NodeKey) bool {
	for _, m := range set {
		if m == member {
			return true
		}
	}
	return false
}
