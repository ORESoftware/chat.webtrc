// source file path: ./src/rest/io/push.go
package vio

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	vapm "github.com/oresoftware/chat.webrtc/src/common/apm"
	vhp "github.com/oresoftware/chat.webrtc/src/common/handle-panic"
	virl_kafka "github.com/oresoftware/chat.webrtc/src/common/kafka"
	virl_mongo "github.com/oresoftware/chat.webrtc/src/common/mongo"
	runtime_validation "github.com/oresoftware/chat.webrtc/src/common/mongo/runtime-validation"
	virl_rabbitmq "github.com/oresoftware/chat.webrtc/src/common/rabbitmq"
	virl_redis "github.com/oresoftware/chat.webrtc/src/common/redis"
	vt "github.com/oresoftware/chat.webrtc/src/common/types"
	virl_const "github.com/oresoftware/chat.webrtc/src/common/v-constants"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	virl_conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/user"
	amqp "github.com/rabbitmq/amqp091-go"
	"sync"
	"time"
)

type IOForREST struct {
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

func (s *IOForREST) SendMessageOutToMultipleTopics(callTrace []string, topicIds []string, uc *user.AuthUser, m interface{}) {

	// here send message to RMQ, Redis and Kafka separately

	go func() {
		// RabbitMQ / RMQ
		var cs = vbu.GetNewStacktraceFrom(callTrace)
		defer func() {
			if r := vhp.HandlePanic("0105eef573d3"); r != nil {
				uc.Log.Error(vbl.Id("vid/45a5035cf646"), fmt.Sprintf("%v", r))
				vapm.SendTrace("e33c790e-06e0-4dd1-9989-05cce3b80289", fmt.Sprintf("%v", r))
			}
		}()

		for _, t := range topicIds {

			startTime := time.Now()

			pyld := vt.RabbitPayload{
				Rabbit:    true,
				Topic:     t,
				TopicType: "userz-id",
				Meta: struct {
					Temp1       string
					TimeCreated string `json:"TimeCreated"`
				}{
					Temp1:       "721c593a-f688-432a-aa93-4d2c74360938",
					TimeCreated: time.Now().UTC().String(),
				},
				Data: m,
			}

			if err := s.PublishMessageToRabbit(
				cs, t, uc, &pyld, 0,
			); err != nil {
				uc.Log.Warn(vbl.Id("vid/6fd899c96b7e"), err)
			} else {
				// this doesn't necessarily mean a success
				uc.Log.Trace(vbl.Id("vid/cdb8a151032e"), "successfully sent RabbitMQ message out:", pyld)
			}

			elapsedTime := time.Since(startTime).Milliseconds()
			vbl.Stdout.Warn(
				vbl.Id("vid/082fc3729e69"),
				fmt.Sprintf("Rabbit execution time: '%v'", elapsedTime),
			)
		}

	}()

	go func() {
		// Send to Redis
		var cs = vbu.GetNewStacktraceFrom(callTrace)

		defer func() {
			if r := vhp.HandlePanic("838caa72530c"); r != nil {
				uc.Log.Error(vbl.Id("vid/485b91ab9575"), fmt.Sprintf("%v", r))
				vapm.SendTrace("01b7ce59-3f0a-4d16-b908-30ca9431e770", fmt.Sprintf("%v", r))
			}
		}()

		var msg = vt.RedisPayload{
			Redis: true,
			Meta: struct {
				IsList      bool
				TimeCreated string
			}{
				false,
				time.Now().UTC().String(),
			},
			Data: m,
		}

		for _, t := range topicIds {

			startTime := time.Now()

			if err := s.PublishMessageToRedis(
				t, uc, msg, 0, cs,
			); err != nil {
				uc.Log.Warn(vbl.Id("vid/42f5f3ddc461"), err)
			} else {
				uc.Log.Debug(vbl.Id("vid/7c930acb601d"), "successfully sent Redis message out:", msg)
			}

			elapsedTime := time.Since(startTime).Milliseconds()
			vbl.Stdout.Warn(
				vbl.Id("vid/cabc82fe816e"),
				fmt.Sprintf("Redis execution time: '%v'", elapsedTime),
			)

		}

	}()

	go func() {
		// Send to Kafka
		var cs = vbu.GetNewStacktraceFrom(callTrace)

		defer func() {
			if r := recover(); r != nil {
				uc.Log.Error(vbl.Id("vid/ef42fc472401"), fmt.Sprintf("%v", r))
				vapm.SendTrace("1a026ba6-4971-46b9-b14b-ad7720d7a458", fmt.Sprintf("%v", r))
			}
		}()

		var msg = vt.KafkaPayload{
			Kafka: true,
			Meta: struct {
				IsList      bool
				TimeCreated string
			}{
				false,
				time.Now().UTC().String(),
			},
			Data: m,
		}

		for _, t := range topicIds {

			go func(t string) {

				startTime := time.Now()

				if err := s.publishMessageToKafkaTopic(cs, t, uc, msg, 0); err != nil {
					uc.Log.Warn(vbl.Id("vid/e6fcdd92f756"), err)
				}

				elapsedTime := time.Since(startTime).Milliseconds()
				vbl.Stdout.Warn(
					vbl.Id("vid/786634a34b56"),
					fmt.Sprintf("Kafka execution time: '%v'", elapsedTime),
				)

			}(t)

		}

	}()

}

func (s *IOForREST) SendMessageOut(callTrace []string, topicType string, topicId string, uc *user.AuthUser, m *interface{}) {

	go func() {
		// RabbitMQ
		var cs = vbu.GetNewStacktraceFrom(callTrace)

		defer func() {
			if r := vhp.HandlePanic("ad815886db55"); r != nil {
				uc.Log.Error(vbl.Id("vid/c58affbe6e69"), fmt.Sprintf("%v", r))
				vapm.SendTrace("d69936b8-3f90-4b7b-9b18-0f0e336b008b", fmt.Sprintf("%v", r))
			}
		}()

		pyld := vt.RabbitPayload{
			Rabbit:    true,
			Topic:     topicId,
			TopicType: topicType,
			Meta: struct {
				Temp1       string
				TimeCreated string `json:"TimeCreated"`
				TopicId     string `json:"RoutingKey"`
				TopicType   string `json:"TopicType"`
			}{
				Temp1:       "fd2a528f-7f10-4fcf-86c9-89f87f31bfdb",
				TimeCreated: time.Now().UTC().String(),
				TopicId:     topicId,
				TopicType:   topicType,
			},
			Data: m,
		}

		startTime := time.Now()

		if err := s.PublishMessageToRabbit(
			cs, topicId, uc, &pyld, 0,
		); err != nil {
			uc.Log.Warn(vbl.Id("vid/a8d896a0bd0a"), err)
		} else {
			uc.Log.Debug(vbl.Id("vid/06d28704505f"), "successfully sent RabbitMQ message out:", pyld)
		}

		elapsedTime := time.Since(startTime).Milliseconds()
		vbl.Stdout.Warn(
			vbl.Id("vid/37c2deeff61d"),
			fmt.Sprintf("Rabbit execution time: '%v'", elapsedTime),
		)

	}()

	go func() {
		// Redis
		var cs = vbu.GetNewStacktraceFrom(callTrace)

		defer func() {
			if r := recover(); r != nil {
				uc.Log.Error(vbl.Id("vid/017003209172"), fmt.Sprintf("%v", r))
				vapm.SendTrace("29919110-0157-4468-b782-25bbff535119", fmt.Sprintf("%v", r))
			}
		}()

		var msg = vt.RedisPayload{
			Redis: true,
			Meta: struct {
				IsList      bool
				TimeCreated string
			}{
				false,
				time.Now().UTC().String(),
			},
			Data: m,
		}

		startTime := time.Now()

		if err := s.PublishMessageToRedis(
			topicId, uc, msg, 0, cs,
		); err != nil {
			uc.Log.Warn(vbl.Id("vid/24d17bac74ee"), err)
		} else {
			uc.Log.Debug(vbl.Id("vid/543fce573ffd"), "successfully sent Redis message out:", msg)
		}

		elapsedTime := time.Since(startTime).Milliseconds()
		vbl.Stdout.Warn(
			vbl.Id("vid/f9b3fb6052b2"),
			fmt.Sprintf("Redis execution time: '%v'", elapsedTime),
		)

	}()

	go func() {
		// Kafka
		var cs = vbu.GetNewStacktraceFrom(callTrace)

		defer func() {
			if r := vhp.HandlePanic("9e27926feee4"); r != nil {
				uc.Log.Error(vbl.Id("vid/a468233dee95"), fmt.Sprintf("%v", r))
				vapm.SendTrace("fdfa0614-19fe-4da9-89d1-d614c49367e6", fmt.Sprintf("%v", r))
			}
		}()

		startTime := time.Now()

		if err := s.publishMessageToKafkaTopic(cs, topicId, uc, m, 0); err != nil {
			uc.Log.Warn(vbl.Id("vid/c874c5b659fa"), err)
		}

		elapsedTime := time.Since(startTime).Milliseconds()
		vbl.Stdout.Warn(
			vbl.Id("vid/d0e42c3c615a"),
			fmt.Sprintf("Kafka execution time: '%v'", elapsedTime),
		)

	}()

}

func (s *IOForREST) PublishMessageToRedis(topicId string, uc *user.AuthUser, body vt.RedisPayload, retryCount int, callStack []string) error {

	if retryCount > 3 {
		// do this retry check after deleting the above
		uc.Log.Warn(vbl.Id("vid/3b927022eb02"), "too many retries")
		return nil
	}

	if retryCount < 1 {
		// body.Data.ReplayCount++
		body.BumpReplayCount()
	}

	if body.GetReplayCount() > 4 {
		uc.Log.Warn(vbl.Id("vid/3c8e9d123056"), "too many replays.")
		if false {
			return vbu.ErrorFromArgs("754ff26b-68ac-48fa-8c4b-b98b7aa0df3b", "Too many replays.")
		}
	}

	var once = sync.Once{}

	var doRetry = func(reason string) {
		once.Do(func() {
			go func() {
				time.Sleep(time.Second * 3)
				var cs = vbu.GetNewStacktraceFrom(callStack)
				vbl.Stdout.Warn(vbl.Id("vid/2b408de31d7c"), fmt.Sprintf("doing retry for Redis because reason: '%v'", reason))
				if err := s.PublishMessageToRedis(topicId, uc, body, retryCount+1, cs); err != nil {
					uc.Log.Error(vbl.Id("vid/a89263832ca7"), err)
					vapm.SendTrace("912a984a-9ac3-432f-aabe-811444cd9ff9", err)
				}
			}()
		})
	}

	client, _ := s.RedisConnPool.GetConnection()

	var v = vbu.HandleErrFirst(json.Marshal(body))

	if v.Error != nil {
		uc.Log.Warn(vbl.Id("vid/258baff36630"), v.Error)
		return v.Error
	}

	z := client.Publish(context.Background(), topicId, v.Val)

	var pErr = z.Err()

	if pErr != nil {
		uc.Log.Warn(vbl.Id("vid/10db2dd1cf63"), pErr)
		doRetry(fmt.Sprintf("publishing error - %v", pErr))
		return pErr
	}

	uc.Log.Info(
		vbl.Id("vid/e2c7f7969bb8"),
		"published messaged to redis:", v,
	)

	return nil

}

// TODO: pyld should not be interface{}
func (s *IOForREST) publishMessageToKafkaTopic(callTrace []string, topicId string, uc *user.AuthUser, pyld interface{}, retryCount int) error {

	if retryCount > 3 {
		uc.Log.Warn(vbl.Id("vid/34f2cd8e50a2"), vbu.ErrorFromArgs(
			"29bf8c3e-d294-4442-8d1d-4ae199381fa5",
			"too many retries."),
		)
		return nil
	}

	var once = sync.Once{}

	var doRetry = func(reason string) {
		once.Do(func() {
			go func() {
				time.Sleep(time.Second * 3)
				var cs = vbu.GetNewStacktraceFrom(callTrace)
				vbl.Stdout.Warn(vbl.Id("vid/230c9c4375cd"), fmt.Sprintf("doing retry for kafka because reason: '%v'", reason))
				if err := s.publishMessageToKafkaTopic(cs, topicId, uc, pyld, retryCount+1); err != nil {
					uc.Log.Warn(vbl.Id("vid/e72f6e61e9f9"), err)
				}
			}()
		})
	}

	// TODO: implement retries here
	producer, _ := s.KafkaConnectionPool.GetConnection()

	var jsn []byte
	var err error

	switch pyld.(type) {
	case string:
		jsn = []byte(pyld.(string))
	case []byte:
		jsn = pyld.([]byte)
	default:
		jsn, err = json.Marshal(pyld)
	}

	if err != nil {
		uc.Log.Error(vbl.Id("vid/3a4b67c46589"), err)
		vapm.SendTrace("fc924d0d-9387-4239-8d34-34d8db04b927", err)
		return err
	}

	deliveryChan := make(chan kafka.Event)

	defer func() {
		close(deliveryChan)
	}()

	//
	if err := producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topicId, Partition: int32(0)},
		Value:          jsn,
	}, deliveryChan); err != nil {
		uc.Log.WarnF("Failed to produce message: %v", err)
		// do retry attempt
		doRetry("failed to publish")
		return err
	}

	x := <-deliveryChan
	m := x.(*kafka.Message)

	if m.TopicPartition.Error != nil {
		uc.Log.WarnF("1d0a0f17-0642-426b-9d27-3295ec9372c4", "Delivery failed: %v", m)
		// do retry attempt
		doRetry("some partition error")
		return vbu.ErrorFromArgs("e39b2b98-3137-4da2-9589-53f32d7d815f", m.String())
	}

	uc.Log.Debug(
		vbl.Id("vid/753b766c6c92"),
		fmt.Sprintf("Delivered message to topic %s [%d] at offset %v",
			*m.TopicPartition.Topic,
			m.TopicPartition.Partition,
			m.TopicPartition.Offset,
		),
	)

	vbl.Stdout.Debug(
		vbl.Id("vid/62fe0427e256"),
		"Successfully pushed message to kafka:",
		topicId,
		pyld,
	)

	return nil

}

func (s *IOForREST) PublishMessageToRabbit(callTrace []string, rk string, uc *user.AuthUser, body *vt.RabbitPayload, retryCount int) error {

	// separate routing than the one from WS because we dont have websocket conn to write messages to
	// we could combine the routines into shared code, sometime done the line

	if retryCount > 5 {
		uc.Log.Warn(vbl.Id("vid/9bc266b88987"), "too many retries")
		return vbu.ErrorFromArgs("d1ce1760-a367-4f00-91bf-a25e81153b78", "too many retries")
	}

	if retryCount < 1 {
		// only bump replay once here
		// body.Data.ReplayCount++
		body.BumpReplayCount()
	}

	if body.GetReplayCount() > 4 {
		uc.Log.Warn(vbl.Id("vid/8dee77ae7ff4"), "too many replays.")
		if false {
			return vbu.ErrorFromArgs("b87a9116-da71-4dba-bffa-27349786eb41", "Too many replays.")
		}
	}

	if v := runtime_validation.ValidateBeforeInsert(body); len(v) > 0 {
		err := errors.New(vbu.JoinList(v))
		uc.Log.Warn(vbl.Id("vid/c7a8ae0db43c"), err)
		return err
	}

	var once = sync.Once{}

	var doRetry = func(rk string, reason string) {
		once.Do(func() {
			go func() {
				time.Sleep(time.Second * 3)
				var cs = vbu.GetNewStacktraceFrom(callTrace)
				vbl.Stdout.Warn(vbl.Id("vid/1990d054e636"), fmt.Sprintf("doing retry for Rabbit because reason: '%v'", reason))
				if err := s.PublishMessageToRabbit(cs, rk, uc, body, retryCount+1); err != nil {
					uc.Log.Error(vbl.Id("vid/aaf49c7c05d3"), err)
					vapm.SendTrace("dd2f9b22-ba68-4a9a-b0b0-02630dbe1944", err)
				}
			}()
		})
	}

	v, err := json.Marshal(body)

	if err != nil {
		uc.Log.Error(vbl.Id("vid/2d5f662ef68a"), err)
		vapm.SendTrace("503861f8-1b36-4f11-95d4-487f2c938188", err)
		// this is bad, don't do a retry b/c it's pointless
		return err
	}

	go func() {

		conn, _ := s.RabbitMQConnectionPool.GetConnection()
		ch, err := conn.Channel()
		// TODO reuse channel?
		// ch, err := uc.GetRMQChannel(conn)

		if err != nil {
			uc.Log.Error(vbl.Id("vid/fd10c33eb160"), err)
			vapm.SendTrace("982ad92f-2f32-4f56-aec1-ea5b00ac44dd", err)
			doRetry(rk, fmt.Sprintf("error creating channel: %v", err))
			return
		}

		defer func() {
			if err := ch.Close(); err != nil {
				vbl.Stdout.Warn("5ae45a97-65aa-47f7-a940-a725d6a1d291", err)
			}
		}()

		// if err := ch.Confirm(false); err != nil {
		//   vbl.Stdout.Error(vbl.Id("vid/38665feac76f"), fmt.Sprintf("Error enabling Confirm mode: %v", err))
		// }

		toCtx, cancel := context.WithTimeout(context.Background(), time.Second*8)

		defer func() {
			cancel()
		}()

		/*

		   Mandatory publishing option for AMQP:

		   When a message is published with the mandatory flag set, it tells the RabbitMQ broker that the message must be routable,
		   meaning it must be possible to deliver the message to at least one queue.
		   If the message cannot be routed to any queue (i.e., no queue is bound to the exchange with a matching routing key),
		   the message is returned to the publisher with a Basic.Return AMQP method.
		   This is useful when the publisher needs to know if a message was not routable, so it can take appropriate action,
		   like sending the message to an alternate exchange or logging the failure.

		*/

		// TODO: need to wait using Confirm() to get real result (true error / true no error)

		if err := ch.PublishWithContext(
			toCtx,
			virl_const.RMQMainExchange, // exchange
			rk,                         // routing key
			false,                      // mandatory **
			false,                      // immediate (deprecated flag)
			amqp.Publishing{
				ContentType:  "application/json",
				Body:         v,
				DeliveryMode: amqp.Transient, // non durable
				Headers: map[string]interface{}{
					"x-vibe-type": 1, // TODO
				},
			},
		); err != nil {
			uc.Log.Error(vbl.Id("vid/88f368be73d2"), err)
			vapm.SendTrace("50851279-9882-49ee-8c86-88eebc288967", err)

		}

	}()

	return nil
}
