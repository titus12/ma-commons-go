package stsmq

import (
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/sirupsen/logrus"
)

var impl string

const (
	redis  = "redis"  //redis实现
	kafka  = "kafka"  //kafka实现
	rabbit = "rabbit" //rabbit实现
	rocket = "rocket" //rocket实现
)

const env_name string = "SER_MQ" // 环境变理名称

// 消息处理器(各模块自行处理)
type MessageProcessor interface {
	Dispose(msg *InnerSerMsg) //消息分发
}

// 服务器消息队列结构
type SerMessageQueue interface {
	Name() string                            //队列名称
	Stop()                                   //停止队列
	Push(message *InnerSerMsg)               //放入消息(消息放给自已队列)
	SetProcessor(processor MessageProcessor) //设置处理器(队列里接收到的消息交给哪些处理器处理)
	BeginSequence() *MsgSequence             //开始一个消息序列
}

// 队列组，//TODO:暂时没有实现，考虑中....
type Group struct {
	gn     string                      //组名称
	queues []string                    //绑定到组的队列
	fn     func(message proto.Message) //消息在组里的分发逻缉
}

func init() {
	impl = os.Getenv(env_name)
	if impl == "" {
		impl = redis
	} else {
		switch impl {
		case redis:
		case kafka:
		case rabbit:
		case rocket:
		default:
			impl = redis
		}
	}
}

// 未实现
func NewGroup(gn string, fn func(message proto.Message)) *Group {
	//TODO:
	return nil
}

// 构建一个新的服务器消息队列
func New(name string) SerMessageQueue {
	var mq SerMessageQueue = nil
	switch impl {
	case redis:
		mq = newRedisMq(name)
	case kafka:
		mq = newKafkaMq(name)
	case rabbit:
		mq = newRabbitMq(name)
	case rocket:
		mq = newRocketMq(name)
	default:
		plog.WithFields(logrus.Fields{
			"impl": impl,
		}).Error("not impl")
	}
	return mq
}
