package testconsole

import (
	"context"
	"fmt"
	"github.com/titus12/ma-commons-go/setting"
	"time"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/titus12/ma-commons-go/actor"
	"github.com/titus12/ma-commons-go/services"
	"github.com/titus12/ma-commons-go/testconsole/testmsg"

	_ "github.com/titus12/ma-commons-go/logwrap"
)

var (
	ActorSystem *actor.System
)

type TestPlayer struct {
	Id int64
}

func (p *TestPlayer) OnStarted(ref *actor.Ref) {
	logrus.Infof("收到Actor开始消息[actorId=%d]", p.Id)
}

func (p *TestPlayer) OnDestroy(ref *actor.Ref) {
	// todo: 玩家最后离开保存数据(打印日志)
}

func (p *TestPlayer) OnOutTime(ref *actor.Ref) {
	// todo: 玩家超时，如 5 分钟没有进行操作, 超时后必然会进入OnDestroy(打印日志)
}

func (p *TestPlayer) OnPing(ref *actor.Ref, pingCount int64) {
	// todo: ping，如果系统启动时设置了ping的操作，那么玩家永不超时，
	// todo: 每次ping都会进入
}

func (p *TestPlayer) OnProcess(ctx actor.Context) {
	switch msg := ctx.Msg().(type) {
	case *testmsg.LocalRun:
		logrus.Infof("收到Actor处理消息[actorId=%d] testmsg.LocalRun 消息...%v", p.Id, msg)
		break
	default:
		logrus.Infof("收到不能处理的消息.....%v", msg)
	}

	// todo: 玩家消息的处理
}

func buildPlayer(id int64) actor.Handler {
	return &TestPlayer{
		Id: id,
	}
}

func Example() {
	etcdHost := []string{
		"127.0.0.1:2379",
	}

	// 当前服务器准备允许哪些服务
	serviceNames := []string{
		"gameser",
		"guildser",
	}



	nodeAddr := fmt.Sprintf("%s:%d", setting.Ip, setting.Port)
	// 完成集群的初始化
	services.Init("/backend", etcdHost, serviceNames, "gameser", setting.Key, nodeAddr)

	// 可以花10分钟起动
	//ctx, _ := context.WithTimeout(context.Background(), 10*time.Minute)

	// 对于gameser即时服务的服务器也是服务的客户,必须在SyncStartService之前调用，如果存在。
	err := services.SyncStartClient(context.Background())
	if err != nil {
		logrus.WithError(err).Panic("启动失败")
	}

	// 同步启动服务
	services.SyncStartService(context.Background(), setting.Port, func(server *grpc.Server, service *services.Service) error {
		// todo: 这里启动时间很长的话，会影响租约，让租约找不到，失效。
		//初始化actor,确定开启远程服务
		err := actor.Initialize(actor.WithAddress(server))
		if err != nil {
			return err
		}

		// 构建一个actor系统，并使用service提供的集群信息
		ActorSystem = actor.NewSystem("gameser", buildPlayer, actor.WithCluster(service))

		StartConsoleServer(server)

		return nil
	})

	// 启动之后可以用ActorSystem进行消息投递

	//ReadLine()
	time.Sleep(24 * time.Hour)
}
