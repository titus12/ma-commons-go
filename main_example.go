package main

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"


	"github.com/titus12/ma-commons-go/actor"
	"github.com/titus12/ma-commons-go/services"
	"google.golang.org/grpc"
)

var (
	ActorSystem *actor.System
)

type Player struct {
	id int64
}

func (p *Player) OnStarted(ref *actor.Ref) {
	// todo: 玩家加载数据
}

func (p *Player) OnDestroy(ref *actor.Ref) {
	// todo: 玩家最后离开保存数据
}

func (p *Player) OnOutTime(ref *actor.Ref) {
	// todo: 玩家超时，如 5 分钟没有进行操作, 超时后必然会进入OnDestroy
}

func (p *Player) OnPing(ref *actor.Ref, pingCount int64) {
	// todo: ping，如果系统启动时设置了ping的操作，那么玩家永不超时，
	// todo: 每次ping都会进入
}

func (p *Player) OnProcess(ctx actor.Context) {
	// todo: 玩家消息的处理
}

func buildPlayer(id int64) actor.Handler {
	return &Player{
		id: id,
	}
}

func example() {
	etcdHost := []string{
		"127.0.0.1:2379",
	}

	// 当前服务器准备允许哪些服务
	serviceNames := []string{
		"gameser",
		"guildser",
	}

	// 完成集群的初始化
	services.Init("/backend", etcdHost, serviceNames, "gameser", "g001", "192.168.3.2:8888")

	// 可以花10分钟起动
	//ctx, _ := context.WithTimeout(context.Background(), 10*time.Minute)

	// 对于gameser即时服务的服务器也是服务的客户,必须在SyncStartService之前调用，如果存在。
	err := services.SyncStartClient(context.Background())
	if err != nil {
		logrus.WithError(err).Panic("启动失败")
		// 启动失败
		//os.Exit(1)
	}

	// 同步启动服务
	services.SyncStartService(context.Background(), 8888, func(server *grpc.Server, service *services.Service) error {
		//初始化actor,确定开启远程服务
		err := actor.Initialize(actor.WithAddress(server))
		if err != nil {
			return err
		}

		// 构建一个actor系统，并使用service提供的集群信息
		ActorSystem = actor.NewSystem("gameser", buildPlayer,
			actor.WithCluster(service),
			actor.WithTimeout(5*time.Minute),
			actor.WithPingInterval(10*time.Second))
		return nil
	})

	// 启动之后可以用ActorSystem进行消息投递

	time.Sleep(time.Hour)

}
