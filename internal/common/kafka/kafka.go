package kafka

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/qri-io/jsonschema"
	"github.com/spf13/viper"
)

// https://github.com/edenhill/librdkafka/blob/master/CONFIGURATION.md

func NewProducer(config *viper.Viper) (*kafka.Producer, error) {
	return kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":        config.GetString("kafka.bootstrap.servers"),
		"request.required.acks":    config.GetInt("kafka.request.required.acks"),
		"message.send.max.retries": config.GetInt("kafka.message.send.max.retries"),
		"retry.backoff.ms":         config.GetInt("kafka.retry.backoff.ms"),
	})
}

func NewConsumer(config *viper.Viper, topic string) (*kafka.Consumer, error) {
	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":       config.GetString("kafka.bootstrap.servers"),
		"group.id":                config.GetString("kafka.group.id"),
		"auto.offset.reset":       config.GetString("kafka.auto.offset.reset"),
		"auto.commit.interval.ms": config.GetInt("kafka.auto.commit.interval.ms"),
	})

	if err != nil {
		return nil, err
	}

	consumer.SubscribeTopics([]string{topic}, nil)
	return consumer, nil
}

func NewConsumerEventLoop(
	consumer *kafka.Consumer,
	schema *jsonschema.Schema,
	handler func(*kafka.Message),
	log *zap.SugaredLogger,
) (start, stop func()) {

	stopped := false
	wg := sync.WaitGroup{}
	wg.Add(1)

	start = func() {
		for !stopped {
			msg, err := consumer.ReadMessage(1 * time.Second)

			if err != nil {
				if err.(kafka.Error).Code() != kafka.ErrTimedOut {
					log.Error(err)
				}

				continue
			}

			errors, parserError := schema.ValidateBytes(context.Background(), msg.Value)

			if len(errors) > 0 {
				log.Warn(errors[0])
			} else if parserError != nil {
				log.Warn(parserError)
			} else {
				handler(msg)
			}
		}

		wg.Done()
	}

	stop = func() {
		stopped = true
		wg.Wait()
	}

	return
}