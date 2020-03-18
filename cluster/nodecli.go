package cluster

import (
	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/discovery"
	"google.golang.org/grpc"
)

// 节点客户端，请区分发现服务中的Node， 这个是根据Node构建的一个客户端访问结构
// 其中重点是grpc的连接和客户端调用接口(cliFace)
type NodeClient struct {
	serviceName string           //服务名称
	uid         string           //节点id
	ip          string           //节点所在ip
	port        int32            //节点端口
	grpc        *grpc.ClientConn //grpc的连接
	weight      int              //权重(用记录节点负载的)
	cliFace     interface{}      //客户端调用接口
}

// 构建一个节点客户端
func newNodeClient(node *discovery.Node, conn *grpc.ClientConn, cliFace interface{}) *NodeClient {
	if conn == nil {
		logrus.Panicf("节点的grpc连接不能为空 %v", node)
	}
	if cliFace == nil {
		logrus.Panicf("节点的客户端接口不能为空 %v", node)
	}

	return &NodeClient{
		serviceName: node.ServiceName,
		uid:         node.Uid,
		ip:          node.Ip,
		port:        node.Port,
		grpc:        conn,
		weight:      0,
		cliFace:     cliFace,
	}
}

// 返回客户端调用接口, 这个调用接口实际是grpc的服务接口，不同服务的接口不同，需要
// 使用者强转 例如：
//   nodeCli *NodeClient
//   wc := nodeCli.ClientInterface().(WaiterClient)
// 其中 WaiterClient 就是grpc的某个服务的接口，需要通过protobuf进行定义并生成
// 对于强转来说，使用都肯定清楚是使用哪类服务
func (cli *NodeClient) ClientInterface() interface{} {
	return cli.cliFace
}

// 关闭连接
func (cli *NodeClient) Close() error {
	if cli.grpc == nil {
		logrus.Errorf("节点关闭时发现不存在连接(grpc=nil)...service: %s, uid: %s, ip: %s",
			cli.serviceName, cli.uid, cli.ip)
		return nil
	}
	return cli.grpc.Close()
}
