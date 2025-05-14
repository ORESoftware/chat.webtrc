// source file path: ./src/rest/io/read.go
package vio

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	vhp "github.com/oresoftware/chat.webrtc/src/common/handle-panic"
	virl_kafka "github.com/oresoftware/chat.webrtc/src/common/kafka"
	virl_mongo "github.com/oresoftware/chat.webrtc/src/common/mongo"
	virl_rabbitmq "github.com/oresoftware/chat.webrtc/src/common/rabbitmq"
	virl_redis "github.com/oresoftware/chat.webrtc/src/common/redis"
	vt "github.com/oresoftware/chat.webrtc/src/common/types"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	virl_conf "github.com/oresoftware/chat.webrtc/src/config"
	"strings"
	"sync"
	"time"
)

type IOForRead struct {
	Config                 *virl_conf.ConfigVars
	RabbitMQConnectionPool *virl_rabbitmq.RabbitMQConnectionPool
	// topicIdToRMQChannels   map[string]map[*amqp.Channel]*amqp.Channel
	Mtx                 *sync.Mutex
	M                   *virl_mongo.M
	RedisConnPool       *virl_redis.RedisConnectionPool
	KafkaConnectionPool *virl_kafka.KafkaProducerConnPool
	KafkaProducerConf   *kafka.ConfigMap
	KillOnce            *sync.Once
}

func (ior *IOForRead) ConsumeFromRedisChannel(topicId string, retryCount int, callTrace []string, cb func(err error, data interface{})) error {

	var maxRetries = 3

	if retryCount > maxRetries {
		// do this retry check after deleting the above
		vbl.Stdout.Warn(vbl.Id("vid/a7b18f550480"), "too many retries")
		return nil
	}

	var isRetry = true
	var isBreak = false
	var once = sync.Once{}

	var doRetry = func(d time.Duration, reason string) {
		// TODO
		once.Do(func() {

			if !isRetry {
				return
			}

			if retryCount+1 > maxRetries {
				vbl.Stdout.Warn(vbl.Id("vid/16e33d43a463"), "too many retries")
				return
			}

			vbl.Stdout.Warn(vbl.Id("vid/235be5049086"), fmt.Sprintf("doing retry for Redis sub because reason: '%v'", reason))

			go func() {
				// retry
				time.Sleep(d)

				// TODO: we may need to delete the map entry
				if err := ior.ConsumeFromRedisChannel(topicId, retryCount+1, callTrace, cb); err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/f8c0594ff806"), err)
				}
			}()
		})
	}

	vbl.Stdout.Info(
		vbl.Id("vid/fbf1690609a5"),
		"listening on redis channel:",
		topicId,
	)

	go func() {

		defer func() {
			doRetry(time.Second*3, "doing retry because read-loop ended")
		}()

		client, _ := ior.RedisConnPool.GetConnection()
		pubsub := client.Subscribe(context.Background(), topicId)

		// Wait for confirmation that subscription is created
		recv, err := pubsub.Receive(context.Background())

		if err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/6cac7b0b8550"), err, recv)
			doRetry(time.Second*3, fmt.Sprintf("doing retry because receive error: %v", err))
			return
		}

		vbl.Stdout.Debug(vbl.Id("vid/c4310f954f07"), "pubsub.Receive() call yields:", recv)

		// Go channel which receives messages, is read-only channel
		// so we do not need to close it...

		for msg := range pubsub.Channel() {

			if isBreak {
				// superfluous check, because "return" keyword (even within select{}) will break us out,
				// but just in case
				vbl.Stdout.Warn("a69dd036-cf24-46d0-a8b8-f778f15fb673", "superflous is-break hit")
				return
			}

			select {

			// Process message
			case <-time.After(1000 * time.Second):
				// Timeout - no message in 30 seconds, might indicate an issue
				vbl.Stdout.Warn(vbl.Id("vid/2797e6b0a6b7"), "redis messages not arriving")
				continue

			default:

				vbl.Stdout.Info(vbl.Id("vid/8cfac86affe2"), "Received message from Redis:", msg)

				go func() {

					defer func() {
						if r := vhp.HandlePanic("cd172d55afae"); r != nil {
							vbl.Stdout.Error(vbl.Id("vid/334ced45d70e"), fmt.Sprintf("%v", r))
						}
					}()

					var x vt.RedisPayload

					if err := json.NewDecoder(strings.NewReader(msg.Payload)).Decode(&x); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/5c3e5c0190b8"), err)
						return
					}

					cb(nil, struct {
						TopicId string
						Data    interface{}
					}{topicId, x})

				}()

			}

		}

	}()

	return nil

}
