package wlog

import "github.com/sirupsen/logrus"

func ExampleInitialize() {
	// 假如要初始日志，并让日志按天打印到文件中去，同时控制台也能看到，同时只让他能在Linux和Windows下有效
	Initialize(logrus.DebugLevel, WithFile("log/record.log", Linux|Windows, Day, 100))

	// 假如要初始日志，并让日志打印到ELK中去（前题：先拥有一个ELK系统，并在kafka上建立了game-log的topic)
	var address = []string{
		"127.0.0.1:9092",
	}
	Initialize(logrus.DebugLevel, WithELK(address, "TEST_APP", "game-log"))

	// 假如要初始日志，并让日志打印到ELK中去，同时又打印到文件，且只支持Linux（前题：先拥有一个ELK系统，并在kafka上建立了game-log的topic)
	Initialize(logrus.DebugLevel,
		WithFile("log/record.log", Linux, Day, 100),
		WithELK(address, "TEST_APP", "game-log"),
	)
}
