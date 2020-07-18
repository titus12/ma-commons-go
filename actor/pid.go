package actor

import (
	"sync"

	"github.com/titus12/ma-commons-go/actor/pb"
)

// 这个结构标识一个本地actor，这是另一种对actor的表达方式，一个进程中可能不止存在一个actor系统
// 这个结构体就是表示一个本地进程中的actor
type Pid struct {
	systemName string //actor系统的名称
	id         int64  //actor的id
	once       sync.Once
	system     *System
}

func (p *Pid) Ref() *Ref {
	s := p.System()
	val, _ := s.NewRef(p.id)
	return val
}

func (p *Pid) System() *System {
	p.once.Do(func() {
		if p.system == nil {
			p.system = GetSystem(p.systemName)
		}
	})
	return p.system
}

func (p *Pid) ToActorDesc() *pb.ActorDesc {
	return &pb.ActorDesc{
		Id:     p.id,
		System: p.systemName,
	}
}
