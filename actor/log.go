package actor

import (
	fmt "fmt"
	"runtime"
	"strings"

	"github.com/sirupsen/logrus"
)

var (
	plog = logrus.New().WithField("TAG", "[ACTOR]")
)

func SetLogLevel(level logrus.Level) {
	plog.Logger.SetLevel(level)
}

func Stack() string {
	var name, file string
	var line int
	var pc [16]uintptr

	n := runtime.Callers(4, pc[:])
	callers := pc[:n]
	frames := runtime.CallersFrames(callers)

	for {
		frame, more := frames.Next()
		file = frame.File
		line = frame.Line
		name = frame.Function
		if !strings.HasPrefix(name, "runtime.") || !more {
			break
		}
	}
	var str string
	switch {
	case name != "":
		str = fmt.Sprintf("%v:%v", name, line)
	case file != "":
		str = fmt.Sprintf("%v:%v", file, line)
	default:
		str = fmt.Sprintf("pc:%x", pc)

	}
	return str
}