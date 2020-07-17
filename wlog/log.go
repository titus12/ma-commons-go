package wlog

import (
	"fmt"
	"runtime"
	"time"

	"github.com/sirupsen/logrus"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

const (
	Linux   int = 1 << 1
	Windows int = 1 << 2
)

// 切割时间
type slicTime struct {
	dur    time.Duration
	format string
}

func LogLevel(level string) logrus.Level {
	switch level {
	case "Panic":
		return logrus.PanicLevel
	case "Fatal":
		return logrus.FatalLevel
	case "Error":
		return logrus.ErrorLevel
	case "Warn":
		return logrus.WarnLevel
	case "Info":
		return logrus.InfoLevel
	case "Debug":
		return logrus.DebugLevel
	}
	return logrus.InfoLevel
}

func newSlicTime(dur time.Duration, format string) *slicTime {
	return &slicTime{dur: dur, format: format}
}

var (
	Day    = newSlicTime(24*time.Hour, "%Y%m%d")
	Hour   = newSlicTime(time.Hour, "%Y%m%d%H")
	Minute = newSlicTime(time.Minute, "%Y%m%d%H%M")
)

type optFunc func()

func Initialize(level logrus.Level, opts ...optFunc) {
	logrus.SetLevel(level)
	logrus.AddHook(newConsoleHook())
	for _, opt := range opts {
		opt()
	}
}

// 设置日志打印到文件
// file: 文件名，全路径或相对路径，但这不仅是一个文件名，还包含路径
// goos: 进行初始化的操作系统 (Linux, Windows)
// dur: 日志会按时间切割，这个是切割周长(Day,Hour,Minute)
// rotation: 旋转数量，日志文件切割后保留的最大数量，超过后会自动删除
func WithFile(file string, goos int, dur *slicTime, rotation int) optFunc {
	return func() {
		sysType := runtime.GOOS
		switch sysType {
		case "windows":
			if goos&Windows <= 0 {
				return
			}
		case "linux":
			if goos&Linux <= 0 {
				return
			}
		}

		p := fmt.Sprintf("%s.%s", file, dur.format)

		writer, err := rotatelogs.New(
			p,
			rotatelogs.WithRotationTime(dur.dur),
			rotatelogs.WithRotationCount(uint(rotation)),
			rotatelogs.WithLinkName(file),
		)

		if err != nil {
			logrus.WithError(err).Panic("日志初始化失败....")
		}
		logrus.AddHook(newFileHook(writer))
	}
}

// 设置日志打印到ELK套件
// kafkaBrokers: kafka集群地址
// appName: 给打印设置一个名称
// topic: 打印到哪个主题
func WithELK(kafkaBrokers []string, appName, topic string) optFunc {
	return func() {
		if len(kafkaBrokers) == 0 {
			logrus.Panic("日志初始化失败....没有提供kafka地址")
		}

		if len(appName) <= 0 {
			logrus.Panic("日志初始化失败...没有提供appname参数")
		}

		if len(topic) <= 0 {
			logrus.Panic("日志初始化失败...没有提供topic参数")
		}

		hook, err := newKafkaHook(kafkaBrokers, appName, topic)
		if err != nil {
			logrus.WithError(err).Panic("日志初始化失败....初始化kafka失败....")
		}
		logrus.AddHook(hook)
	}
}
