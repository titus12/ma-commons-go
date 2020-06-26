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

const DEFAULT_REPLICAS = 150

type uints []uint32

func (x uints) Len() int           { return len(x) }
func (x uints) Less(i, j int) bool { return x[i] < x[j] }
func (x uints) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }

// 不存在任何节点，进行Get操作返回错误
var ErrEmptyCircle = errors.New("empty circle")

// 节点构建出来不允许外部进行修改
type Node struct {
	ip       string // ip地址
	port     int    //端口
	hostName string //节点名称
	weight   int    //权重

	ipAndPort string //避免重复组织
}

func (node *Node) Ip() string {
	return node.ip
}

func (node *Node) Port() int {
	return node.port
}

func (node *Node) HostName() string {
	return node.hostName
}

func (node *Node) Weight() int {
	return node.weight
}

// 节点的id就是ip:port
func (node *Node) Id() string {
	return node.ipAndPort
}

func (node *Node) String() string {
	return fmt.Sprintf("{IP:%s,PORT:%d,NAME:%s,WEIGHT:%d", node.ip, node.port, node.hostName, node.weight)
}

// 构建一个节点出来
func NewNode(ip string, port int, name string, weight int) *Node {
	if weight <= 0 {
		weight = 1
	}
	return &Node{
		ip:        ip,
		port:      port,
		hostName:  name,
		weight:    weight,
		ipAndPort: fmt.Sprintf("%s:%d", ip, port),
	}
}

type Consistent struct {
	circle           map[uint32]*Node //节点在环上的位置
	members          map[string]*Node //真实节点，有多少真实节点
	sortedHashes     uints            //给环上的hash排序
	NumberOfReplicas int              //每个节点产生多少虚拟节点
	count            int64            //有多少节点
	UseFnv           bool
	sync.RWMutex
}

// 构建一致性哈希对像
func NewConsistent() *Consistent {
	c := new(Consistent)
	c.NumberOfReplicas = DEFAULT_REPLICAS
	c.circle = make(map[uint32]*Node)
	c.members = make(map[string]*Node)
	return c
}

// 根据索引生成代表虚拟节点的字符串
func (c *Consistent) eltKey(elt string, idx int) string {
	return strconv.Itoa(idx) + elt
}

// 添加节点
func (c *Consistent) Add(node *Node) bool {
	c.Lock()
	defer c.Unlock()

	// 已经加入过的就不需要加入了
	if _, ok := c.members[node.ipAndPort]; ok {
		return false
	}

	count := c.NumberOfReplicas * node.Weight()
	for i := 0; i < count; i++ {
		c.circle[c.hashKey(c.eltKey(node.ipAndPort, i))] = node
	}
	c.members[node.ipAndPort] = node
	c.updateSortedHashes()
	c.count++

	return true
}

// 移除节点，节点id=ipAndPort
func (c *Consistent) Remove(ipAndPort string) {
	c.Lock()
	defer c.Unlock()

	// 如果不存在节点，直接返回
	if _, ok := c.members[ipAndPort]; !ok {
		return
	}

	node := c.members[ipAndPort]
	count := c.NumberOfReplicas * node.Weight()
	for i := 0; i < count; i++ {
		delete(c.circle, c.hashKey(c.eltKey(ipAndPort, i)))
	}
	delete(c.members, ipAndPort)
	c.updateSortedHashes()
	c.count--
}

// 所有节点
func (c *Consistent) Members() []*Node {
	c.RLock()
	defer c.RUnlock()
	var m []*Node
	for _, v := range c.members {
		m = append(m, v)
	}
	return m
}

// 获取所在的节点
func (c *Consistent) Get(name string) (*Node, error) {
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
func (c *Consistent) GetTwo(name string) (*Node, *Node, error) {
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
	var b *Node
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
func (c *Consistent) GetN(name string, n int) ([]*Node, error) {
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
		res   = make([]*Node, 0, n)
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

func sliceContainsMember(set []*Node, member *Node) bool {
	for _, m := range set {
		if m == member {
			return true
		}
	}
	return false
}
