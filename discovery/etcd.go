// etcd的服务发现, 这里服务发现采用v2的api，v3版本的可视化界面没有找到,etcd的服务发现一般都是
// 开发环境，有可视化界面操作会很方便，后继etcd也会作为利用容器进行发现的操作，另外启动一个容器
// 专门用作服务注册，把注册的信息添加到etcd，而每个节点上的服务只需要etcd进行监控即可
// +build etcd

package discovery

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"github.com/titus12/ma-commons-go/utils"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/server"
	"github.com/titus12/ma-commons-go/utils/ctxfunc"
	"github.com/titus12/ma-commons-go/utils/diectrl"
)

const (
	DefaultRoot = "backends"
)

type etcdimpl struct {
	diectrl.ControlV2            //死亡控制器
	nodeNofity        chan *Node // 节点通知，节点发现后会放到这个通道

	//cancel context.CancelFunc
	cli etcdclient.Client
}

func newImpl() *etcdimpl {
	if len(server.EtcdHosts) <= 0 {
		logrus.Panic("系统没有提供ETCD的连接配置.....")
	}

	cfg := etcdclient.Config{
		Endpoints: server.EtcdHosts,
		Transport: etcdclient.DefaultTransport,
	}

	cli, err := etcdclient.New(cfg)
	if err != nil {
		logrus.WithError(err).Panic("连接etcd错误")
	}

	etcd := &etcdimpl{
		nodeNofity: make(chan *Node, defaultListenSize),
		cli:        cli,
	}

	etcd.Init(1)
	return etcd
}

// 加载已有的节点
func (e *etcdimpl) mustLoad() {
	kAPI := etcdclient.NewKeysAPI(e.cli)
	var (
		resp *etcdclient.Response
		err  error
	)
	ctxfunc.Timeout30s(func(ctx context.Context) {
		resp, err = kAPI.Get(ctx, DefaultRoot, &etcdclient.GetOptions{Recursive: true})
	})

	if err != nil {
		logrus.WithError(err).Panicf("etcd加载 %s 下的节点失败......", DefaultRoot)
	}

	// 根节点只能是目录
	if !resp.Node.Dir {
		logrus.Panicf("etcd的根节点一定要是目录， %s", DefaultRoot)
	}

	for _, etcdnode := range resp.Node.Nodes {
		if etcdnode.Dir {
			serviceName := utils.Trim(filepath.Base(etcdnode.Key))
			for _, nodeInfo := range etcdnode.Nodes {
				if nodeInfo.Dir {
					continue
				}
				uid := utils.Trim(filepath.Base(nodeInfo.Key))
				ipAndPort := utils.Trim(nodeInfo.Value)

				ip, port, err := e.parseValue(ipAndPort)
				if err != nil {
					logrus.WithError(err).Panicf("etcd解析叶子节点出错，%s", ipAndPort)
				}

				node := &Node{
					ServiceName: serviceName,
					Uid:         uid,
					Ip:          ip,
					Port:        int32(port),
				}

				node.Off = !checkNet(node)

				e.nodeNofity <- node
			}
		}
	}
}

// 解析叶子节点上的值，分离出ip地址，端口
func (e *etcdimpl) parseValue(value string) (ip string, port int, err error) {
	if !utils.IsIpAndPort(value) {
		err = errors.New("不是正常的ip:port格式,格式错误")
		return
	}

	strArray := strings.Split(value, ":")
	ip = strArray[0]

	port, err = strconv.Atoi(strArray[1])
	return
}

func (e *etcdimpl) Listen() <-chan *Node {
	e.mustLoad() //加载已有的服务节点
	go e.watcher()
	return e.nodeNofity
}

func (e *etcdimpl) watcher() {
	defer func() {
		e.Done()
	}()

	kAPI := etcdclient.NewKeysAPI(e.cli)
	w := kAPI.Watcher(DefaultRoot, &etcdclient.WatcherOptions{Recursive: true})
	for {
		select {
		case <-e.WaitDie():
			return
		default:
			resp, err := w.Next(e.Ctx())
			if err != nil {
				logrus.WithError(err).Error("etcd 监控发生错误....")
				continue
			}

			if resp.Node.Dir {
				continue
			}

			var (
				nodeKey   string
				nodeValue string
			)

			switch resp.Action {
			case "set", "create", "update", "compareAndSwap":
				nodeKey = resp.Node.Key
				nodeValue = resp.Node.Value
			case "delete":
				nodeKey = resp.PrevNode.Key
				nodeValue = resp.PrevNode.Value
			}

			serviceName := utils.Trim(filepath.Base(filepath.Dir(nodeKey)))
			uid := utils.Trim(filepath.Base(nodeKey))
			ipAndPort := utils.Trim(nodeValue)

			ip, port, err := e.parseValue(ipAndPort)
			if err != nil {
				logrus.WithError(err).Errorf("etcd解析叶子节点失败 key: %s, value: %s",
					nodeKey, nodeValue)
				continue
			}

			node := &Node{
				ServiceName: serviceName,
				Uid:         uid,
				Ip:          ip,
				Port:        int32(port),
			}
			node.Off = !checkNet(node)

			e.nodeNofity <- node
		}
	}
}

func (e *etcdimpl) Stop() <-chan struct{} {
	return e.CloseAndEnd(nil)
}
