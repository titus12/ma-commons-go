package actor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"sort"
	"time"

	"github.com/titus12/ma-commons-go/setting"

	"github.com/sirupsen/logrus"
)

type ItemId []int64

func (a ItemId) Len() int           { return len(a) }
func (a ItemId) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ItemId) Less(i, j int) bool { return a[i] < a[j] }

// todo: actor的牵移可以优化，考虑用一个工作池来进行，这样不需要一个一个的牵，然后一个一个的等。
// todo: 最早这里商量的，如果发生错误，回应错误，起动不再执行????， 这里如果通过工作池展开是否有问题?

// 工作
type work struct {
	jobGroup    chan *Ref
	resultGroup chan error

	status int32    // 正常0， 关闭1

	wg sync.WaitGroup

	job         func()
	jobsNum int     //任务数量
}

// 向工作中添加任务
func (w *work) addJob(job *Ref) {
	w.jobGroup <- job
}



// 开始工作
func (w *work) start(worker int) {
	// 任务数量
	w.jobsNum = len(w.jobGroup)

	w.wg.Add(worker)
	for i := 0; i < worker; i++ {
		go w.job()
	}
}

// 关闭
func (w *work) close() {
	if atomic.CompareAndSwapInt32(&w.status, 0, 1) {
		close(w.jobGroup)
		close(w.resultGroup)
	}
}

// 构建一个工作
func makeWork(queueSize int) *work {
	w := &work{
		jobGroup:    make(chan *Ref, queueSize),
		resultGroup: make(chan error, queueSize + 1),   // 考虑到巩慌后要添加一个错误
	}


	job := func() {
		defer func() {
			w.wg.Done()
			if err := recover(); err != nil {
				logrus.Errorf("move workjob painc err %v", err)
				w.resultGroup <- fmt.Errorf("move workjob painc err %v", err)
			}
		}()
		for {
			select {
			case ref, ok := <- w.jobGroup:
				if !ok {
					// 关闭了通道, 使用, 没有调用wg.Done,在wg上面的等待就不成立
					return
				}

				ref.stop()
				logrus.Infof("move actorid: %d, 等待actor到摧毁状态....", ref.id)
				err := ref.WaitDestroyed(30 * time.Second)

				w.resultGroup <- err
			}
		}
	}

	w.job = job
	return w
}


func (w *work) wait(ctx context.Context) error {
	if w.jobsNum <= 0 {
		return nil
	}

	var err error

	for {
		select {
		case tmperr , ok := <-w.resultGroup:
			if !ok {
				err = fmt.Errorf("start resultServiceJob close")
				goto end
			}

			// 从结果里拿出一个错误
			if tmperr != nil {
				err = tmperr
				goto end
			}

			if w.jobsNum--; w.jobsNum <= 0 {
				goto end
			}
		case <-ctx.Done():
			//w.close()
			err = ctx.Err()
			goto end
		}
	}

end:
	w.close()
	w.wg.Wait()
	return  err
}

func (s *System) move(nodeKey string, nodeStatus int32) error {
	if nodeStatus == nodeStatusRunning {
		return nil
	}

	logrus.Warnf("开始移动actor.... nodeKey: %s, nodeStatus: %d", nodeKey, nodeStatus)

	//if setting.Test {
	//	if nodeStatus == nodeStatusPending {
	//		if setting.Key == "g002" {
	//			time.Sleep(5 * time.Second)
	//			return errors.New("move simulate err")
	//		}
	//	}
	//}

	ids := s.Ids()
	if setting.Test {
		sort.Sort(ItemId(ids))
	}

	var nonlocalCount int         //非本地计数

	for _, id := range ids {
		local, nk, ns, _, err := s.cluster.IsLocalWithUnstableRing(id)
		if err != nil {
			logrus.WithError(err).Errorf("计算不稳定稳错误...actor: %d, nodeKey: %s, nodeStatus: %d", id, nodeKey, nodeStatus)
			return err
		}


		if !local {
			logrus.Debugf("move actorid: %d, nk: %s, ns: %d 开始牵移", id, nk, ns)
			if setting.Test {
				// todo: 模拟延迟
				if nonlocalCount >= 13 {
					logrus.Errorf("move simulate err %d", nonlocalCount)
					return errors.New("move simulate err")
				}

				time.Sleep(1 * time.Second)
			}

			nonlocalCount ++

			ref := s.Ref(id)
			if ref == nil {
				continue
			}

			if ref.IsSystemRef() {
				return errors.New("move can't stop system actor")
			}
			ref.stop()
			logrus.Infof("move actorid: %d, 等待actor到摧毁状态....", id)

			err = ref.WaitDestroyed(30 * time.Second)
			if err != nil {
				logrus.Errorf("move actorid(%d) ref.WaitDestroyed err %v", id, err)
				return err
			}
		}
	}
	return nil
}
