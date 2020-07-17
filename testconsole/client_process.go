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

// 集群处于不稳定状态
// 控制台拿到node_key所代表的节点
// 1.确保node_key在集群中是running状态
// 2.确保集群当前一定是超过1个以上的节点，并且有一个节点是处于Pending状态下
func LocalRunPendingRequest(msg proto.Message) {
	nodes := GetAllNodeData(etcdRoot)
	fmt.Println("节点数量:", len(nodes))

	if !MustOnePendingNode(nodes) {
		fmt.Println("集群中必须要有一个节点处于pending状态")
		return
	}

	logicMsg, ok := msg.(*testmsg.LocalRunPending)
	if !ok {
		fmt.Println("不能正常转换成 testmsg.LocalRunPending 消息")
		return
	}

	if logicMsg.TargetId <= 0 {
		fmt.Println("目标actor不能为空.....TargetId:", logicMsg.TargetId)
		return
	}

	targetNode := HitNodeWithRunningWithActor(nodes, logicMsg.TargetId)
	if targetNode == nil {
		fmt.Println("集群中不存在指定的节点")
		return
	}

	if targetNode.Status != services.ServiceStatusRunning {
		fmt.Println("节点状态不在Running")
		return
	}

	if logicMsg.SenderId > 0 {
		fmt.Println("暂时不支持senderId有值")
		return
	}

	err := ExecGrpcCall(targetNode, func(ctx context.Context, cli testmsg.TestConsoleClient) error {
		logicMsg.ReqId = utils.GenUuidWithUint64()

		// 打印发出去的消息，发到哪个节点，当前节点情况
		resp, err1 := cli.LocalRunPendingRequest(ctx, logicMsg)
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

	targetNode := HitNodeWithActor(nodes, logicMsg.TargetId)

	if logicMsg.SenderId > 0 {
		senderNode := HitNodeWithActor(nodes, logicMsg.SenderId)
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

// 集群必须只存在一个pending状态的节点
func MustOnePendingNode(nodes []*Node) bool {
	var count int
	for _, v := range nodes {
		if v.Status == services.ServiceStatusPending {
			count++
		}
	}

	if count == 1 {
		return true
	}
	return false
}

func HitNodeWithNodeKey(nodes []*Node, nodeKey string) *Node {
	if len(nodes) <= 0 {
		logrus.Panicf("HitNodeWithActor fail, 没有任何节点数据")
	}

	for _, v := range nodes {
		if v.Key == nodeKey {
			return v
		}
	}

	return nil
}

// 从running状态中的节点中通过actorid命中一个节点
func HitNodeWithRunningWithActor(nodes []*Node, actorId int64) *Node {
	if len(nodes) <= 0 {
		logrus.Panicf("HitNodeWithActor fail, 没有任何节点数据")
	}

	ring := utils.NewConsistent()
	for _, v := range nodes {
		if v.Status == services.ServiceStatusRunning {
			ring.Add(utils.NewNodeKey(v.Key, 1))
		}
	}

	nodeKey, err := ring.Get(fmt.Sprintf("%d", actorId))
	if err != nil {
		logrus.WithError(err).Panicf("HitNodeWithActor ring.Get(%d) fail", actorId)
	}

	for _, v := range nodes {
		if v.Key == nodeKey.Key() {
			return v
		}
	}
	return nil
}

// 命中节点
func HitNodeWithActor(nodes []*Node, actorId int64) *Node {
	if len(nodes) <= 0 {
		logrus.Panicf("HitNodeWithActor fail, 没有任何节点数据")
	}

	ring := utils.NewConsistent()
	for _, v := range nodes {
		ring.Add(utils.NewNodeKey(v.Key, 1))
	}

	nodeKey, err := ring.Get(fmt.Sprintf("%d", actorId))
	if err != nil {
		logrus.WithError(err).Panicf("HitNodeWithActor ring.Get(%d) fail", actorId)
	}

	for _, v := range nodes {
		if v.Key == nodeKey.Key() {
			return v
		}
	}
	logrus.Panicf("HitNodeWithActor Fail, 没有命中到节点....")
	return nil
}

// 执行gprc的运行
func ExecGrpcCall(node *Node, fn func(ctx context.Context, cli testmsg.TestConsoleClient) error) error {
	conn, err := grpc.Dial(node.Addr, grpc.WithTimeout(2*time.Second), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5 *time.Minute)

	defer conn.Close()
	defer cancel()

	cli := testmsg.NewTestConsoleClient(conn)

	fmt.Println("GRPC调用开始时间...", time.Now())
	err = fn(ctx, cli)
	fmt.Println("GRPC调用结束时间...", time.Now())
	if err != nil {
		return err
	}
	return nil
}
