package main

import (
	"runtime"

	"github.com/titus12/ma-commons-go/logwrap"



	"github.com/sirupsen/logrus"

	"github.com/titus12/ma-commons-go/setting"
	"github.com/titus12/ma-commons-go/testconsole"

	_ "github.com/titus12/ma-commons-go/testconsole/testmsg"

)

func main() {
	logwrap.InitLogWithFile()
	numCpu := runtime.NumCPU()
	runtime.GOMAXPROCS(numCpu)
	logrus.SetLevel(logrus.DebugLevel)
	setting.Initialize()
	if setting.TestConsole {
		console := testconsole.NewConsole()
		console.Command("LocalRun", testconsole.LocalRunRequest)
		console.Run()
	} else {
		if setting.Test {
			testconsole.Example()
		} else {
			logrus.Panic("斩时不支持非测试起动...")
			//example()
		}
	}
}
