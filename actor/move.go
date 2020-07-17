package actor

import (
	"time"

	"github.com/sirupsen/logrus"
)

func (s *System) move(nodeKey string, nodeStatus int32) error {
	if nodeStatus == nodeStatusRunning {
		return nil
	}

	logrus.Warnf("开始移动actor.... nodeKey: %s, nodeStatus: %d", nodeKey, nodeStatus)
	ids := s.Ids()

	for _, id := range ids {
		local, nk, ns, _, err := s.cluster.IsLocalWithUnstableRing(id)
		if err != nil {
			logrus.WithError(err).Errorf("计算不稳定稳错误...actor: %d, nodeKey: %s, nodeStatus: %d", id, nodeKey, nodeStatus)
			return err
		}
		if !local {
			logrus.Debugf("move actorid: %d, nk: %s, ns: %d 开始牵移", id, nk, ns)

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
