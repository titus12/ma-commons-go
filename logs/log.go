package logs

import (
	log "github.com/sirupsen/logrus"
)

func LogLevel(level string) log.Level {
	switch level {
	case "Panic":
		return log.PanicLevel
	case "Fatal":
		return log.FatalLevel
	case "Error":
		return log.ErrorLevel
	case "Warn":
		return log.WarnLevel
	case "Info":
		return log.InfoLevel
	case "Debug":
		return log.DebugLevel
	}

	return log.InfoLevel
}
