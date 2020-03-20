package server

import (
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	EtcdHosts []string
)

func init() {
	EtcdHosts = stringSliceEnv("ETCD_HOST", ",")
	if len(EtcdHosts) <= 0 {
		logrus.Panic("不存在ETCD_HOST的环境变量")
	}
	logrus.Infof("ETCD_HOST: %s", strings.Join(EtcdHosts, ","))
}
