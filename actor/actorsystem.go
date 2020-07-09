package actor

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/titus12/ma-commons-go/actor/pb"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const DefaultQueueSize = 1024                 //默认每个actor的队列长度
const DefaultRequestTimeout = 5 * time.Second //默认的请求超时时间

const (
	Runnable = iota
	Running
	Destroying
	Destroyed
)

var (
	// 重定向错误，网络层在投递消息，返回这个错误时，说明需要网络层重定向
	RedirectErr = errors.New("")
)

// 构建方法
type BuildFunc func(id int64) Handler

const NoSenderId = -1

var (
	NoSender = (*Pid)(nil)
)

// 一个actor的方法消息，消息本身就是方法(只适应于本地调用)
type ProcessMsgFunc func(self *Ref)

// 处理者，一个结构体实现这些接口方法，就是一个有效的actor
type Handler interface {
	OnStarted(ref *Ref) //actor创建后触发
	OnDestroy(ref *Ref) //acotr摧毁前触发
	OnOutTime(ref *Ref) //actor超时后触发，超时后立即会触发OnDestroy

	// 如果常驻actor会每隔20秒收到一次ping
	OnPing(ref *Ref, pingCount int64)

	// 所有非系统消息，非方法消息(ProcessMsgFunc)都会在这个接口中被接收到
	OnProcess(ctx Context)
}

// 一个system表示一个对actor进行管理的组织，这个组织在一个进程中可以有多个
// 为什么会有多个？ 我们通常把一组有相同行为方式的结构组织成一个system
// 比如所有玩家,比如所有公会等.....
// 一个系统中会有一个常驻
type System struct {
	name string //actor系统的名称

	// container克隆 与 sendmsg 互拆
	mu sync.RWMutex

	container sync.Map //存放actorRef的

	//系统actor(常驻), 常驻actor会每隔20秒收到一次ping消息
	systemActor *Ref

	ctx    context.Context
	cancel context.CancelFunc

	//超时时间
	otime time.Duration

	// 发起ping的间隔, > 0才有效，<=0 不存在ping
	pingInterval time.Duration

	// 构建建actor的方法
	builder BuildFunc

	// 委托的cache
	cache CacheTrust

	// 委托的集群
	cluster ClusterInfo
}

// 新建一个actor系统，如果存在相同名字的actor系统，则会抛出panic,这会使得整个进程崩溃
// 一般Actor系统都是在系统启动时构建，所以这里如果一个进程中准备包含多个Actor系统，则要
// 编程人员自行保证不会有重复.....
// 系统actor是没有超时消息的......
// 调用 WithPingInterval 会开启actor的ping消息，如果想系统actor也存在ping消息，
// 请在WithSystemHandler之前调用WithPingInterval
func NewSystem(name string, builder BuildFunc, opts ...systemOptFunc) *System {
	_, ok := systemMap.Load(name)
	if ok {
		logrus.Panicf("不能存在重复的ActorSystem....")
	}

	if rootContext == nil {
		logrus.Panicf("没有进行初始化......")
	}

	if builder == nil {
		logrus.Panicf("builder: 不能为空.....")
	}

	// 做进一步的检查，尽最大可能保证传进来的参数是正确的。
	h := builder(rand.Int63n(5000))
	if h == nil {
		logrus.Panicf("builder: 有问题，构建出来的handler为空....")
	}

	// actor系统上下文，通过进程中actor体系生成
	systemCtx, systemCancel := context.WithCancel(rootContext)

	system := &System{
		name:    name,
		ctx:     systemCtx,
		cancel:  systemCancel,
		builder: builder,
	}

	for _, opt := range opts {
		opt(system)
	}

	systemMap.Store(name, system)
	return system
}

var (
	// 根上下文
	rootContext context.Context
	rootCancel  context.CancelFunc

	// 存放所有System的容器
	systemMap sync.Map

	//grpcServer *grpc.Server
)

// 为进程初始化actor体系
func Initialize(opts ...optFunc) error {

	rootContext, rootCancel = context.WithCancel(context.Background())

	for _, opt := range opts {
		opt()
	}

	return nil
}

// 为进程关闭actor体系
func Close() {
	rootCancel()
}

// 获取系统
func GetSystem(name string) *System {
	a, ok := systemMap.Load(name)
	if !ok {
		return nil
	}
	return a.(*System)
}

type optFunc func()

// 开启远程服务
func WithAddress(grpcServer *grpc.Server) optFunc {
	return func() {
		startRemoteServer(grpcServer)
	}
}

// 系统选项
type systemOptFunc func(*System)

// 设置超时时间，如果不设置，则没有超时事件
func WithTimeout(otime time.Duration) systemOptFunc {
	return func(s *System) {
		s.otime = otime
	}
}

// 设置系统handler，如果不设置，则没有系统actor
func WithSystemHandler(handler Handler) systemOptFunc {
	return func(s *System) {
		actorCtx, actorCancel := context.WithCancel(s.ctx)
		s.systemActor = newActor(actorCtx, actorCancel, s, 0, handler)
		s.systemActor.start()
	}
}

// 设置ping的间隔时间，如果不设置，则不存在ping事件
func WithPingInterval(interval time.Duration) systemOptFunc {
	return func(s *System) {
		s.pingInterval = interval
	}
}

// 设置cache托管
func WithCache(cache CacheTrust) systemOptFunc {
	return func(s *System) {
		s.cache = cache
	}
}

// 设置集群托管
func WithCluster(cluster ClusterInfo) systemOptFunc {
	return func(s *System) {
		s.cluster = cluster
		s.cluster.SetCallBack(s.move)
	}
}

// 构建一个新的ref,如果存在，则返回原来的，如果不存在，则新建
func (system *System) NewRef(id int64) (*Ref, bool) {
	actorCtx, actorCancel := context.WithCancel(system.ctx)

	handler := system.builder(id)
	actorRef := newActor(actorCtx, actorCancel, system, id, handler)

	actual, loaded := system.container.LoadOrStore(id, actorRef)
	if loaded {
		// 存在相同的actor,停止掉声明的，然后返回原来的
		actorRef.cancel()
		return actual.(*Ref), loaded
	}

	//不存在，存储到map里了。
	actorRef.start()
	return actorRef, false
}

// 把actor容器中的所有id克隆一份出来
func (system *System) Ids() []int64 {
	system.mu.Lock()
	defer system.mu.Unlock()

	var ids []int64
	system.container.Range(func(key, value interface{}) bool {
		ids = append(ids, key.(int64))
		return true
	})
	return ids
}

// 根据id拿到actor的引用
func (system *System) Ref(id int64) *Ref {
	ref, ok := system.container.Load(id)
	if ok {
		return ref.(*Ref)
	}
	return nil
}

func (system *System) NewPid(id int64) *Pid {
	if id < 0 {
		return NoSender
	}
	return &Pid{
		systemName: system.name,
		id:         id,
		system:     system,
	}
}

func NewPid(system *System, id int64) *Pid {
	return system.NewPid(id)
}

// 1. 通过网络向actor投送消息(无响应) - 重定向交回网关 (msg = context)
// 2. 内部发送消息（无响应） - 重定向（由gs直接定向到gs) (msg != context)
// 3. 内部发送消息（有响应） - 重定向（由gs直接定向到gs) (msg != context)

// 投递消息的工作流程
func workflow(system *System, target int64, net Response, localProcess func(),
	redirect func(_info *pb.RedirectInfo, _conn *grpc.ClientConn), errHandler func(_err error)) {

	system.mu.RLock()
	defer system.mu.RUnlock()

	var (
		local  bool
		key    string
		status int32
		conn   *grpc.ClientConn
		err    error
	)

	// 为空的情况，说明不是网络层调用，而是actor内部投递
	if net == nil {
		// 老环计算目标
		local, key, status, conn, err = system.cluster.IsLocalWithStableRing(target)
		if err != nil {
			logrus.WithError(err).Errorf("workflow: 计算稳定环错误... target: %d", target)
			errHandler(err)
			return
		}
		if !local {
			// 重定向
			redirect(&pb.RedirectInfo{NodeKey: key, NodeStatus: status}, conn)
			return
		}
	}

	// 新环计算目标
	local, key, status, conn, err = system.cluster.IsLocalWithUnstableRing(target)

	if err != nil {
		logrus.WithError(err).Errorf("workflow: 计算不稳定环错误... target: %d", target)
		errHandler(err)
		return
	}

	fn := func() {
		ref := system.Ref(target)
		if ref == nil {
			// 重定向
			redirect(&pb.RedirectInfo{NodeKey: key, NodeStatus: status}, conn)
		} else {
			// todo: 这里是否要加入ref.stop()
			//ref.Stop()
			if err := ref.WaitDestroyed(10 * time.Second); err != nil {
				logrus.WithError(err).Errorf("workflow: 等待actor催费错误, target: %d", target)
				errHandler(err)
			} else {
				// 重定向
				redirect(&pb.RedirectInfo{NodeKey: key, NodeStatus: status}, conn)
			}
		}
	}

	if status != nodeStatusStoping {
		if local {
			// 本地处理
			localProcess()
			return
		}
		fn()
	} else {
		if local {
			logrus.Errorf("节点状态: Stoping, 节点: %s, 目标: %d, 是否本地: %t", key, target, local)
		}
		fn()
	}
}

// 提供给网络层调用的投递消息的方法，参数net是实现了Response的session
// 返回key,err  err:说明投递是否成功，err == RedirectErr: 说明需要网络层重定向, key则是重定向的位置
// nodeKey: 重定向的位置
// nodeStatus: 重定向所到节点的状态
func (system *System) TellWithResp(target int64, msg interface{}, net Response) (nodeKey string, nodeStatus int32, err error) {
	workflow(system, target, net, func() {
		ctx := newDefaultContext(
			system.NewPid(NoSenderId),
			system.NewPid(target),
			system,
			msg,
			net,
		)

		target := ctx.Self().id
		ref, _ := system.NewRef(target)
		err = ref.pushmsg(ctx)

	}, func(_info *pb.RedirectInfo, _conn *grpc.ClientConn) {
		err = RedirectErr
		nodeKey = _info.NodeKey
		nodeStatus = _info.NodeStatus
	}, func(_err error) {
		err = _err
	})
	return
}

// 同一个系统内发生请求操作
func (system *System) Ask(sender, target int64, msg interface{}) (resp interface{}, err error) {
	senderPid := system.NewPid(sender)
	targetPid := system.NewPid(target)

	workflow(system, target, nil, func() {
		proxy := newProxyContext(system, senderPid, targetPid, msg, true, nil, nil)
		resp, err = proxy.request()
	}, func(_info *pb.RedirectInfo, _conn *grpc.ClientConn) {
		proxy := newProxyContext(system, senderPid, targetPid, msg, true, _info, _conn)
		resp, err = proxy.request()
	}, func(_err error) {
		err = _err
	})
	return
}

// 内部使用的消息投递，消息投递实际上是投递一个Context接口的实现
func (system *System) tellWithContext(ctx Context) (err error) {
	target := ctx.Self().id
	workflow(system, target, nil, func() {
		ref, _ := system.NewRef(target)
		err = ref.pushmsg(ctx)
	}, func(_info *pb.RedirectInfo, _conn *grpc.ClientConn) {
		proxy := newProxyContext(system, ctx.Sender(), ctx.Self(), ctx.Msg(), false, _info, _conn)
		_, err = proxy.request()
	}, func(_err error) {
		err = _err
	})
	return
}

// 发送消息
func (system *System) Tell(sender, target int64, msg interface{}) error {
	switch v := msg.(type) {
	case Context:
		return system.tellWithContext(v)
	case ProcessMsgFunc:
		return errors.New("nonsupport ProcessMsgFunc type, need Ref type")
	default:
		ctx := newDefaultContext(system.NewPid(sender), system.NewPid(target), system, msg, nil)
		return system.tellWithContext(ctx)
	}
}

// 重定向的最终处理结果（如果重定向过程中发生多次转移，最后一定会有机会转移到padding状态的节点上），这里不再进行转移了
// 按最新哈希环计算，本地存在进行逻缉处理，不在返错。
func redirectFinalWorkflow(system *System, target int64, fn func(), errHandler func(_err error)) {
	system.mu.RLock()
	defer system.mu.RUnlock()

	// 在新环上计算目标
	local, _, _, _, err := system.cluster.IsLocalWithUnstableRing(target)
	if err != nil {
		logrus.WithError(err).Errorf("redirectFinalWorkflow: 计算不稳定环错误, target: %d", target)
		errHandler(err)
		return
	}

	if local {
		fn()
		return
	}
	errHandler(errors.New("unable deal with"))
}

func (system *System) redirectFinalWithAsk(sender, target int64, msg interface{}) (resp interface{}, err error) {
	redirectFinalWorkflow(system, target, func() {
		proxy := newProxyContext(system, system.NewPid(sender), system.NewPid(target), msg, true, nil, nil)
		resp, err = proxy.request()
	}, func(_err error) {
		err = _err
	})
	return
}

// 重定向的最终处理结果
func (system *System) redirectFinalWithTell(sender, target int64, msg interface{}) (err error) {
	redirectFinalWorkflow(system, target, func() {
		ctx := newDefaultContext(system.NewPid(sender), system.NewPid(target), system, msg, nil)
		ref, _ := system.NewRef(target)
		err = ref.pushmsg(ctx)
	}, func(_err error) {
		err = _err
	})
	return
}
