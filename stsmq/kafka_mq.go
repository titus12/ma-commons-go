package stsmq

import (
	"encoding/json"
	"github.com/Shopify/sarama"
	log "github.com/sirupsen/logrus"
)

var (
	kSyncProducer sarama.SyncProducer
	kClient       sarama.Client
)

func newKafkaMq(name string) SerMessageQueue {
	return nil
}

func Init(brokers []string) {
	if len(brokers) == 0 {
		log.Fatal("kafka is brokers not config")
		return
	}

	config := sarama.NewConfig()
	//set false if async as below:
	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true
	config.Version = sarama.V0_10_2_0
	config.Consumer.Offsets.Initial = sarama.OffsetOldest
	producer, err := sarama.NewSyncProducer(brokers, config)
	if err != nil {
		log.Fatal(err)
	}
	kSyncProducer = producer
	cli, err := sarama.NewClient(brokers, config)
	if err != nil {
		log.Fatal(err)
	}

	kClient = cli
}

func Submit(topic string, value interface{}) (partition int32, offset int64, err error) {
	var bytes []byte
	if bytes, err = json.Marshal(&value); err == nil {
		return kSyncProducer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Value: sarama.ByteEncoder(bytes),
		})
	}
	return -1, -1, err
}

func SubmitWithKey(topic, key string, value interface{}) (partition int32, offset int64, err error) {
	var bytes []byte
	if bytes, err = json.Marshal(&value); err == nil {
		return kSyncProducer.SendMessage(&sarama.ProducerMessage{
			Topic: topic,
			Key:   sarama.StringEncoder(key),
			Value: sarama.ByteEncoder(bytes),
		})
	}
	return -1, -1, err
}

func NewConsumer() (sarama.Consumer, error) {
	return sarama.NewConsumerFromClient(kClient)
}

func NewGroupConsumer(groupId string) (sarama.ConsumerGroup, error) {
	return sarama.NewConsumerGroupFromClient(groupId, kClient)
}
