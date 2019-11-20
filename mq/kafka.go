package mq

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/sirupsen/logrus"
)

type KafkaHook struct {
	appName       string
	topic         string
	asyncProducer sarama.AsyncProducer
}

func NewKafkaHook(addrs []string, appName, topic string) (*KafkaHook, error) {
	config := sarama.NewConfig()
	// ignore errors
	//config.Producer.Return.Errors = false
	producer, err := sarama.NewAsyncProducer(addrs, config)
	if err != nil {
		return nil, err
	}

	go func() {
		for err := range producer.Errors() {
			fmt.Println(err)
		}
	}()

	return &KafkaHook{asyncProducer: producer, appName: appName, topic: topic}, nil
}

func (hook *KafkaHook) Fire(entry *logrus.Entry) error {
	file, line := getCallerIgnoringLogMulti(1)
	entry.Data["app"] = hook.appName
	entry.Data["file"] = file
	entry.Data["line"] = line
	message, err := entry.String()
	if err != nil {
		return err
	}

	hook.asyncProducer.Input() <- &sarama.ProducerMessage{Topic: hook.topic, Value: sarama.StringEncoder(strings.TrimSpace(message))}
	return nil
}

func (hook *KafkaHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

// getCaller returns the filename and the line info of a function
// further down in the call stack.  Passing 0 in as callDepth would
// return info on the function calling getCallerIgnoringLog, 1 the
// parent function, and so on.  Any suffixes passed to getCaller are
// path fragments like "/pkg/log/log.go", and functions in the call
// stack from that file are ignored.
func getCaller(callDepth int, suffixesToIgnore ...string) (file string, line int) {
	// bump by 1 to ignore the getCaller (this) stackframe
	callDepth++
outer:
	for {
		var ok bool
		_, file, line, ok = runtime.Caller(callDepth)
		if !ok {
			file = "???"
			line = 0
			break
		}

		for _, s := range suffixesToIgnore {
			if strings.HasSuffix(file, s) {
				callDepth++
				continue outer
			}
		}
		break
	}
	return
}

func getCallerIgnoringLogMulti(callDepth int) (string, int) {
	// the +1 is to ignore this (getCallerIgnoringLogMulti) frame
	return getCaller(callDepth+1, "logrus/hooks.go", "logrus/entry.go", "logrus/logger.go", "logrus/exported.go", "asm_amd64.s")
}
