package state

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	etcdclient "github.com/coreos/etcd/client"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	NUMBER_PREFIX_NODE = "/number_prefixs"
)

var (
	once          sync.Once
	defaultServer server
)

func Init(root, serviceId string, etcdHosts []string) {
	once.Do(func() {
		defaultServer.init(root, serviceId, etcdHosts)
	})
}

type server struct {
	root          string
	serviceId     string
	client        etcdclient.Client
	numberPrefixs map[string]bool
	numberDatas   map[string]map[string]int
	stringDatas   map[string]map[string]string
	mu            sync.RWMutex
}

func (p *server) init(root, serviceId string, etcdHosts []string) {
	p.root = root
	p.serviceId = serviceId

	p.numberPrefixs = make(map[string]bool)
	p.numberDatas = make(map[string]map[string]int)
	p.stringDatas = make(map[string]map[string]string)

	cfg := etcdclient.Config{
		Endpoints: etcdHosts,
		Transport: etcdclient.DefaultTransport,
	}

	c, err := etcdclient.New(cfg)
	if err != nil {
		log.Panic(err)
		os.Exit(-1)
	}

	p.client = c

	p.loadNumberPrefixs(p.root + NUMBER_PREFIX_NODE)

	//
	p.load()
}

func (p *server) isNumberType(name string) bool {
	for k := range p.numberPrefixs {
		if strings.HasPrefix(name, k) {
			return true
		}
	}

	return false
}

func (p *server) path(key string) (category, service string, err error) {
	params := strings.Split(key, "/")
	if len(params) != 4 {
		err = fmt.Errorf("Split %v len not equal 4", key)
		return
	}

	category = params[2]
	service = params[3]
	return
}

func (p *server) remove(key string) {
	category, service, err := p.path(key)
	if err != nil {
		log.Error(err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if ok := p.isNumberType(category); ok {
		if _, ok := p.numberDatas[category]; ok {
			delete(p.numberDatas[category], service)
		}
	} else {
		if _, ok := p.stringDatas[category]; ok {
			delete(p.numberDatas[category], service)
		}
	}
}

func (p *server) getInt(category, service string) (value int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ok := p.isNumberType(category); ok {
		if _, ok := p.numberDatas[category]; ok {
			value = p.numberDatas[category][service]
		}
	}

	return
}

func (p *server) getStr(category, service string) (value string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if ok := p.isNumberType(category); !ok {
		if _, ok := p.stringDatas[category]; ok {
			value = p.stringDatas[category][service]
		}
	}

	return
}

func (p *server) execGroupInt(category string, f func(map[string]int)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var group map[string]int
	if ok := p.isNumberType(category); ok {
		group = p.numberDatas[category]
	}

	f(group)

	return
}

func (p *server) execGroupStr(category string, f func(map[string]string)) {
	p.mu.Lock()
	defer p.mu.Unlock()

	var group map[string]string
	if ok := p.isNumberType(category); !ok {
		group = p.stringDatas[category]
	}

	f(group)

	return
}

func (p *server) set(key, value string) {

	category, service, err := p.path(key)
	if err != nil {
		log.Error(err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if ok := p.isNumberType(category); ok {
		num, err := strconv.Atoi(value)
		if err != nil {
			log.Errorf("Set %v = %v strconv.Atoi err %v", key, value, err)
			return
		}

		if _, ok := p.numberDatas[category]; !ok {
			p.numberDatas[category] = make(map[string]int)
		}

		p.numberDatas[category][service] = num
	} else {
		if _, ok := p.stringDatas[category]; !ok {
			p.stringDatas[category] = make(map[string]string)
		}

		p.stringDatas[category][service] = value
	}
}

func (p *server) update(key, value string) {
	kAPI := etcdclient.NewKeysAPI(p.client)

	_, err := kAPI.Set(context.Background(), key, value, nil)
	if err != nil {
		log.Errorf("kapi set %v=%v err %v", key, value, err)
	}
}

func (p *server) loadNumberPrefixs(filepath string) {
	kAPI := etcdclient.NewKeysAPI(p.client)

	// get the keys under directory
	log.Infof("reading number types from:%v", filepath)
	resp, err := kAPI.Get(context.Background(), filepath, nil)
	if err != nil {
		log.Error(err)
		return
	}

	// validation check
	if resp.Node.Dir {
		log.Error("types is not a node")
		return
	}

	// split types
	types := strings.Split(resp.Node.Value, " ")
	for _, v := range types {
		p.numberPrefixs[v] = true
	}

	log.Infof("reading number types :%v", resp.Node.Value)
}

func (p *server) load() {
	kAPI := etcdclient.NewKeysAPI(p.client)

	resp, err := kAPI.Get(context.Background(), p.root, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		log.Error(err)
		return
	}

	if !resp.Node.Dir {
		return
	}

	for _, node := range resp.Node.Nodes {
		if node.Dir {
			for _, v := range node.Nodes {
				p.set(v.Key, v.Value)
			}
		}
	}

	go p.watcher()
}

func (p *server) watcher() {
	kAPI := etcdclient.NewKeysAPI(p.client)
	w := kAPI.Watcher(p.root, &etcdclient.WatcherOptions{Recursive: true})
	for {
		resp, err := w.Next(context.Background())
		if err != nil {
			log.Error(err)
			continue
		}

		if resp.Node.Dir {
			continue
		}

		switch resp.Action {
		case "set", "create", "update", "compareAndSwap":
			p.set(resp.Node.Key, resp.Node.Value)
		case "delete":
			p.remove(resp.PrevNode.Key)
		}
	}
}

func ServiceVarInt(category, service string) int {
	return defaultServer.getInt(category, service)
}

func ServiceVarStr(category, service string) string {
	return defaultServer.getStr(category, service)
}

func ExecuteGroupInt(category string, f func(map[string]int)) {
	defaultServer.execGroupInt(category, f)
}

func ExecuteGroupStr(category string, f func(map[string]string)) {
	defaultServer.execGroupStr(category, f)
}

func SetServiceVar(key, value string) {
	defaultServer.update(defaultServer.root+"/"+key+"/"+defaultServer.serviceId, value)
}
