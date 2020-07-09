package actor

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/sirupsen/logrus"

	"google.golang.org/grpc"

	"github.com/titus12/ma-commons-go/actor/pb"
	"google.golang.org/protobuf/proto"
)

// 用于响应的接口，凡实现此接口的用于接收actor的响应
type Response interface {
	Write(msg interface{})
}

type Context interface {
	Self() *Pid
	Sender() *Pid
	System() *System
	Msg() interface{}
	Response
}

type defaultContext struct {
	self   *Pid
	sender *Pid
	system *System
	msg    interface{}
	Response
}

func (ctx *defaultContext) Self() *Pid {
	return ctx.self
}

func (ctx *defaultContext) Sender() *Pid {
	return ctx.sender
}

func (ctx *defaultContext) System() *System {
	return ctx.system
}

func (ctx *defaultContext) Msg() interface{} {
	return ctx.msg
}

// 构建一个新的context，如果msg本身就是context则直接返回
func newDefaultContext(sender, target *Pid, system *System, msg interface{}, resp Response) Context {
	if ctx, ok := msg.(Context); ok {
		return ctx
	}

	if resp == nil {
		resp = new(defaultResponse)
	}

	ctx := &defaultContext{
		self:     target,
		sender:   sender,
		system:   system,
		msg:      msg,
		Response: resp,
	}
	return ctx
}

type defaultResponse struct{}

func (*defaultResponse) Write(msg interface{}) {
	logrus.Warn("暂时不支持写入", msg)
}

// 构建代理actor, 由代理actor向其他actor发出请求
func newProxyContext(system *System, sender *Pid, target *Pid, msg interface{}, isRespond bool, info *pb.RedirectInfo, conn *grpc.ClientConn) *proxyContext {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultRequestTimeout)
	proxy := &proxyContext{
		system:    system,
		sender:    sender,
		target:    target,
		reqMsg:    msg,
		ctx:       ctx,
		cancel:    cancel,
		IsRespond: isRespond,
		info:      info,
	}

	if conn != nil {
		proxy.cli = pb.NewRemoteServiceClient(conn)
	} else {
		proxy.respcha = make(chan interface{}, 1)
	}

	return proxy
}

// 如果是请求，会生成一个代理context
type proxyContext struct {
	sender *Pid
	target *Pid

	system *System

	reqMsg interface{}

	// 完成Write的锁定，确保多次调用Write不会阻塞
	status int32

	// 远程访问
	cli pb.RemoteServiceClient

	// 本地访问
	respcha chan interface{}

	ctx    context.Context
	cancel context.CancelFunc

	// 是否需要响应
	IsRespond bool

	// 重定向信息
	info *pb.RedirectInfo
}

func (proxy *proxyContext) Self() *Pid {
	return proxy.target
}

func (proxy *proxyContext) Sender() *Pid {
	return proxy.sender
}

func (proxy *proxyContext) System() *System {
	return proxy.system
}

func (proxy *proxyContext) Msg() interface{} {
	return proxy.reqMsg
}

func (proxy *proxyContext) Write(msg interface{}) {
	//防止多次写操作，使其阻塞
	if atomic.CompareAndSwapInt32(&proxy.status, 0, 1) {
		proxy.respcha <- msg
	}
}

//// 等待消息,如果超时会返回错误
func (proxy *proxyContext) waitMsg() (interface{}, error) {
	defer func() {
		close(proxy.respcha)
		proxy.cancel()
	}()
	select {
	case msg := <-proxy.respcha:
		return msg, nil
	case <-proxy.ctx.Done():
		return proxy.ctx.Err(), nil
	}
}

func (proxy *proxyContext) toProxyRequestMsg() (*pb.RequestMsg, error) {
	protoReqMsg, ok := proxy.reqMsg.(proto.Message)
	if !ok {
		return nil, errors.New("request not proto.Message")
	}

	wrap, err := pb.NewWrapMsg(protoReqMsg)
	if err != nil {
		return nil, err
	}

	proxyRequestMsg := &pb.RequestMsg{
		Target:    proxy.target.ToActorDesc(),
		IsRespond: proxy.IsRespond,
		Redirect:  proxy.info,
		Data:      wrap,
	}
	if proxy.sender != nil {
		proxyRequestMsg.Sender = proxy.sender.ToActorDesc()
	}

	return proxyRequestMsg, nil
}

// 代理发起请求
func (proxy *proxyContext) request() (interface{}, error) {
	// grpc 向网络请求
	if proxy.cli != nil {
		proxyRequestMsg, err := proxy.toProxyRequestMsg()

		if err != nil {
			return nil, err
		}

		proxyResponseMsg, err := proxy.cli.Request(proxy.ctx, proxyRequestMsg)
		if err != nil {
			return nil, err
		}

		// 不需要响应时，没有返回值
		if !proxy.IsRespond {
			return nil, nil
		}

		if proxyRequestMsg.Data == nil {
			logrus.Error("响应数据怎么可能为空....请检查")
			return nil, nil
		}

		protoRespMsg, err := proxyResponseMsg.Data.UnPack()
		return protoRespMsg, err
	} else {
		// 向本地推送消息
		system := proxy.system
		if system == nil {
			return nil, fmt.Errorf("actor system not exist...systemName=%s", proxy.target.systemName)
		}

		ref := proxy.target.Ref()
		if ref == nil {
			return nil, fmt.Errorf("actor ref not exist... target: %v", proxy.target)
		}

		err := ref.pushmsg(proxy)
		if err != nil {
			return nil, err
		}
		respMsg, err := proxy.waitMsg()
		if err != nil {
			return nil, err
		}
		if proxy.IsRespond {
			return respMsg, err
		}
		return nil, nil
	}
}
