package control

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/titus12/ma-commons-go/utils"

	"github.com/coreos/etcd/clientv3"

	"github.com/sirupsen/logrus"

	"github.com/titus12/ma-commons-go/services"
	"github.com/titus12/ma-commons-go/setting"

	"google.golang.org/grpc"

	control "github.com/titus12/ma-commons-go/control/proto"
)

var (
	etcdCli *clientv3.Client
	once    sync.Once
)

type NodeInfo struct {
	services.NodeData
	Key string
}

type service struct{}

func (*service) StopNode(ctx context.Context, req *control.Request) (resp *control.Response, err error) {
	logrus.Infof("ctrl StopNode start stop node nodeKey %s", setting.Key)

	// 请求的关闭的节点名称与节点不匹配
	if req.NodeKey != setting.Key {
		err = fmt.Errorf("the requested closed node name does not match the node")
		logrus.Errorf("StopNode req nodekey %s curr nodekey %s err %v", req.NodeKey, setting.Key, err)
		return
	}

	addr := fmt.Sprintf("%s:%d", setting.Ip, setting.Port)
	if req.Addr != addr {
		err = fmt.Errorf("the requested closed node address does not match")
		logrus.Errorf("StopNode req addr %s curr addr %s err %v", req.Addr, addr, err)
		return
	}

	err = services.StopNode()
	return
}

func Register(server *grpc.Server) {
	control.RegisterControlServiceServer(server, &service{})
}

// 获取etcd客户端
func GetEtcdCli() *clientv3.Client {
	once.Do(func() {
		etcdHosts := setting.EtcdHosts

		cfg := clientv3.Config{
			Endpoints:   etcdHosts,
			DialTimeout: 30 * time.Second,
		}

		var err error
		etcdCli, err = clientv3.New(cfg)
		if err != nil {
			logrus.WithError(err).Panic("实始化etcd失败....")
		}
	})
	return etcdCli
}

func GetAllNodeData(serviceName string) (nodeInfos []*NodeInfo) {
	etcdPath := setting.EtcdRoot + "/" + serviceName

	cli := GetEtcdCli()
	resp, err := cli.Get(context.Background(), etcdPath, clientv3.WithPrefix())
	if err != nil {
		logrus.Panicf("在etcd中获取节点数据失败 %v", err)
	}
	for _, ev := range resp.Kvs {
		keyStr := utils.BytesToString(ev.Key)
		if keyStr == etcdPath {
			continue
		}
		info := &NodeInfo{}
		err := json.Unmarshal(ev.Value, info)
		if err != nil {
			logrus.Panicf("节点数据不是json %v", err)
		}

		if info.Status <= 0 || len(info.Addr) <= 0 {
			logrus.Panicf("节点数据不正确 key=%s, val=%s", keyStr, ev.Value)
		}

		info.Key = filepath.Base(keyStr)
		nodeInfos = append(nodeInfos, info)
	}
	return
}

// 执行grpc调用
func ExecGrpcCall(nodeInfo *NodeInfo, fn func(ctx context.Context, cli control.ControlServiceClient) error) error {
	conn, err := grpc.Dial(nodeInfo.Addr, grpc.WithTimeout(2*time.Second), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)

	defer conn.Close()
	defer cancel()

	cli := control.NewControlServiceClient(conn)

	st := time.Now()
	err = fn(ctx, cli)

	if err != nil {
		logrus.Errorf("ExecGrpcCall err %v", err)
		return err
	}

	logrus.Infof("ExecGrpcCall TimeLen %v", time.Now().Sub(st))

	return nil
}
