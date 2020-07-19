package main

import (
	"github.com/sirupsen/logrus"
	"github.com/titus12/ma-commons-go/setting"
	"github.com/titus12/ma-commons-go/testconsole"
	_ "github.com/titus12/ma-commons-go/testconsole/testmsg"
	"github.com/titus12/ma-commons-go/wlog"
	"runtime"
)

func main() {
	setting.Initialize()

	numCpu := runtime.NumCPU()
	runtime.GOMAXPROCS(numCpu)


	if setting.TestConsole {
		console := testconsole.NewConsole()
		console.Command("LocalRun", testconsole.LocalRunRequest)
		console.Command("LocalRunPending", testconsole.LocalRunPendingRequest)
		console.Command("RunMsg", testconsole.RunMsgRequest)
		console.Command("mreq", testconsole.MultiMsgRequest)
		console.Command("query", testconsole.QueryRequest)
		console.Run()
	} else {
		wlog.Initialize(logrus.DebugLevel, wlog.WithELK([]string{"127.0.0.1:9092"}, setting.Key, "game-log"))
		if setting.Test {
			testconsole.Example()
		} else {
			logrus.Panic("斩时不支持非测试起动...")
			//example()
		}
	}
}
