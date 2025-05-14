// source file path: ./src/rest/routes/v1/kafka/handlers.go
package v1_routes_kafka

import (
	"context"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/common/zmap"
	conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"github.com/oresoftware/go-iterators/v1/iter"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"time"
)

func createKafkaHandler(c *ctx.VibeCtx) http.HandlerFunc {

	var cfg = conf.GetConf()

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			var Payload struct {
				TopicIds *[]primitive.ObjectID
			}

			var d = c.MustGetRequestBodyMap(req)
			var z = zmap.NewZMap(d)

			if err := z.MustGetObjectIdSlice("TopicIds", func(ids *[]primitive.ObjectID) error {
				Payload.TopicIds = ids
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "f88c59e1-2f1b-41a0-9b06-6566d0a83b4e",
					Err:   err,
				})
			}

			chanLogs := make(chan kafka.LogEvent)

			defer func() {
				close(chanLogs)
			}()

			if false {
				go func() {
					for {
						// logEv := <-chanLogs
						// vibelog.Stdout.Info(vbl.Id("kafka admin -", logEv)
					}
				}()
			}

			adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{
				"bootstrap.servers": cfg.KAFKA_BOOTSTRAP_SERVERS,
				// TODO: add auth
				"sasl.mechanism":    cfg.KAFKA_SASL_MECHANISM,    // Or "SCRAM-SHA-256", "SCRAM-SHA-512" as per your setup
				"security.protocol": cfg.KAFKA_SASL_SEC_PROTOCOL, // Or "SASL_PLAINTEXT" based on your requirement
				"sasl.username":     cfg.KAFKA_SASL_USER,
				"sasl.password":     cfg.KAFKA_SASL_PWD,
				// "sasl.jaas.config": cfg.KAFKA_SASL_JAAS_CONFIG,
				// "go.logs.channel":        chanLogs,
				// "go.logs.channel.enable": true,
			})

			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/d12248c37a42"), err)
				return c.GenericMarshal(500, err)
			}

			defer func() {
				adminClient.Close()
			}()

			// Define topic specifications
			var topicSpecs = []kafka.TopicSpecification{}
			for _, v := range *Payload.TopicIds {
				topicSpecs = append(topicSpecs, kafka.TopicSpecification{
					Topic:             v.Hex(),
					NumPartitions:     1, // Adjust as needed
					ReplicationFactor: 1, // Adjust as needed
				})
			}

			ctxWTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			defer func() {
				cancel()
			}()

			for v := range iter.SeqFromList(8, topicSpecs) {

				defer func() {
					v.MarkTaskAsComplete()
					v.StartNextTask()
				}()

				// Create topic with a timeout duration
				results, err := adminClient.CreateTopics(ctxWTimeout, []kafka.TopicSpecification{v.Value}, nil)

				if err != nil {
					vbl.Stdout.Error(vbl.Id("vid/cb77bde4eb25"), err)
					return c.GenericMarshal(500, err)
				}

				// Check for specific topic creation result
				for _, result := range results {
					if result.Error.Code() != kafka.ErrNoError {
						vbl.Stdout.WarnF("Failed to create topic %v", result)
					}
				}
			}

			return c.GenericMarshal(200, struct {
				Topics interface{}
			}{
				topicSpecs,
			})
		}))
}
