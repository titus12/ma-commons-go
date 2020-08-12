package setting

import (
	"flag"

	"github.com/sirupsen/logrus"
)

var (
	Test        bool // 启动测试节点
	TestConsole bool //启动测试控制台

	Ip   string //填写本机ip
	Port int    //端口

	Key string //服务器key,nodename

	EtcdHosts []string

	// 服务名称
	ServiceName string

	EtcdRoot string

//var EtcdRoot = "/root/backend/gameser"
)

func init() {
	flag.BoolVar(&Test, "test", false, "启动测试节点")
	flag.BoolVar(&TestConsole, "console", false, "启动测试控制台")
	flag.StringVar(&Ip, "ip", "", "启动时提供的本机ip")
	flag.IntVar(&Port, "port", 0, "启动时提供的本机端口")
	flag.StringVar(&Key, "key", "", "启动时提供的本节点名称")
}

func Initialize() {
	flag.Parse()

	if !TestConsole {
		if len(Ip) <= 0 {
			logrus.Fatal("IP地址没有设置")
		}
		if Port <= 0 {
			logrus.Fatal("端口没有设置")
		}

		if len(Key) <= 0 {
			logrus.Fatal("节点名称没有设置")
		}

		logrus.Infof("KEY: %s", Key)
		logrus.Infof("IP: %s", Ip)
		logrus.Infof("PORT: %d", Port)
	}
}
