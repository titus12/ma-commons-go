package testconsole

// 所有操作都是当场建立连接，然后关闭释放

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/pkg/errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/titus12/ma-commons-go/services"

	"github.com/coreos/etcd/clientv3"
	"github.com/sirupsen/logrus"

	"github.com/titus12/ma-commons-go/utils"

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


func QueryRequest(msgstr interface{}) {
	str, ok := msgstr.(string)
	if !ok {
		fmt.Println("不能正常转换成 string 消息")
		return
	}

	strarr := strings.Split(str, " ")
	if len(strarr) != 2 {
		fmt.Println("命令格式错误, 需要提供二个数字参数")
		return
	}

	sActorId, err := strconv.Atoi(strarr[0])
	if err != nil {
		fmt.Println("命令格式错误, 第一个参数需要是数字...", strarr[0])
		return
	}

	eActorId, err := strconv.Atoi(strarr[1])
	if err != nil {
		fmt.Println("命令格式错误, 第二个参数需要是数字...", strarr[1])
		return
	}

	if eActorId <= sActorId {
		fmt.Println("命令格式错误, 第二个参数不能小于等于第一个参数....")
		return
	}

	nodes := GetAllNodeData(etcdRoot)
	if !IsStable(nodes) {
		fmt.Println("集群不稳定，不能执行，集群所有节点都要在Running状态下")
		return
	}

	// nodes 中的所有节点都处在running中
	ring := utils.NewConsistent()
	for _, v := range nodes {
		ring.Add(utils.NewNodeKey(v.Key, 1))
	}


	for id:=sActorId; id<=eActorId; id++ {
		// 计算出id预期在哪个节点
		nodeKey, err := ring.Get(fmt.Sprintf("%d", id))
		var expect string
		if err != nil {
			expect = err.Error()
		} else {
			expect = nodeKey.Key()
		}

		printstr := fmt.Sprintf("%d(expect:%s)|", id, expect)

		for _, targetNode := range nodes {

			printstr = fmt.Sprintf("%s{%s", printstr, targetNode.Key)

			ExecGrpcCall(false, targetNode, func(ctx context.Context, cli testmsg.TestConsoleClient) error {
				queryMsg := &testmsg.QueryMsg{
					ReqId:                utils.GenUuidWithUint64(),
					TargetId:             int64(id),
				}


				resp, err1 := cli.QueryMsgRequest(ctx, queryMsg)
				if err1 != nil {
					printstr = fmt.Sprintf("%s 失败(%v)", printstr, err1)
					return nil
				}

				if targetNode.Key == resp.NodeName {
					printstr = fmt.Sprintf("%s 存在", printstr)
				} else {
					printstr = fmt.Sprintf("%s 不存在", printstr)
				}

				return nil
			})

			printstr = fmt.Sprintf("%s}", printstr)
		}
		fmt.Println(printstr)
	}
}

// 多消息请求
func MultiMsgRequest(msgstr interface{}) {
	str, ok := msgstr.(string)
	if !ok {
		fmt.Println("不能正常转换成 string 消息")
		return
	}

	strarr := strings.Split(str, " ")
	if len(strarr) != 2 {
		fmt.Println("命令格式错误, 需要提供二个数字参数")
		return
	}

	sActorId, err := strconv.Atoi(strarr[0])
	if err != nil {
		fmt.Println("命令格式错误, 第一个参数需要是数字...", strarr[0])
		return
	}

	eActorId, err := strconv.Atoi(strarr[1])
	if err != nil {
		fmt.Println("命令格式错误, 第二个参数需要是数字...", strarr[1])
		return
	}

	if eActorId <= sActorId {
		fmt.Println("命令格式错误, 第二个参数不能小于等于第一个参数....")
		return
	}

	for id:=sActorId; id<=eActorId; id++ {
		go func(_id int64) {
			msg := &testmsg.RunMsg{
				TargetId: _id,
			}

			var err error

			// 错误后重试5次，每次间隔1秒，加上本身要执行的一次，所以是6次
			for i:=0; i<6; i++ {
				err = execRunMsg(msg)
				if err == nil {
					break
				} else {
					time.Sleep(time.Second)
				}
			}

			if err != nil {
				fmt.Println(err)
			}
		}(int64(id))
	}
}

func execRunMsg(msg interface{}) error {
	nodes := GetAllNodeDataWithRunning(etcdRoot)
	fmt.Println("节点数量:", len(nodes))

	logicMsg, ok := msg.(*testmsg.RunMsg)
	if !ok {
		return errors.New("不能正常转换成 testmsg.RunMsg 消息")
	}

	if logicMsg.TargetId <= 0 {
		return errors.Errorf("目标actor不能为空.....TargetId: %d", logicMsg.TargetId)
	}

	targetNode := HitNodeWithActor(nodes, logicMsg.TargetId)

	err := ExecGrpcCall(true, targetNode, func(ctx context.Context, cli testmsg.TestConsoleClient) error {
		logicMsg.ReqId = utils.GenUuidWithUint64()

		resp, err1 := cli.RunMsgRequest(ctx, logicMsg)
		if err1 == nil {
			fmt.Println("RunMsgResponse Complete...", resp)
		}
		return err1
	})

	if err != nil {
		return errors.Wrap(err, "向远程节点调用错误....")
	}
	return nil
}

func RunMsgRequest(msg interface{}) {
	err := execRunMsg(msg)
	if err != nil {
		fmt.Println(err)
	}
}


// 集群处于不稳定状态
// 控制台拿到node_key所代表的节点
// 1.确保node_key在集群中是running状态
// 2.确保集群当前一定是超过1个以上的节点，并且有一个节点是处于Pending状态下
func LocalRunPendingRequest(msg interface{}) {
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

	err := ExecGrpcCall(true, targetNode, func(ctx context.Context, cli testmsg.TestConsoleClient) error {
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

func LocalRunRequest(msg interface{}) {
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

	err := ExecGrpcCall(true, targetNode, func(ctx context.Context, cli testmsg.TestConsoleClient) error {
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

// 获取所有running状态下的节点
func GetAllNodeDataWithRunning(etcdRootPath string) (nodes []*Node) {
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

		if info.Status == services.ServiceStatusRunning {
			info.Key = filepath.Base(keyStr)
			nodes = append(nodes, info)
		}
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
func ExecGrpcCall(showcalltime bool, node *Node, fn func(ctx context.Context, cli testmsg.TestConsoleClient) error) error {
	conn, err := grpc.Dial(node.Addr, grpc.WithTimeout(2*time.Second), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5 *time.Minute)

	defer conn.Close()
	defer cancel()

	cli := testmsg.NewTestConsoleClient(conn)

	st := time.Now()
	err = fn(ctx, cli)
	et := time.Now()

	if showcalltime {
		// todo: 打印调用时间长度，发生在错误的时候....
		if err != nil {
			fmt.Println("GRPC调用时长...", node.Key, et.Sub(st))
		}
	}

	if err != nil {
		return err
	}
	return nil
}
