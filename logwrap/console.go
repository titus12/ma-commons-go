package logwrap

import (
	"fmt"

	"github.com/sirupsen/logrus"
)

type consoleHook struct {
}

func newConsoleHook() *consoleHook {
	return &consoleHook{}
}

func (hook *consoleHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (hook *consoleHook) Fire(entry *logrus.Entry) error {
	file, line := getCallerIgnoringLogMulti(1)
	entry.Data["line"] = fmt.Sprintf("%s:%d", file, line)
	return nil
}
