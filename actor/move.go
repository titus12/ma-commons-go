package actor

import (
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

func (s *System) move(nodeKey string, nodeStatus int32) error {
	if nodeStatus == nodeStatusRunning {
		return nil
	}

	logrus.Warnf("开始移动actor.... nodeKey: %s, nodeStatus: %d", nodeKey, nodeStatus)
	ids := s.Ids()
	if setting.Test {
		sort.Sort(ItemId(ids))
	}
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
				time.Sleep(1 * time.Second)
			}

			ref := s.Ref(id)
			if ref == nil {
				continue
			}
			err := ref.Stop()
			if err != nil {
				logrus.WithError(err).Errorf("actor 在牵移过程中，发现actor已经处于摧毁流程")
			} else {
				logrus.Infof("move actorid: %d, 等待actor到摧毁状态....", id)
				err = ref.WaitDestroyed(30 * time.Second)

				if err != nil {
					logrus.WithError(err).Errorf("move 错误 actorid: %d", id)
				}
			}
			if err != nil {
				return err
			}
		}
	}
	return nil
}
