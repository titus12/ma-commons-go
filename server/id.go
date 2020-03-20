package server

import "github.com/sirupsen/logrus"

var (
	ID string
)

func init() {
	idInitImpl()
	logrus.Infof("服务器ID: %s", ID)
}
