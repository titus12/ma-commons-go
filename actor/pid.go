package actor

import (
	"fmt"
	"strconv"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/sirupsen/logrus"
)

// 代表一个actor的标识,随时可以构建，actor系统本身不会存储这个结构构建的对像
// 但如果使用者不想重复构建PID,是可以自行进行保存的
type PID struct {
	ActorRef
	// 提供一个内部的进行消息通讯的入口，在对pid进行操作时不敢要每次都到管理map里
	// 去找这个入口
	p *Process
	//ctx           Context
}

// 对比
func (this *PID) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
	}

	//先检查二个比较结构的类型是否符合
	that1, ok := that.(*PID)
	if !ok {
		that2, ok := that.(PID)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}

	//当都属于同一类型，就开始比较具体值了
	if this.Location != that1.Location {
		return false
	}
	if this.Category != that1.Category {
		return false
	}
	if this.Name != that1.Name {
		return false
	}
	return true
}

// 获取一个执行引用
func (pid *PID) ref() Process {
	p := (*Process)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p))))
	if p != nil {
		if l, ok := (*p).(*ActorProcess); ok && atomic.LoadInt32(&l.dead) == 1 {
			atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p)), nil)
		} else {
			return *p
		}
	}
	ref := getLocalProcessWithPid(pid)
	if ref != nil {
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&pid.p)), unsafe.Pointer(&ref))
	}
	return ref
}

// 发送用户消息
func (pid *PID) sendUserMessage(message interface{}) {
	p := pid.ref()
	if p == nil {
		header, msg, sender := UnwrapEnvelope(message)
		plog.WithFields(logrus.Fields{
			"self":    pid,
			"sender":  sender,
			"header":  header,
			"message": msg,
		}).Error(fmt.Sprintf("Actor Local Not Exist RefSystem[%t]", pid.IsSystem()))
		return
	}
	p.SendUserMessage(pid, message)
}

// 发送系统消息
func (pid *PID) sendSystemMessage(message interface{}) {
	p := pid.ref()
	if p == nil {
		plog.WithFields(logrus.Fields{
			"self":    pid,
			"message": message,
		}).Error("Actor Local Not Exist(system)")
		return
	}
	p.SendSystemMessage(pid, message)
}

// 构建一个本地PID
func NewLocalPid(category int, name string) *PID {
	return &PID{
		ActorRef: ActorRef{
			Category: uint32(category),
			Name:     name,
			Location: SystemRef.Location,
		},
	}
}

// 通过一个ActorRef构建一个pid
func NewPidWithActorRef(ref *ActorRef) *PID {
	return &PID{
		ActorRef: *ref,
	}
}

// 停止指定的actor
func (pid *PID) Stop() {
	StopActor(pid)
}

//发送消息给指定的人
func (pid *PID) Send(target *PID, message interface{}) {
	SendMsg(pid, target, message)
}

//把消息给自已
func (pid *PID) Tell(message interface{}) {
	SendMsg(pid, pid, message)
}

//请求消息
func (pid *PID) RequestFuture(target *PID, message interface{}, timeout time.Duration) *Future {
	return RequestFuture(pid, target, message, timeout)
}

func (pid *PID) String() string {
	if pid == nil {
		return "nil"
	}
	return strconv.Itoa(int(pid.Location)) + "@" + strconv.Itoa(int(pid.Category)) + "/" + pid.Name
}

func (ref *ActorRef) IsSystem() bool {
	if ref.Name == "" {
		return true
	}
	return false
}

func (ref *ActorRef) IsLocal() bool {
	if ref.Location == 0 || ref.Location == SystemRef.Location {
		return true
	}
	return false
}
