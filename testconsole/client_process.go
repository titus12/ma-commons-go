package testconsole

// 所有操作都是当场建立连接，然后关闭释放

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/titus12/ma-commons-go/services"


	"github.com/coreos/etcd/clientv3"
	"github.com/sirupsen/logrus"

	"github.com/titus12/ma-commons-go/utils"

	"github.com/golang/protobuf/proto"
	"github.com/titus12/ma-commons-go/testconsole/testmsg"
	"google.golang.org/grpc"
)

//cfg := etcdclient.Config{
//Endpoints:   etcdHosts,
//DialTimeout: DefaultTimeout,
//}
//c, err := etcdclient.New(cfg)

var (
	etcdCli *clientv3.Client
	once    sync.Once
)

type Node struct {
	Key    string
	Addr   string `json:"addr"`   //ip:port
	Status int32  `json:"status"` // 节点状态
}

func (n *Node) String() string {
	return "{key:" + n.Key + ",status:" + strconv.Itoa(int(n.Status)) + "}"
}

const etcdRoot = "/root/backend/gameser"

func LocalRunRequest(msg proto.Message) {
	// 拿到所有节点
	nodes := GetAllNodeData(etcdRoot)
	if len(nodes) <= 0 {
		fmt.Println("集群中不存在节点....至少需要一个节点....")
		return
	}

	if !IsStable(nodes) {
		fmt.Println("集群中的所有节点必须都在Running状态中...", nodes)
		return
	}
	logicMsg, ok := msg.(*testmsg.LocalRun)
	if !ok {
		fmt.Println("不能正常转换成 testmsg.LocalRun 消息")
		return
	}
	// 确定 senderId与targetId在同一个节点上
	if logicMsg.TargetId <= 0 {
		fmt.Println("目标actor不能为空.....TargetId:", logicMsg.TargetId)
		return
	}

	targetNode := HitNode(nodes, logicMsg.TargetId)

	if logicMsg.SenderId > 0 {
		senderNode := HitNode(nodes, logicMsg.SenderId)
		if targetNode.Key != senderNode.Key || targetNode.Addr != senderNode.Addr {
			// 发送actor与目标actor不在同一个节点上
			fmt.Println("发送者 与 目标 不在同一个节点上", senderNode, targetNode)
			return
		}
	}

	err := ExecGrpcCall(targetNode, func(ctx context.Context, cli testmsg.TestConsoleClient) error {
		logicMsg.ReqId = utils.GenUuidWithUint64()

		// 打印发出去的消息，发到哪个节点，当前节点情况

		resp, err1 := cli.LocalRunRequest(ctx, logicMsg)
		if err1 == nil {
			fmt.Println("LocalRunResponse Complete...", resp)
		}
		return err1
	})

	if err != nil {
		fmt.Println("向远程节点调用错误....", err)
		return
	}
}

// 获取etcd客户端
func GetEtcdCli() *clientv3.Client {
	once.Do(func() {
		etcdHosts := []string{"127.0.0.1:2379"}

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

func GetAllNodeData(etcdRootPath string) (nodes []*Node) {
	cli := GetEtcdCli()
	resp, err := cli.Get(context.Background(), etcdRootPath, clientv3.WithPrefix())
	if err != nil {
		logrus.WithError(err).Panic("在etcd中获取节点数据失败")
	}
	for _, ev := range resp.Kvs {
		keyStr := utils.BytesToString(ev.Key)
		if keyStr == etcdRootPath {
			continue
		}
		info := &Node{}
		err := json.Unmarshal(ev.Value, info)
		if err != nil {
			logrus.WithError(err).Panic("节点数据不是json")
		}

		if info.Status <= 0 || len(info.Addr) <= 0 {
			logrus.Panicf("节点数据不正确 key=%s, val=%s", keyStr, ev.Value)
		}

		info.Key = filepath.Base(keyStr)
		nodes = append(nodes, info)
	}
	return
}

// 集群是否稳定
func IsStable(nodes []*Node) bool {
	for _, v := range nodes {
		if v.Status != services.ServiceStatusRunning {
			return false
		}
	}
	return true
}

// 命中节点
func HitNode(nodes []*Node, actorId int64) *Node {
	if len(nodes) <= 0 {
		logrus.Panicf("HitNode fail, 没有任何节点数据")
	}

	ring := utils.NewConsistent()
	for _, v := range nodes {
		ring.Add(utils.NewNodeKey(v.Key, 1))
	}

	nodeKey, err := ring.Get(fmt.Sprintf("%d", actorId))
	if err != nil {
		logrus.WithError(err).Panicf("HitNode ring.Get(%d) fail", actorId)
	}

	for _, v := range nodes {
		if v.Key == nodeKey.Key() {
			return v
		}
	}
	logrus.Panicf("HitNode Fail, 没有命中到节点....")
	return nil
}

// 执行gprc的运行
func ExecGrpcCall(node *Node, fn func(ctx context.Context, cli testmsg.TestConsoleClient) error) error {
	conn, err := grpc.Dial(node.Addr, grpc.WithTimeout(2*time.Second), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer conn.Close()
	defer cancel()

	cli := testmsg.NewTestConsoleClient(conn)

	err = fn(ctx, cli)
	if err != nil {
		return err
	}
	return nil
}
