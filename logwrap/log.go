package logwrap

import (
	"runtime"
	"time"

	"github.com/sirupsen/logrus"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.AddHook(newConsoleHook())
}

func InitLogWithEs() {
	// todo: 日志在生产环境打印到es
}

// 初始化日志，日志在生厂环境打印到日志文件
// 初始化日志，日志在生厂环境打印到日志文件
func InitLogWithFile() {
	sysType := runtime.GOOS
	if sysType == "windows" {
		// 如果是windows开发环境，基本可以确定是调试环境，不加载
		return
	}

	writer, err := rotatelogs.New(
		"./log/record.log.%Y%m%d",
		rotatelogs.WithRotationTime(time.Hour*24),
		rotatelogs.WithRotationCount(90),
		rotatelogs.WithLinkName("./log/record.log"),
	)

	if err != nil {
		logrus.WithError(err).Panic("日志初始化失败....")
	}
	logrus.AddHook(newFileHook(writer))
}
