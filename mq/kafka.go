package mq

import (
	"fmt"
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
	entry.Data["app"] = hook.appName
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
