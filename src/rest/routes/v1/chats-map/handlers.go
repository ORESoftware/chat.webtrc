// source file path: ./src/rest/routes/v1/chats-map/handlers.go
package v1_routes_chat_map

import (
	"context"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	virl_mongo "github.com/oresoftware/chat.webrtc/src/common/mongo"
	mngo_types "github.com/oresoftware/chat.webrtc/src/common/mongo/types"
	"github.com/oresoftware/chat.webrtc/src/common/mongo/vctx"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	virl_const "github.com/oresoftware/chat.webrtc/src/common/v-constants"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	zmap "github.com/oresoftware/chat.webrtc/src/common/zmap"
	conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
	"time"
)

func addUserToChat(c *ctx.VibeCtx) http.HandlerFunc {

	var cfg = conf.GetConf()

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			loggedInUser := c.MustGetLoggedInUser(req)

			// var Payload struct {
			//	UserIds   []interface{}
			//	ChatTitle string
			//	CreatedBy string
			// }

			vbl.Stdout.Warn(vbl.Id("vid/8dd7b1339914"), "user-id-raw 4", loggedInUser)

			var d = c.MustGetRequestBodyMap(req)
			var z = zmap.NewZMap(d)

			var Payload = struct {
				UserId    *primitive.ObjectID
				DeviceId  *primitive.ObjectID
				ChatId    *primitive.ObjectID
				InviterId *primitive.ObjectID
				IsAdmin   bool
			}{}

			if err := z.GetMongoObjectIdFromString("ChatId", func(v *primitive.ObjectID) error {
				Payload.ChatId = v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "2d6fc255-4670-41d4-9494-f8b4fb60901e",
					Err:   err,
				})
			}

			if err := z.GetMongoObjectIdFromString("UserId", func(v *primitive.ObjectID) error {
				Payload.UserId = v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "3cc9a5d0-5a27-4dc2-8894-d62040e8a1e9",
					Err:   err,
				})
			}

			if err := z.GetMongoObjectIdFromString("InviterId", func(v *primitive.ObjectID) error {
				Payload.InviterId = v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "5f6ed00f-c43f-4c32-a406-0001b50c9e1c",
					Err:   err,
				})
			}

			if err := z.MustGetBool("IsAdmin", func(v bool) error {
				Payload.IsAdmin = v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "e1acb083-8ebc-49d6-ba62-caad744019fd",
					Err:   err,
				})
			}

			session, err := c.MongoDb.Client().StartSession()

			if err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "fdec30ba-084b-42bb-911a-6fb60511d83f",
					Err:   err,
				})
			}

			defer session.EndSession(context.Background())

			responseBody := struct {
				Results []interface{}
			}{}

			if err := c.M.DoTrx(9, 0, 3, func(sc *mongo.SessionContext) error {
				// Perform your transactions here
				// Example: collection.InsertOne(sessionContext, bson.M{"key": "value"})

				{
					coll := c.M.Col.ChatConvUsers
					var newRecord mngo_types.MongoChatMapToUser
					newRecord.Id = primitive.NewObjectID()
					newRecord.ChatId = *Payload.ChatId
					newRecord.UserId = *Payload.UserId
					newRecord.IsRemoved = false
					newRecord.DateAdded = time.Now()

					insertResult, err := coll.InsertOne(*sc, newRecord)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/986569ce6f8c"), err)
						return err
					}

					responseBody.Results = append(responseBody.Results, insertResult)
				}

				{

					coll := c.M.Col.ChatConv
					var newRecord mngo_types.MongoChatConv
					// TODO encrypt this with something
					// Like all of their public keys
					newRecord.PrivateKey = primitive.NewObjectID().Hex()
					insertResult, err := coll.InsertOne(*sc, newRecord)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/4d215322e909"), err)
						return err
					}

					responseBody.Results = append(responseBody.Results, insertResult)

				}

				return nil
			}); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/1efef9ca4ba7"), err)
				return c.GenericMarshal(500, err)
			}

			{
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
					vbl.Stdout.Error(vbl.Id("vid/d2e1eb57b9dd"), err)
					return c.GenericMarshalWithErr(500, err, responseBody)
				}

				defer adminClient.Close()

				// Define topic specifications
				topicSpecs := kafka.TopicSpecification{
					Topic:             Payload.UserId.Hex(),
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
						vbl.Stdout.Error(vbl.Id("vid/84998e3fe455"), err)
						return c.GenericMarshalWithErr(500, err, responseBody)
					}

				}

			}

			{

				conn, _ := c.RMQConnPool.GetConnection()
				ch, err := conn.Channel()

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/1e97d74b1952"), err)
					return c.GenericMarshal(500, err) // handle failed connection appropriately
				}

				defer func() {
					if err := ch.Close(); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/533addd4e82d"), err)
					}
				}()

				q, err := ch.QueueDeclare(
					fmt.Sprintf("user-id:%s:device-id:%s", Payload.UserId.Hex(), Payload.DeviceId.Hex()), // uuid.New().String(), // name
					false, // durable
					false, // delete when unused
					false, // exclusive
					false, // no-wait
					amqp.Table{
						"x-expires": 86400000 * 9, // 9 days
					}, // arguments
				)

				if err != nil {
					vbl.Stdout.Error(vbl.Id("vid/a2956429abb7"), err)
				} else {
					if err := ch.QueueBind(
						q.Name,                     // queue name
						Payload.UserId.Hex(),       // routing key
						virl_const.RMQMainExchange, // exchange
						false,                      // no-wait
						amqp.Table{},               // arguments
					); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/a89c4dc0aa0a"), err)
					}
				}

				var filter = bson.M{"UserId": Payload.UserId}
				var findOpts = options.Find().SetLimit(100).SetBatchSize(30).SetProjection(bson.D{})

				cur, err := c.M.FindManyCursor(vctx.NewStdMgoCtx(), c.M.Col.ChatUserDevices, filter, findOpts)

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/686c8251b331"), err)
					return c.GenericMarshalWithErr(500, err, responseBody) // handle failed connection appropriately
				}

				defer func() {
					if err := cur.Close(context.Background()); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/7ba0508581ca"), err)
					}
				}()

				for cur.Next(context.Background()) {
					// ////
					var result mngo_types.MongoChatUserDevice // Replace with your result type
					if err := cur.Decode(&result); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/cce6a76d62ae"), err)
						continue
					}

					if result.Id.IsZero() {
						vbl.Stdout.Error(vbl.Id("vid/1c3bcea87078"), "zero object id")
						continue
					}

					if result.DeviceId.IsZero() {
						vbl.Stdout.Error(vbl.Id("vid/3e1f291d4e00"), "zero object id")
						continue
					}

					if result.UserId.Hex() != Payload.UserId.Hex() {
						vbl.Stdout.Warn(vbl.Id("vid/016104bf0f92"), err)
						return c.GenericMarshalWithErr(500, fmt.Errorf("user id mismatch"), responseBody) // handle failed connection appropriately
					}

					q, err := ch.QueueDeclare(
						fmt.Sprintf("user-id:%s:device-id:%s", result.UserId.Hex(), result.DeviceId.Hex()), // uuid.New().String(), // name
						false, // durable
						false, // TODO                                                                // delete when unused
						false, // exclusive
						false, // TODO                                                                // no-wait
						amqp.Table{
							"x-expires": 86400000 * 9, // 9 days
						},
						// args,                                                                        // arguments
					)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/bc08a727e855"), "could not declare queue:", err)
						continue
					}

					if err := ch.QueueBind(
						q.Name,                     // queue name
						Payload.ChatId.Hex(),       // routing key
						virl_const.RMQMainExchange, // exchange
						true,                       // TODO: no-wait
						amqp.Table{},               // arguments
					); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/9425cebd9c76"), "could not bind queue:", err)
						continue
					}

					vbl.Stdout.Warn(
						vbl.Id("vid/7560e11c0d66"),
						fmt.Sprintf("successfully bound queue '%s' to chat id '%s'", q.Name, Payload.ChatId.Hex()),
					)

				}

				if err := cur.Err(); err != nil {
					vbl.Stdout.Error(vbl.Id("vid/1599bbb8dcf6"), err)
				}
			}

			vbl.Stdout.Warn(vbl.Id("vid/6f9bb6bd94ad"), "sending response", responseBody)
			return c.GenericMarshal(200, responseBody)
		}))
}

func removeUserFromChat(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			loggedInUser := c.MustGetLoggedInUser(req)

			// var Payload struct {
			//	UserIds   []interface{}
			//	ChatTitle string
			//	CreatedBy string
			// }

			vbl.Stdout.Warn(vbl.Id("vid/29d28d99e04e"), "user-id-raw 4", loggedInUser)

			var d = c.MustGetRequestBodyMap(req)
			var z = zmap.NewZMap(d)

			var Payload struct {
				ChatId    primitive.ObjectID
				UserId    primitive.ObjectID
				DeviceId  primitive.ObjectID
				CreatedBy primitive.ObjectID
			}

			Payload.CreatedBy = loggedInUser.UserId

			if err := z.GetMongoObjectIdFromString("UserId", func(v *primitive.ObjectID) error {
				Payload.UserId = *v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "bfa29fc2-245f-412f-bc17-7dab5c320435",
					Err:   err,
				})
			}

			if err := z.GetMongoObjectIdFromString("ChatId", func(v *primitive.ObjectID) error {
				Payload.ChatId = *v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "5012a76c-0a9b-437f-92e6-2d5f7190e401",
					Err:   err,
				})
			}

			if err := z.GetMongoObjectIdFromString("DeviceId", func(v *primitive.ObjectID) error {
				Payload.DeviceId = *v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "vid/210261dd7536",
					Err:   err,
				})
			}

			session, err := c.MongoDb.Client().StartSession()

			if err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "a2489de9-0f23-4b88-af44-26c3f95fc463",
					Err:   err,
				})
			}

			defer session.EndSession(context.Background())

			responseBody := struct {
				Results []interface{}
			}{}

			cancelCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			transaction := func(sc mongo.SessionContext) error {
				// Perform your transactions here
				// Example: collection.InsertOne(sessionContext, bson.M{"key": "value"})

				{
					collection := c.MongoDb.Collection(virl_mongo.VibeChatConvUsers)

					var newRecord mngo_types.MongoChatMapToUser
					newRecord.Id = Payload.ChatId
					newRecord.UserId = Payload.UserId
					newRecord.ChatId = Payload.ChatId
					newRecord.IsRemoved = true
					newRecord.DateRemoved = time.Now()
					insertResult, err := collection.InsertOne(sc, newRecord)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/a881994e700c"), err)
						return err
					}

					responseBody.Results = append(responseBody.Results, insertResult)
				}

				{

					collection := c.MongoDb.Collection(virl_mongo.VibeChatConv)

					var newRecord mngo_types.MongoChatConv
					// TODO encrypt this with something
					// Like all of their public keys
					newRecord.PrivateKey = primitive.NewObjectID().Hex()

					insertResult, err := collection.InsertOne(sc, newRecord)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/e809d02f503a"), err)
						return err
					}

					responseBody.Results = append(responseBody.Results, insertResult)

				}

				return nil
			}

			// Execute transaction
			if err := mongo.WithSession(cancelCtx, session, func(sc mongo.SessionContext) error {
				// withTransaction automagically commits the transaction, too..
				_, err := session.WithTransaction(sc, func(ctx mongo.SessionContext) (interface{}, error) {
					return true, transaction(sc) // true means success...could be anything
				})
				return err
			}); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/e30cc3d48aae"), err)
				return c.GenericMarshal(500, err)
			}

			{

				conn, _ := c.RMQConnPool.GetConnection()
				ch, err := conn.Channel()

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/1e97d74b1952"), err)
					return c.GenericMarshal(500, err) // handle failed connection appropriately
				}

				defer func() {
					if err := ch.Close(); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/0c97ad736dc3"), err)
					}
				}()

				if err := ch.QueueUnbind(
					fmt.Sprintf("user-id:%s:device-id:%s", Payload.UserId.Hex(), Payload.DeviceId.Hex()), // queue name
					Payload.ChatId.Hex(),       // routing key
					virl_const.RMQMainExchange, // exchange
					amqp.Table{},               // arguments
				); err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/1e97d74b1952"), err)
					return c.GenericMarshalWithErr(500, err, responseBody) // handle failed connection appropriately
				}

				var filter = bson.M{"UserId": Payload.UserId}
				var findOpts = options.Find().SetLimit(100).SetBatchSize(30).SetProjection(bson.D{})

				cur, err := c.M.FindManyCursor(vctx.NewStdMgoCtx(), c.M.Col.ChatUserDevices, filter, findOpts)

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/35c7321bbbb6"), err)
					return c.GenericMarshalWithErr(500, err, responseBody) // handle failed connection appropriately
				}

				defer func() {
					if err := cur.Close(context.Background()); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/8a216504acd0"), err)
					}
				}()

				for cur.Next(context.Background()) {
					// ////
					var result mngo_types.MongoChatUserDevice // Replace with your result type
					if err := cur.Decode(&result); err != nil {
						vbl.Stdout.Error(vbl.Id("vid/682305e702b8"), err)
						continue
					}

					if result.Id.IsZero() {
						vbl.Stdout.Error(vbl.Id("vid/bd52cfa58392"), "zero object id")
						continue
					}

					if result.DeviceId.IsZero() {
						vbl.Stdout.Error(vbl.Id("vid/eb13567cd06d"), "zero object id")
						continue
					}

					if result.UserId.Hex() != Payload.UserId.Hex() {
						vbl.Stdout.Warn(vbl.Id("vid/f0e85ee1eddf"), err)
						return c.GenericMarshalWithErr(500, fmt.Errorf("user id mismatch"), responseBody) // handle failed connection appropriately
					}

					if err := ch.QueueUnbind(
						fmt.Sprintf("user-id:%s:device-id:%s", Payload.UserId.Hex(), result.DeviceId.Hex()), // queue name
						Payload.ChatId.Hex(),       // routing key
						virl_const.RMQMainExchange, // exchange
						amqp.Table{},               // arguments
					); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/1e97d74b1952"), err)
						return c.GenericMarshalWithErr(500, err, responseBody) // handle failed connection appropriately
					}

				}
			}

			vbl.Stdout.Warn(vbl.Id("vid/704da9538230"), "sending response", responseBody)
			return c.GenericMarshal(200, responseBody)
		}))
}
