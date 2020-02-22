package main

import (
	"fmt"
	"time"

	"github.com/titus12/ma-commons-go/actor"
	"github.com/titus12/ma-commons-go/console"
)

func init() {
	//启动actor系统
	actor.StartActorSystem(1)

	//注册actor处理器, 注：每类型的处理器不是actor,actor是的处理逻缉由这个承担
	actor.RegisterCategory(TestActorHandleWithOne, 1)
	actor.RegisterCategory(TestActorHandleWithTwo, 2)
}

//测试消息
type TestMsg struct {
	Msg string
}

//第一步测试消息
type TestSetp1Msg struct {
	Msg      string
	Target   string
	Category int
}

//测试请求
type TestRequest struct {
	Msg string
}

//测试响应
type TestResponse struct {
	Msg string
}

//抛出异常
type ThrowException struct {
	Msg string
}

func main() {
	one_pid_a := CreateActor("a", 1) //创建一个1类型，名称为a的actor
	one_pid_b := CreateActor("b", 1) //创建一个1类型，名称为b的actor
	two_pid_c := CreateActor("c", 2)

	actor.SendMsg(actor.SystemPid, two_pid_c, &ThrowException{"ExceptionMsg"})

	//演示同一类别，系统与其他actor向actor发送消息
	actor.SendMsg(actor.SystemPid, one_pid_a, "hello word system")
	actor.SendMsg(one_pid_b, one_pid_a, &TestMsg{"Hello word One_pid_b"})

	//演示请求
	future := actor.RequestFuture(actor.SystemPid, one_pid_a, &TestRequest{"Request Hello"}, time.Second*5)
	result, err := future.Result()
	if err != nil {
		panic(err)
	}
	if resp, ok := result.(*TestResponse); ok {
		fmt.Printf("收到响应, Resp: %v\n", resp)
	}

	//演示系统发送消息到actor，actor再转发消息到另外的actor
	setp1msg := &TestSetp1Msg{
		Msg:      "SystemMsgFireTarget",
		Target:   two_pid_c.Name,
		Category: int(two_pid_c.Category),
	}
	actor.SendMsg(actor.SystemPid, one_pid_a, setp1msg)

	console.ReadLine()
}

func CreateActor(name string, category int) *actor.PID {
	pid, err := actor.CreateActor(name, category)
	if err != nil {
		panic(err)
	}
	return pid
}

func TestActorHandleWithOne(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case string:
		fmt.Printf("收到来自：%v 的消息 %s\n", ctx.Sender(), msg)
	case *TestMsg:
		fmt.Printf("收到来自: %v 的消息 %v\n", ctx.Sender(), msg)
	case *TestRequest:
		fmt.Printf("收到来自：%v 的请求消息 %v, 必须回应\n", ctx.Sender(), msg)
		if !ctx.Respond(&TestResponse{"Response World"}) {
			fmt.Println("响应不成功.....")
		}
	case *TestSetp1Msg:
		fmt.Printf("收到来自系统的第一步消息 %s 然后触发发送给其他Actor\n", msg.Msg)
		target := actor.GetLocalActorRef(msg.Category, msg.Target)
		if target == nil {
			fmt.Println("没有找到目录.....")
		} else {
			ctx.Send(target, "Two Hello Worl -- "+msg.Msg)
		}
	}
}

func TestActorHandleWithTwo(ctx actor.Context) {
	switch msg := ctx.Message().(type) {
	case string:
		fmt.Printf("收到来自：%v 的消息 %s\n", ctx.Sender(), msg)
	case *ThrowException:
		fmt.Println("抛出异常")
		panic(msg)
	}
}
