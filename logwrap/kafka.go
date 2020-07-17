package logwrap

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"

	"github.com/Shopify/sarama"
)

type kafkaHook struct {
	appName       string
	topic         string
	asyncProducer sarama.AsyncProducer
}

func newKafkaHook(addrs []string, appName, topic string) (*kafkaHook, error) {
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

	return &kafkaHook{asyncProducer: producer, appName: appName, topic: topic}, nil
}

func (hook *kafkaHook) Fire(entry *logrus.Entry) error {
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

func (hook *kafkaHook) Levels() []logrus.Level {
	return logrus.AllLevels
}
