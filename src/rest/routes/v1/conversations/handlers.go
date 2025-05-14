// source file path: ./src/rest/routes/v1/conversations/handlers.go
package v1_routes_conversations

import (
	"context"
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	virl_mongo "github.com/oresoftware/chat.webrtc/src/common/mongo"
	mngo_types "github.com/oresoftware/chat.webrtc/src/common/mongo/types"
	"github.com/oresoftware/chat.webrtc/src/common/mongo/vctx"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	virl_const "github.com/oresoftware/chat.webrtc/src/common/v-constants"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	zmap "github.com/oresoftware/chat.webrtc/src/common/zmap"
	virl_conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"github.com/oresoftware/go-iterators/v1/iter"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"net/http"
	"time"
)

func getChatConversationsUsingQueryFilter(c *ctx.VibeCtx) http.HandlerFunc {

	var _ = virl_conf.GetConf()
	var _ = context.Background()

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			loggedInUser := c.MustGetLoggedInUser(req)
			vbl.Stdout.Warn(vbl.Id("vid/af4aa5517fcc"), "user-id-raw 4", loggedInUser)

			var rtn struct {
				filterQuery struct {
					StartTime     time.Time
					EndTime       time.Time
					ChatId        primitive.ObjectID
					UserIdStrings *[]string
					UserIds       *[]primitive.ObjectID
					ChatTitle     string
					CreatedBy     primitive.ObjectID
				}
			}

			var d = c.MustGetFilterQueryParamMap(req)
			var z = zmap.NewZMap(d)
			var filter = bson.M{}

			rtn.filterQuery.CreatedBy = loggedInUser.UserId

			if err := z.GetTime("StartTime", func(v time.Time) error {
				if v.After(time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)) {
					filter["CreatedAt"] = bson.M{"$gte": v}
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.GetTime("EndTime", func(v time.Time) error {
				if v.After(time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)) {
					filter["CreatedAt"] = bson.M{"$lte": v}
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.GetMongoObjectIdFromString("ChatId", func(v *primitive.ObjectID) error {
				if len(v) > 0 {
					rtn.filterQuery.ChatId = *v
					filter["ChatId"] = *v
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.MustGetBothStringAndObjectIdSlice("UserIds", func(idsAsString *[]string, ids *[]primitive.ObjectID) error {
				rtn.filterQuery.UserIdStrings = idsAsString
				rtn.filterQuery.UserIds = ids
				if len(*ids) > 0 {
					filter["UserId"] = bson.M{"$in": ids}
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "1c38ed10-c838-4944-8d7e-4a7cea276ad8",
					Err:   err,
				})
			}

			if err := z.GetString("ChatTitle", func(v string) error {
				if len(v) > 0 {
					rtn.filterQuery.ChatTitle = v
					filter["ChatTitle"] = v
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "9e60a462-d323-4ed2-83f1-cece6780ec8d",
					Err:   err,
				})
			}

			responseBody := struct {
				Results []interface{}
			}{}

			var findOpts = options.Find().SetLimit(100).SetBatchSize(30).SetProjection(bson.D{})

			cur, err := c.M.FindManyCursor(vctx.NewStdMgoCtx(), c.M.Col.ChatConv, filter, findOpts)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/e29a2488130b"), err)
				return c.GenericMarshalWithErr(500, err, responseBody)
			}

			defer func() {
				if err := cur.Close(context.Background()); err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/48e86559e28d"), err)
				}
			}()

			for cur.Next(context.Background()) {
				// ////
				var result mngo_types.MongoChatUserDevice // Replace with your result type
				if err := cur.Decode(&result); err != nil {
					vbl.Stdout.Error(vbl.Id("vid/77efdfbd8d42"), err)
					continue
				}

				if result.Id.IsZero() {
					vbl.Stdout.Error(vbl.Id("vid/3b531520861e"), "zero object id")
					continue
				}

				if result.DeviceId.IsZero() {
					vbl.Stdout.Error(vbl.Id("vid/09298374c320"), "zero object id")
					continue
				}

			}

			if err := cur.Err(); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/d4467dde7aff"), err)
			}

			vbl.Stdout.Info(vbl.Id("vid/1e2063a72347"), "sending response", responseBody)
			return c.GenericMarshal(200, responseBody)
		}))
}

func getChatConversations(c *ctx.VibeCtx) http.HandlerFunc {

	var _ = virl_conf.GetConf()
	var _ = context.Background()

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			loggedInUser := c.MustGetLoggedInUser(req)
			vbl.Stdout.Warn(vbl.Id("vid/af4aa5517fcc"), "user-id-raw 4", loggedInUser)

			var rtn struct {
				payload struct {
					StartTime     time.Time
					EndTime       time.Time
					ChatId        primitive.ObjectID
					UserIdStrings *[]string
					UserIds       *[]primitive.ObjectID
					ChatTitle     string
					CreatedBy     primitive.ObjectID
				}
			}

			var d = c.MustGetRequestBodyMap(req)
			var z = zmap.NewZMap(d)
			var filter = bson.M{}

			rtn.payload.CreatedBy = loggedInUser.UserId

			if err := z.GetTime("StartTime", func(v time.Time) error {
				if v.After(time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)) {
					filter["CreatedAt"] = bson.M{"$gte": v}
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.GetTime("EndTime", func(v time.Time) error {
				if v.After(time.Date(2010, time.January, 1, 0, 0, 0, 0, time.UTC)) {
					filter["CreatedAt"] = bson.M{"$lte": v}
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.GetMongoObjectIdFromString("ChatId", func(v *primitive.ObjectID) error {
				rtn.payload.ChatId = *v
				filter["ChatId"] = *v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.MustGetBothStringAndObjectIdSlice("UserIds", func(idsAsString *[]string, ids *[]primitive.ObjectID) error {
				rtn.payload.UserIdStrings = idsAsString
				rtn.payload.UserIds = ids
				if len(*ids) > 0 {
					filter["UserId"] = bson.M{"$in": ids}
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "1c38ed10-c838-4944-8d7e-4a7cea276ad8",
					Err:   err,
				})
			}

			if err := z.GetString("ChatTitle", func(v string) error {
				rtn.payload.ChatTitle = v
				if len(v) > 0 {
					filter["ChatTitle"] = v
				}
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "9e60a462-d323-4ed2-83f1-cece6780ec8d",
					Err:   err,
				})
			}

			responseBody := struct {
				Results []interface{}
			}{}

			var findOpts = options.Find().SetLimit(100).SetBatchSize(30).SetProjection(bson.D{})

			cur, err := c.M.FindManyCursor(vctx.NewStdMgoCtx(), c.M.Col.ChatConv, filter, findOpts)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/e29a2488130b"), err)
				return c.GenericMarshalWithErr(500, err, responseBody)
			}

			defer func() {
				if err := cur.Close(context.Background()); err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/48e86559e28d"), err)
				}
			}()

			for cur.Next(context.Background()) {
				// ////
				var result mngo_types.MongoChatUserDevice // Replace with your result type
				if err := cur.Decode(&result); err != nil {
					vbl.Stdout.Error(vbl.Id("vid/77efdfbd8d42"), err)
					continue
				}

				if result.Id.IsZero() {
					vbl.Stdout.Error(vbl.Id("vid/3b531520861e"), "zero object id")
					continue
				}

				if result.DeviceId.IsZero() {
					vbl.Stdout.Error(vbl.Id("vid/09298374c320"), "zero object id")
					continue
				}

			}

			if err := cur.Err(); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/d4467dde7aff"), err)
			}

			vbl.Stdout.Info(vbl.Id("vid/1e2063a72347"), "sending response", responseBody)
			return c.GenericMarshal(200, responseBody)
		}))
}

func createNewChatConversation(c *ctx.VibeCtx) http.HandlerFunc {

	var cfg = virl_conf.GetConf()
	var bgCtx = context.Background()

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			var cs = vbu.GetFilteredStacktrace()
			loggedInUser := c.MustGetLoggedInUser(req)
			vbl.Stdout.Warn(vbl.Id("vid/af4aa5517fcc"), "user-id-raw 4", loggedInUser)

			var rtn struct {
				payload struct {
					ChatId        primitive.ObjectID
					UserIdStrings *[]string
					UserIds       *[]primitive.ObjectID
					ChatTitle     string
					CreatedBy     primitive.ObjectID
				}
			}

			var d = c.MustGetRequestBodyMap(req)
			var z = zmap.NewZMap(d)

			rtn.payload.CreatedBy = loggedInUser.UserId

			if err := z.GetMongoObjectIdFromString("ChatId", func(v *primitive.ObjectID) error {
				rtn.payload.ChatId = *v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "ec1c7df0-d159-40f5-9cc2-05fee1f507f2",
					Err:   err,
				})
			}

			if err := z.MustGetBothStringAndObjectIdSlice("UserIds", func(idsAsString *[]string, ids *[]primitive.ObjectID) error {
				rtn.payload.UserIdStrings = idsAsString
				rtn.payload.UserIds = ids
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "1c38ed10-c838-4944-8d7e-4a7cea276ad8",
					Err:   err,
				})
			}

			if err := z.GetString("ChatTitle", func(v string) error {
				rtn.payload.ChatTitle = v
				return nil
			}); err != nil {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "9e60a462-d323-4ed2-83f1-cece6780ec8d",
					Err:   err,
				})
			}

			responseBody := struct {
				Results []interface{}
			}{}

			// inside DoTrx, we do one retry if it's a transient error
			if err := c.M.DoTrx(9.000, 0, 3, func(sessionCtx *mongo.SessionContext) error {

				seqNum, err := c.M.IncrementSequence(*sessionCtx, "vibe_chat_conv_seq")

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/8ea18e9ba92b"), err, seqNum)
					return err
				}

				{
					// some stuff
					collection := c.MongoDb.Collection(virl_mongo.VibeChatConv)

					var newRecord mngo_types.MongoChatConv
					newRecord.Id = rtn.payload.ChatId
					newRecord.CreatedBy = rtn.payload.CreatedBy
					newRecord.ChatTitle = rtn.payload.ChatTitle
					newRecord.SeqNum = seqNum
					insertResult, err := c.M.DoInsertOne(vctx.NewMgoSessionCtx(sessionCtx), collection, newRecord)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/1bee990c5b87"), err)
						return err
					}

					responseBody.Results = append(responseBody.Results, insertResult)
				}

				{
					// more stuff
					collection := c.MongoDb.Collection(virl_mongo.VibeChatConvUsers)

					for _, userId := range *rtn.payload.UserIds {

						var newRecord mngo_types.MongoChatMapToUser
						newRecord.Id = primitive.NewObjectID()
						newRecord.ChatId = rtn.payload.ChatId
						newRecord.UserId = userId

						insertResult, err := c.M.DoInsertOne(vctx.NewMgoSessionCtx(sessionCtx), collection, newRecord)

						if err != nil {
							vbl.Stdout.Error(vbl.Id("vid/50b639109adf"), err)
							return err
						}

						responseBody.Results = append(responseBody.Results, insertResult)
					}
				}

				return nil
			}); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/4464be5817e6"), err)
				return c.GenericMarshal(500, err)
			}

			// vibelog.Stdout.Info(vbl.Id("vid/718e9ebac28f"), "sending response", responseBody)
			// return c.GenericMarshal(200, responseBody)

			adminClient, err := kafka.NewAdminClient(&kafka.ConfigMap{
				"bootstrap.servers": cfg.KAFKA_BOOTSTRAP_SERVERS,
				// TODO: add auth
				"sasl.mechanism":    cfg.KAFKA_SASL_MECHANISM,    // Or "SCRAM-SHA-256", "SCRAM-SHA-512" as per your setup
				"security.protocol": cfg.KAFKA_SASL_SEC_PROTOCOL, // Or "SASL_PLAINTEXT" based on your requirement
				"sasl.username":     cfg.KAFKA_SASL_USER,
				"sasl.password":     cfg.KAFKA_SASL_PWD,
			})

			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/284d354f2b20"), err)
				return c.GenericMarshal(500, err)
			}

			defer adminClient.Close()

			// Define topic specifications
			topic := rtn.payload.ChatId.Hex()
			topicSpecification := kafka.TopicSpecification{
				Topic:             topic,
				NumPartitions:     1, // Adjust as needed
				ReplicationFactor: 1, // Adjust as needed
			}

			ctxWTimeout, cancel := context.WithTimeout(context.Background(), 9*time.Second)

			defer func() {
				cancel()
			}()

			// Create topic with a timeout duration
			results, err := adminClient.CreateTopics(ctxWTimeout, []kafka.TopicSpecification{topicSpecification}, nil)

			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/a26d9296c0a2"), err)
				return c.GenericMarshal(500, err)
			}

			responseBody.Results = append(responseBody.Results, results)

			// Check for specific topic creation result
			for _, result := range results {
				if result.Topic == topic {
					if result.Error.Code() != kafka.ErrNoError {
						vbl.Stdout.WarnF("Failed to create topic %s: %s", topic, result.Error, "733895d5-c912-43aa-9339-0b48080f5697")
					}
				}
			}

			go func() {

				conn, _ := c.RMQConnPool.GetConnection()

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/cb561871a82f"), err)
					return
				}

				ch, err := conn.Channel()

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/b5386e171e23"), err)
					return // handle failed connection appropriately
				}

				defer func() {
					if err := ch.Close(); err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/322c387a7a36"), err)
					}
				}()

				var chatId = rtn.payload.ChatId

				// dlxArgs := amqp.Table{
				//   "x-max-length":              99999,
				//   "x-max-length-bytes":        25000000,
				//   "x-overflow":                "drop-head",
				//   "x-expires":                 86400000 * 9, // 19 days worth of milliseconds
				//   "x-dead-letter-exchange":    virl_const.RMQDeadLetterExchange,
				//   "x-dead-letter-routing-key": chatId.Hex(),
				//   // instead of here, you can set a TTL on individual messages instead
				//   "x-message-ttl": 86400000 * 13, //  3 days worth of milliseconds
				// }
				//
				// dlxQ, err := ch.QueueDeclare(
				//   chatId.Hex(), // Queue name
				//   true,             // Durable
				//   false,            // Delete when unused
				//   false,            // Exclusive
				//   false,            // No-wait
				//   dlxArgs,          // Arguments
				// )
				//
				// if err != nil {
				//   vbl.Stdout.Warn(vbl.Id("vid/ab6429a3beb4"), "Error declaring queue:", err)
				// } else {
				//   if err := ch.QueueBind(
				//     dlxQ.Name,                        // Queue name
				//     dlxQ.Name,                        // Routing key - can be specific or empty for a fanout DLX
				//     virl_const.RMQDeadLetterExchange, // Exchange name
				//     false,
				//     amqp.Table{},
				//   ); err != nil {
				//     vbl.Stdout.Warn(vbl.Id("vid/d73908867386"), "Error declaring queue:", err)
				//   }
				// }

				// args := amqp.Table{
				//   "x-max-length":              300,
				//   "x-max-length-bytes":        250000,
				//   "x-overflow":                "drop-head",
				//   "x-expires":                 86400000 * 9, // 19 days worth of milliseconds
				//   "x-dead-letter-exchange":    virl_const.RMQDeadLetterExchange,
				//   "x-dead-letter-routing-key": "dlx-queue-name",
				//   // instead of here, you can set a TTL on individual messages instead
				//   "x-message-ttl": 86400000 * 3, //  3 days worth of milliseconds
				// }

				if false {
					q, err := ch.QueueDeclare(
						fmt.Sprintf("conv-id:%s", chatId.Hex()), // uuid.New().String(), // name
						false,                                   // durable
						false,                                   // delete when unused
						false,                                   // exclusive
						false,                                   // no-wait
						amqp.Table{
							"x-expires": 86400000 * 9, // 9 days
						},
						// args,                                    // arguments
					)

					if err != nil {
						vbl.Stdout.Error(vbl.Id("vid/3805421e97c1"), err)
					} else {
						if err := ch.QueueBind(
							q.Name,                     // queue name
							chatId.Hex(),               // routing key
							virl_const.RMQMainExchange, // exchange
							true,                       // no-wait
							amqp.Table{},               // arguments
						); err != nil {
							vbl.Stdout.Warn(vbl.Id("vid/3505d4745978"), err)
						}
					}
				}

				for n := range iter.SeqFromList(3, *rtn.payload.UserIds) {

					go func(n iter.Ret[primitive.ObjectID]) {

						defer func() {
							n.StartNextTask()
							n.MarkTaskAsComplete()
						}()

						var userId = n.Value
						var user mngo_types.MongoChatUser

						{
							var filter = bson.M{"_id": userId}
							if err := c.MongoDb.Collection(virl_mongo.VibeChatUser).FindOne(bgCtx, filter).Decode(&user); err != nil {
								vbl.Stdout.Warn(vbl.Id("vid/3633ea9e01d6"), err, filter)
								return
							}
						}

						if user.Id.Hex() != userId.Hex() {
							vbl.Stdout.Warn(vbl.Id("vid/0507620b36d2"), "user id mismatch")
							return
						}

						if false { // TODO: maybe should be true not false
							q, err := ch.QueueDeclare(
								fmt.Sprintf("user-id:%s", userId.Hex()), // uuid.New().String(), // name
								false,                                   // durable
								false,                                   // delete when unused
								false,                                   // exclusive
								false,                                   // no-wait
								amqp.Table{
									"x-expires": 86400000 * 9, // 9 days
								},
								// args,                                                             // arguments
							)

							if err != nil {
								vbl.Stdout.Error(vbl.Id("vid/aca2208b3777"), err)
							} else {
								if err := ch.QueueBind(
									q.Name,                     // queue name
									chatId.Hex(),               // routing key
									virl_const.RMQMainExchange, // exchange
									true,                       // no-wait
									amqp.Table{},               // arguments
								); err != nil {
									vbl.Stdout.Warn(vbl.Id("vid/4aa3e31ac2ea"), err)
								} else {
									vbl.Stdout.Warn("fddd8b65-1e79-418f-a252-c494bc7ecca4", "done binding queue")
								}
							}
						}

						// TODO: do these async, but need to not close the channel too early above
						// do user devices asynchronously (do not wait for this)

						var filter = bson.M{"UserId": userId}
						var findOpts = options.Find().SetLimit(100).SetBatchSize(30).SetProjection(bson.D{})

						cur, err := c.M.FindManyCursor(vctx.NewStdMgoCtx(), c.M.Col.ChatUserDevices, filter, findOpts)

						if err != nil {
							vbl.Stdout.Warn(vbl.Id("vid/e29a2488130b"), err)
							return
						}

						defer func() {
							if err := cur.Close(context.Background()); err != nil {
								vbl.Stdout.Warn(vbl.Id("vid/48e86559e28d"), err)
							}
						}()

						for cur.Next(context.Background()) {
							// ////
							var result mngo_types.MongoChatUserDevice // Replace with your result type
							if err := cur.Decode(&result); err != nil {
								vbl.Stdout.Error(vbl.Id("vid/77efdfbd8d42"), err)
								continue
							}

							if result.Id.IsZero() {
								vbl.Stdout.Error(vbl.Id("vid/3b531520861e"), "zero object id")
								continue
							}

							if result.DeviceId.IsZero() {
								vbl.Stdout.Error(vbl.Id("vid/09298374c320"), "zero object id")
								continue
							}

							q, err := ch.QueueDeclare(
								fmt.Sprintf("user-id:%s:device-id:%s", userId.Hex(), result.DeviceId.Hex()), // uuid.New().String(), // name
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
								vbl.Stdout.Error(vbl.Id("vid/19d3bb0da246"), "could not declare queue:", err)
								continue
							}

							if err := ch.QueueBind(
								q.Name,                     // queue name
								chatId.Hex(),               // routing key
								virl_const.RMQMainExchange, // exchange
								true,                       // TODO: no-wait
								amqp.Table{},               // arguments
							); err != nil {
								vbl.Stdout.Error(vbl.Id("vid/aeccf87846b5"), "could not bind queue:", err)
								continue
							}

							vbl.Stdout.Warn(vbl.Id("vid/9ab25cbce487"),
								fmt.Sprintf("successfully bound queue '%s' to chat id '%s'", q.Name, chatId.Hex()))

						}

						if err := cur.Err(); err != nil {
							vbl.Stdout.Error(vbl.Id("vid/d4467dde7aff"), err)
						}

					}(n)
				}

				var cs = vbu.GetNewStacktraceFrom(cs)

				c.IO.SendMessageOutToMultipleTopics(
					cs,
					*rtn.payload.UserIdStrings,
					cx.User,
					struct {
						NewConvo bool
						ConvId   string
						UserIds  []string
					}{
						true,
						chatId.Hex(),
						*rtn.payload.UserIdStrings,
					},
				)

			}()

			vbl.Stdout.Info(vbl.Id("vid/1e2063a72347"), "sending response", responseBody)
			return c.GenericMarshal(200, responseBody)
		}))
}
