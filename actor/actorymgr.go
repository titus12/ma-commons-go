package actor

// 进行actor管理的
import (
	"github.com/orcaman/concurrent-map"
	"sync/atomic"
)

// 处理登记值的结构
type ProcessRegistryValue struct {
	Category  int          //类别
	Handler   ActorHandler //某类型的处理
	LocalPIDs cmap.ConcurrentMap
}

// 处理器登记处，用于直接管理actor的一个数据对像，每个创建的actor都存放在这里
var ProcessRegistry = make(map[int]*ProcessRegistryValue)

// 添加类别处理
func addCategoryHandler(category int, handler ActorHandler) bool {
	val, ok := ProcessRegistry[category]
	if !ok {
		val = &ProcessRegistryValue{
			Category:  category,
			Handler:   handler,
			LocalPIDs: cmap.New(),
		}
		ProcessRegistry[category] = val
	}
	return !ok
}

// 获取一个类别的handler
func getCategoryHandler(category int) ActorHandler {
	if !isExistLocalCategory(category) {
		return nil
	}
	dict := ProcessRegistry[category]
	return dict.Handler
}

// 类别是否存在
func isExistLocalCategory(category int) bool {
	_, ok := ProcessRegistry[category]
	return ok
}

// 通过类别和名称检查一个actor是否存在
func isExistLocalActor(category int, name string) bool {
	dict, ok := ProcessRegistry[category]
	if !ok { //如果类别不存在，名字自然也不存在
		return false
	}
	return dict.LocalPIDs.Has(name)
}

// 通过pid检查一个actor是否存在
func isExistLocalActorWithPid(pid *PID) bool {
	if pid == nil || !pid.IsLocal() {
		return false
	}
	return isExistLocalActor(int(pid.Category), pid.Name)
}

// 加入一个Process，每一actor创建时都会有一个Process加入，关联着actor，如果加入成功则返回true
// 加入失败则返回false
func addLocalProcess(process Process, pid *PID) error {
	if pid == nil || !pid.IsLocal() {
		return ErrPidNullOrNotLocal
	}
	if !isExistLocalCategory(int(pid.Category)) { //类别不存在，加入失败
		return ErrCategoryNotExist
	}

	if isExistLocalActorWithPid(pid) { //actor存在，加入失败
		return ErrNameAlreadyExist
	}
	dict, ok := ProcessRegistry[int(pid.Category)]
	if ok {
		sbsent := dict.LocalPIDs.SetIfAbsent(pid.Name, process)
		if !sbsent {
			return ErrNameAlreadyExist
		} else {
			return nil
		}
	}
	return ErrCategoryNotExist
}

// 获取一个本地Process
func getLocalProcessWithPid(pid *PID) Process {
	if pid == nil || !pid.IsLocal() {
		return nil
	}

	dict, ok := ProcessRegistry[int(pid.Category)]
	if !ok {
		return nil
	}
	ref, exists := dict.LocalPIDs.Get(pid.Name)
	if !exists {
		return nil
	}
	return ref.(Process)
}

func getLocalProcess(category int, name string) Process {
	dict, ok := ProcessRegistry[category]
	if !ok {
		return nil
	}
	ref, exists := dict.LocalPIDs.Get(name)
	if !exists {
		return nil
	}
	return ref.(Process)
}

// 移除一个本地Process
func removeLocalProcess(pid *PID) {
	if pid == nil || !pid.IsLocal() {
		return
	}
	dict, ok := ProcessRegistry[int(pid.Category)]
	if !ok {
		return
	}
	ref, _ := dict.LocalPIDs.Pop(pid.Name)
	if l, ok := ref.(*ActorProcess); ok {
		atomic.StoreInt32(&l.dead, 1)
	}
}
