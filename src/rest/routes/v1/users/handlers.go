// source file path: ./src/rest/routes/v1/users/handlers.go
package v1_routes_users

import (
	"context"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	virl_mongo "github.com/oresoftware/chat.webrtc/src/common/mongo"
	runtime_validation "github.com/oresoftware/chat.webrtc/src/common/mongo/runtime-validation"
	mngo_types "github.com/oresoftware/chat.webrtc/src/common/mongo/types"
	"github.com/oresoftware/chat.webrtc/src/common/mongo/vctx"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	virl_const "github.com/oresoftware/chat.webrtc/src/common/v-constants"
	vutils "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/common/zmap"
	conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"net/http"
	"time"
)

func createUserHandler(c *ctx.VibeCtx) http.HandlerFunc {

	// var gid = os2.Getegid()
	var cfg = conf.GetConf()
	var ignoreTransErr = true

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			var rtn struct {
				payload struct {
					UserId   *primitive.ObjectID
					DeviceId *primitive.ObjectID
				}
			}

			var d = c.MustGetRequestBodyMap(req)
			var z = zmap.NewZMap(d)

			if err := z.MustGetMongoObjectId("UserId", func(id *primitive.ObjectID) error {
				rtn.payload.UserId = id
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "bd5c52c9-5e8e-4d87-9d98-b4c0e054a724",
					Err:   err,
				})
			}

			if err := z.MustGetMongoObjectId("DeviceId", func(id *primitive.ObjectID) error {
				rtn.payload.DeviceId = id
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "834eeff9-b67d-4235-bc0f-8033fb547a06",
					Err:   err,
				})
			}

			var user mngo_types.MongoChatUser
			user.Id = *rtn.payload.UserId

			if b := runtime_validation.ValidateBeforeInsert(user); len(b) > 0 {
				err := vutils.ErrorFromArgsList("1cab3d45-04b7-4362-97b0-3a990672933e", b)
				vbl.Stdout.Warn(vbl.Id("vid/52d9de7cc0f7"), err)
				return c.GenericMarshal(500, err)
			}

			var newDeviceId = rtn.payload.DeviceId

			if err := c.M.DoTrx(9, 0, 3, func(sessionContext *mongo.SessionContext) error {

				var userDevice mngo_types.MongoChatUserDevice
				userDevice.Id = primitive.NewObjectID()
				userDevice.DeviceId = *newDeviceId
				userDevice.DeviceName = "<new-device>"
				userDevice.UserId = user.Id
				userDevice.SeqNum = -1
				userDevice.Validate()

				if b := runtime_validation.ValidateBeforeInsert(userDevice); len(b) > 0 {
					err := vutils.ErrorFromArgsList("d7c231e1-d685-4907-a4d8-d26d207e2c27", b)
					vbl.Stdout.Warn(vbl.Id("vid/0a8ff554e3ca"), err)
					return err
				}

				{
					coll := c.MongoDb.Collection(virl_mongo.VibeUserDevices)
					if _, err := c.M.DoInsertOne(vctx.NewMgoSessionCtx(sessionContext), coll, userDevice); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/e65438ed555a"), err)
						return err
					}
				}

				{
					coll := c.MongoDb.Collection(virl_mongo.VibeChatUser)
					if _, err := c.M.DoInsertOne(vctx.NewMgoSessionCtx(sessionContext), coll, user); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/e62cc1c87c1c"), err)
						return err
					}
				}

				return nil

			}); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/51d4e385ff8e"), err)
				if !ignoreTransErr {
					return c.GenericMarshal(500, err)
				}
			}

			go func() {
				// create Kafka topic for user

				adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{
					"bootstrap.servers": cfg.KAFKA_BOOTSTRAP_SERVERS,
					// TODO: add auth
					"sasl.mechanism":    cfg.KAFKA_SASL_MECHANISM,    // Or "SCRAM-SHA-256", "SCRAM-SHA-512" as per your setup
					"security.protocol": cfg.KAFKA_SASL_SEC_PROTOCOL, // Or "SASL_PLAINTEXT" based on your requirement
					"sasl.username":     cfg.KAFKA_SASL_USER,
					"sasl.password":     cfg.KAFKA_SASL_PWD,
				})

				if err != nil {
					vbl.Stdout.Error(vbl.Id("vid/21a7d27c5233"), err)
					return
				}

				defer adminClient.Close()

				// Define topic specifications
				topicSpecs := kafka.TopicSpecification{
					Topic:             user.Id.Hex(),
					NumPartitions:     1, // Adjust as needed
					ReplicationFactor: 1, // Adjust as needed
				}

				ctxWTimeout, cancel := context.WithTimeout(context.Background(), 12*time.Second)

				defer func() {
					cancel()
				}()

				{
					// Create topic with a timeout duration
					_, err := adminClient.CreateTopics(ctxWTimeout, []kafka.TopicSpecification{topicSpecs}, nil)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/ea483260cdfc"), err)
					}

				}

			}()

			go func() {

				conn, _ := c.RMQConnPool.GetConnection()
				ch, err := conn.Channel()

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/ce18ce82488f"), err)
					return // handle failed connection appropriately
				}

				defer func() {
					if err := ch.Close(); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/c8bd8ab576c5"), err)
					}
				}()

				// dlxArgs := amqp.Table{
				//   "x-max-length":              99999,
				//   "x-max-length-bytes":        25000000,
				//   "x-overflow":                "drop-head",
				//   "x-expires":                 86400000 * 9, // 19 days worth of milliseconds
				//   "x-dead-letter-exchange":    virl_const.RMQDeadLetterExchange,
				//   "x-dead-letter-routing-key": user.Id.Hex(),
				//   // instead of here, you can set a TTL on individual messages instead
				//   "x-message-ttl": 86400000 * 13, //  3 days worth of milliseconds
				// }
				//
				// dlxQ, err := ch.QueueDeclare(
				//   user.Id.Hex(), // Queue name
				//   true,             // Durable
				//   false,            // Delete when unused
				//   false,            // Exclusive
				//   false,            // No-wait
				//   dlxArgs,          // Arguments
				// )
				//
				// if err != nil {
				//   vbl.Stdout.Warn(vbl.Id("vid/3d97f2e8a135"), "Error declaring queue:", err)
				// } else {
				//   if err := ch.QueueBind(
				//     dlxQ.Name,                        // Queue name
				//     dlxQ.Name,                        // Routing key - can be specific or empty for a fanout DLX
				//     virl_const.RMQDeadLetterExchange, // Exchange name
				//     false,
				//     amqp.Table{},
				//   ); err != nil {
				//     vbl.Stdout.Warn(vbl.Id("vid/cf0d29712935"), "Error declaring queue:", err)
				//   }
				// }

				// args := amqp.Table{
				// 	"x-max-length":              300,
				// 	"x-max-length-bytes":        250000,
				// 	"x-overflow":                "drop-head",
				// 	"x-expires":                 86400000 * 9, // 9 days worth of milliseconds
				// 	"x-dead-letter-exchange":    virl_const.RMQDeadLetterExchange,
				// 	"x-dead-letter-routing-key": user.Id.Hex(),
				// 	// instead of here, you can set a TTL on individual messages instead
				// 	"x-message-ttl": 86400000 * 3, //  259200000 is 3 days worth of milliseconds
				// }

				if false {
					q, err := ch.QueueDeclare(
						fmt.Sprintf("user-id:%s", user.Id.Hex()), // uuid.New().String(), // name
						false,                                    // durable
						false,                                    // delete when unused
						false,                                    // exclusive
						false,                                    // no-wait
						amqp.Table{
							"x-expires": 86400000 * 9, // 9 days
						}, // arguments
					)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/2d25569eac6d"), err)
					} else {
						if err := ch.QueueBind(
							q.Name,                     // queue name
							user.Id.Hex(),              // routing key
							virl_const.RMQMainExchange, // exchange
							false,                      // no-wait
							amqp.Table{
								"x-expires": 86400000 * 9, // 9 days
							}, // arguments
						); err != nil {
							vbl.Stdout.Error(vbl.Id("vid/f5b265d439ed"), err)
						}
					}
				}

				{
					q, err := ch.QueueDeclare(
						fmt.Sprintf("user-id:%s:device-id:%s", user.Id.Hex(), newDeviceId.Hex()), // uuid.New().String(), // name
						false, // durable
						false, // delete when unused
						false, // exclusive
						false, // no-wait
						amqp.Table{
							"x-expires": 86400000 * 9, // 9 days
						}, // arguments
					)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/d7566edbcace"), err)
					} else {
						if err := ch.QueueBind(
							q.Name,                     // queue name
							user.Id.Hex(),              // routing key
							virl_const.RMQMainExchange, // exchange
							false,                      // no-wait
							amqp.Table{},               // arguments
						); err != nil {
							vbl.Stdout.Error(vbl.Id("vid/bb91998c7af7"), err)
						}
					}
				}

			}()

			return c.GenericMarshal(200, "(done)")

		}))
}
