package stsmq

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/titus12/ma-commons-go/utils"

	gredis "github.com/go-redis/redis"
)

var (
	ErrLocalMQExist = errors.New("Local MQ already Exist")
)

const (
	defaultSenderGoNum   = 1   // 默认的发送消息使用的goroutine数量(到时候看能否根据cpu来决定)
	defaultReceiveGoNum  = 1   // 默认的接收goroutine数量(由于保障接收消息的有序性,暂定只能是1)
	defaultSendBoxNum    = 512 // 默认的发件箱缓存容量
	defaultListenTimeLen = 2   // 默认的MQ监听时长，单位:秒
)

// 初始化redis,这里是写死的
func initRedis() *gredis.Client {
	client := gredis.NewClient(&gredis.Options{
		Addr:     "10.218.108.172:6383",
		Password: "123456",
		DB:       0,
	})

	_, err := client.Ping().Result()
	if err != nil {
		panic(err) //抛出异常
	}
	return client
}

// mq的管理结构
type mq_mgr struct {
	client  *gredis.Client          //redis客户端，只维护一个
	q_items map[string]*redis_queue //所有在本地创建的的队列项
	sync.Mutex
}

type sendbox chan *InnerSerMsg //发件箱

// 表示一个redis实现的mq结构
type redis_queue struct {
	client *gredis.Client
	name   string           //队列名称
	p      MessageProcessor //消息处理器
	boxs   []sendbox        //发件箱
	death  chan struct{}    //死亡标记，标识着一个队列不可用
	die    int32            //0:正常， 1:死亡
}

var mgr = &mq_mgr{
	q_items: make(map[string]*redis_queue),
}

// 创建mq
func (mgr *mq_mgr) createMq(name string) *redis_queue {
	mgr.Lock()
	defer mgr.Unlock()
	if mgr.client == nil {
		mgr.client = initRedis()
	}

	_, ok := mgr.q_items[name]
	if ok {
		panic(ErrLocalMQExist) //本地已经存在
	}

	q := &redis_queue{
		name:   name,
		client: mgr.client,
		death:  make(chan struct{}),
	}

	mgr.q_items[name] = q

	// 循环构建多个发送箱，并提供多个go协程去执行每个发送箱
	for i := 0; i < defaultSenderGoNum; i++ {
		q.boxs = append(q.boxs, make(sendbox, defaultSendBoxNum))
		go q.sender(i)
	}

	for i := 0; i < defaultReceiveGoNum; i++ {
		go q.receive()
	}
	return q
}

func (mgr *mq_mgr) stopMq(name string) {
	mgr.Lock()
	defer mgr.Unlock()
	q, ok := mgr.q_items[name]
	if !ok { //本地不存在mq
		return
	}
	delete(mgr.q_items, name)
	num := len(mgr.q_items)
	if num == 0 {
		if mgr.client != nil {
			mgr.client.Close()
			mgr.client = nil
		}
	}
	if atomic.CompareAndSwapInt32(&q.die, 0, 1) {
		close(q.death) //停止掉队列
	}
}

// 构建一个Redis的MQ，由于这个是消息队列，为了简单的实现功能，这里不做mq是否重复的检查
// 构建者在使用时自已注意，不做检查是基于这类操作很少，而且基本都是每个服进行创建，自已
// 很容易通过命名规则规避队列重复
func newRedisMq(name string) SerMessageQueue {
	q := mgr.createMq(name)
	return q
}

func (q *redis_queue) sender(idx int) {
	box := q.boxs[idx]
	for {
		select {
		case <-q.death: //队列关闭的处理
			goto endfor
		case msg := <-box:
			q.pushmsg(msg)
		}
	}
endfor:
	func() {
		defer recover()
		close(box)
	}()
	plog.Error("SEND-MQ[%s]%d: Closed", q.name, idx)
}

// 这里单独例出来是为了能捕捉异常，消息递交失败，不能让整个go程停止掉,主要是把消息递交到redis上去，顺便在产生异常的时候
// 能捕捉异常，并消化掉，不让异常传递上去。
func (q *redis_queue) pushmsg(msg *InnerSerMsg) {
	defer utils.PrintPanicStack(fmt.Sprintf("SEND-MQ[%s]-TO[%s]: send fail", q.name, msg.revName))

	data, err := msg.Body.Bytes()
	if err != nil {
		plog.WithError(err).Errorf("SEND-MQ[%s]-TO[%s]: send fail", q.name, msg.revName)
		return
	}
	_, err = q.client.LPush(msg.revName, data).Result()
	if err != nil && err != gredis.Nil {
		plog.WithError(err).Errorf("SEND-MQ[%s]-TO[%s]: send fail", q.name, msg.revName)
	}
}

func (q *redis_queue) receive() {
	for {
		select {
		case <-q.death:
			goto endfor
		default:
			q.revcmsg()
		}
	}
endfor:
	plog.Error(fmt.Sprintf("RECEIVE-MQ[%s]: Closed", q.name))
}

// 这里单独例出来是为了能捕捉异常，收消息时如果发生遗产，不能让整个收的go程停止掉。
func (q *redis_queue) revcmsg() {
	defer utils.PrintPanicStack(fmt.Sprintf("RECV-MQ[%s]: recvmsg fail", q.name))
	res, err := q.client.BRPop(defaultListenTimeLen*time.Second, q.name).Result()
	if err == nil {
		if res[0] != q.name {
			plog.Errorf("RECV-MQ[%s]: msg revname err, q.revname=%s,msg.revname=%s", q.name, q.name, res[0])
		} else {
			data := utils.StrToBytes(res[1])
			msg, err := decodeToInnerSerMsg(data)
			if err == nil {
				if q.p != nil {
					q.p.Dispose(msg)
				} else {
					plog.Errorf("RECV-MQ[%s]: processer not exist", q.name)
				}
			} else {
				plog.WithError(err).Errorf("RECV-MQ[%s]: bytes to msg decode fail", q.name)
			}
		}
	} else {
		if err != gredis.Nil {
			plog.WithError(err).Errorf("RECV-MQ[%s]: recvmsg fail", q.name)
		}
	}
}

// 命中一个box
func (q *redis_queue) hitbox() sendbox {
	len := len(q.boxs)
	hit := rand.Intn(len)
	if hit >= 0 && hit < len {
		return q.boxs[hit]
	}
	return nil
}

// 向指定的box推送消息,并且这里独例出来也是为了能捕捉异常，在推消息时发生错误，不影响整个应用
func (q *redis_queue) pushtobox(box sendbox, msg *InnerSerMsg) {
	defer utils.PrintPanicStack(fmt.Sprintf("MQ[%s] Push Msg To MQ[%s], QueueRef fail...", q.name, msg.revName))
	if atomic.LoadInt32(&q.die) == 0 {
		box <- msg
	} else {
		plog.Errorf("MQ[%s] Push Msg To MQ[%s], QueueRef Closed...", q.name, msg.revName)
	}
}

//============ SerMessageQueue  ========

func (q *redis_queue) Name() string {
	return q.name
}

// 停止队列
func (q *redis_queue) Stop() {
	mgr.stopMq(q.name)
}

func (q *redis_queue) Push(message *InnerSerMsg) {
	box := q.hitbox()
	if box == nil {
		plog.Errorf("MQ[%s] Push Msg To MQ[%s], SendBox Not Hit", q.name, message.revName)
	} else {
		q.pushtobox(box, message)
	}
}

// 设置消息处理器
func (q *redis_queue) SetProcessor(processor MessageProcessor) {
	q.p = processor
}

// 开始一个消息序列
func (q *redis_queue) BeginSequence() *MsgSequence {
	seq := &MsgSequence{
		msgs: make([]*InnerSerMsg, 0, 10),
		q:    q,
	}
	return seq
}
