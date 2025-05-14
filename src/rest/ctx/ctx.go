// source file path: ./src/rest/ctx/ctx.go
package ctx

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime/debug"

	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	muxctx "github.com/gorilla/context"
	"github.com/gorilla/mux"
	virl_kafka "github.com/oresoftware/chat.webrtc/src/common/kafka"
	virl_mongo "github.com/oresoftware/chat.webrtc/src/common/mongo"
	virl_rabbitmq "github.com/oresoftware/chat.webrtc/src/common/rabbitmq"
	virl_redis "github.com/oresoftware/chat.webrtc/src/common/redis"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	conf "github.com/oresoftware/chat.webrtc/src/config"
	vio "github.com/oresoftware/chat.webrtc/src/rest/io"
	jlog "github.com/oresoftware/json-logging/jlog/lib"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"sync"
)

type Hold struct {
	QueryStrings []interface{}
	Headers      []interface{}
	Payload      []interface{}
}

type Reg struct {
	Hold Hold
}

func (r *Reg) QueryStringMatch(v ...interface{}) *Reg {
	r.Hold.QueryStrings = append(r.Hold.QueryStrings, v)
	return r
}

func (r *Reg) HeadersMatch(v ...interface{}) *Reg {
	r.Hold.Headers = append(r.Hold.Headers, v)
	return r
}

func (r *Reg) PayloadMatch(v ...interface{}) *Reg {
	r.Hold.Payload = append(r.Hold.Payload, v)
	return r
}

func (r *Reg) Register(z func(r *Hold) *mux.Route) *mux.Route {
	return z(&r.Hold)
}

type VibeCtx struct {
	RedisConnPool *virl_redis.RedisConnectionPool
	RMQConnPool   *virl_rabbitmq.RabbitMQConnectionPool
	Conf          *conf.ConfigVars
	M             *virl_mongo.M
	MongoDb       *mongo.Database
	Routes        []RouteInfo
	Log           *jlog.Logger
	IORead        *vio.IOForRead
	IO            *vio.IOForREST
}

type KV struct {
	Key    string
	Values []string
}

type RouteInfo struct {
	Path    string
	Method  string
	Headers []KV
	Payload []KV
	Query   []KV
}

type LoggedInUser struct {
	UserId primitive.ObjectID
}

func (v *VibeCtx) RegisterRoute(x *mux.Route) {
	v.Routes = append(v.Routes, RouteInfo{
		Path: x.GetName(),
	})
}

func NewVibeCtx(
	conf *conf.ConfigVars,
	v *virl_mongo.M,
	p *virl_rabbitmq.RabbitMQConnectionPool,
	kp *virl_kafka.KafkaProducerConnPool,
	kcf *kafka.ConfigMap,
	rcp *virl_redis.RedisConnectionPool,
) *VibeCtx {
	// ////

	return &VibeCtx{
		Conf:          conf,
		M:             v,
		MongoDb:       v.DB,
		RMQConnPool:   p,
		RedisConnPool: rcp,
		IORead: &vio.IOForRead{
			Config:                 conf,
			RabbitMQConnectionPool: p,
			M:                      v,
			RedisConnPool:          rcp,
			KafkaConnectionPool:    kp,
			KafkaProducerConf:      kcf,
			KillOnce:               &sync.Once{},
			Mtx:                    &sync.Mutex{},
		},
		IO: &vio.IOForREST{
			Config:                 conf,
			RabbitMQConnectionPool: p,
			M:                      v,
			RedisConnPool:          rcp,
			KafkaConnectionPool:    kp,
			KafkaProducerConf:      kcf,
			KillOnce:               &sync.Once{},
			Mtx:                    &sync.Mutex{},
		},
	}
}

func (c *VibeCtx) CreateReg() *Reg {
	return &Reg{}
}

func (c *VibeCtx) GetDefaultCtx() context.Context {
	return context.Background()
}

func (v *VibeCtx) GenericMarshal(code int, x interface{}) (int, []byte) {
	return v.GenericMarshalWithError(code, x, nil)
}

func (v *VibeCtx) GenericMarshalWithErrorFirst(code int, err error, x interface{}) (int, []byte) {

	if err != nil {
		return v.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
			ErrId: "aa1787a7-8b19-4f8e-87db-146756fc88ec",
			Err:   err,
		})
	}

	z, err := json.Marshal(x)

	if err != nil {
		return v.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
			ErrId: "e3e98bb7-e70a-44b8-80be-c60baacb14a2",
			Err:   err,
		})
	}

	return code, z
}

func (v *VibeCtx) GenericPlainMarshal(x interface{}) []byte {

	z, err := json.Marshal(x)

	if err == nil {
		return z
	}

	z, err = json.Marshal(vibe_types.ErrorResponseData{
		Err:   err,
		ErrId: "e61b26f1-d314-41e9-ab8e-dbfe66e7795b",
	})

	if err == nil {
		return z
	}

	return []byte(`{"error_id":"f3053484-c42d-4121-90c2-e31ea769653f","error":"unknown error"}`)
}

func (v *VibeCtx) WriteJSONErrorResponse(code int, erd vibe_types.ErrorResponseData) (int, []byte) {

	var localTrace = vbu.GetLocalStackTrace()

	if code < 400 {
		vbl.Stdout.Warn(vbl.Id("vid/a2894d4c0861"), "error response code should be greater than or equal to 400.")
		code = 500
	}

	if code >= 500 {
		vbl.Stdout.ErrorF("Internal error: error id: '%s' %v", erd.ErrId, erd.Err)
	} else {
		vbl.Stdout.WarnF("Request hit problem: error id: '%s' %v", erd.ErrId, erd.Err)
	}

	if conf.GetConf().STACK_TRACES {
		vbl.Stdout.Warn(erd.Err, "=> short stack trace:", localTrace)
	}

	if conf.GetConf().FULL_STACK_TRACES {
		vbl.Stdout.Error(erd.Err, "long stack trace:", debug.Stack())
	}

	if erd.Reason != "" {
		// rollbar.Log("WARN", erd.Reason, map[string]interface{}{
		//	"stack_trace": localTrace,
		// })
		vbl.Stdout.Warn(vbl.Id("vid/8268bb7e758c"), erd.Reason)
	}

	if erd.Message != "" {

		if erd.Message != erd.Reason {
			// rollbar.Log("WARN", erd.Reason, map[string]interface{}{
			//	"stack_trace": localTrace,
			// })
		}

		vbl.Stdout.Warn(vbl.Id("vid/df53f91d0d7f"), erd.Message)
	}

	if erd.PrivateMessage != "" {
		vbl.Stdout.Warn(vbl.Id("vid/87a5b571de4a"), erd.PrivateMessage)
	}

	// overwrite the private into, just in case
	erd.PrivateMessage = ""

	if erd.Err != nil {

		// rollbar.Log("ERROR", erd.Err, map[string]interface{}{
		//	"stack_trace": localTrace,
		// })

		if erd.Reason == "" {
			erd.Reason = erd.Err.Error()
		}

		if erd.Message == "" {
			erd.Message = erd.Err.Error()
		}

		// if pgerr, ok := erd.Err.(*pq.Error); ok {
		//	fmt.Println(aw.Blue("Postgres error code: "), pgerr.Code)
		//	fmt.Println(aw.Blue("Postgres error message: "), pgerr.Messages)
		//	fmt.Println(aw.Blue("Postgres error detail: "), pgerr.Detail)
		// }

	}

	if erd.Message == "" {
		erd.Message = erd.Reason
	}

	if erd.Reason == "" {
		erd.Reason = erd.Message
	}

	if erd.StatusCode != 0 && erd.StatusCode != code {
		vbl.Stdout.Warn(
			vbl.Id("vid/684349cfbce1"),
			fmt.Sprintf("status codes were not the same: '%v' '%v'", erd.StatusCode, code),
		)
	}

	erd.StatusCode = code
	return v.GenericMarshal(code, erd)
}

func (v *VibeCtx) GenericMarshalWithErr(code int, err error, x interface{}) (int, []byte) {

	if err != nil {
		return v.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
			ErrId: "88656071-8ded-4b11-a678-8c366f7e0686",
			Err:   err,
		})
	}

	z, err := json.Marshal(x)

	if err != nil {
		return v.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
			ErrId: "528d02f4-35b9-441d-9784-5f881fda5104",
			Err:   err,
		})
	}

	return code, z
}

func (v *VibeCtx) GenericMarshalWithError(code int, x interface{}, err error) (int, []byte) {
	return v.GenericMarshalWithErr(code, err, x)
}

func (c *VibeCtx) MustGetFilterQueryParamMap(req *http.Request) *vibe_types.BodyMap {
	ptr := muxctx.Get(req, "req-query-param-filter-map")
	v, ok := ptr.(*vibe_types.QueryMap)
	if !ok {
		panic(vbu.ErrorFromArgs(
			"b577a82a-a850-4dc6-bcd3-0971d1ee3a97:",
			"could not cast to QueryParam{} type"))
	}
	return v
}

func (c *VibeCtx) MustGetRequestBodyMap(req *http.Request) *vibe_types.BodyMap {
	ptr := muxctx.Get(req, "req-body-map")
	v, ok := ptr.(*vibe_types.BodyMap)
	if !ok {
		panic(vbu.ErrorFromArgs(
			"b577a82a-a850-4dc6-bcd3-0971d1ee3a97:",
			"could not cast to BodyMap{} type"))
	}
	return v
}

func (c *VibeCtx) MustGetLoggedInUser(req *http.Request) *LoggedInUser {
	ptr := muxctx.Get(req, "logged-in-user")

	// vibelog.Stdout.Warn(vbl.Id("vid/4ee8c8e1788c"), "logged in user:", ptr)

	var s = req.Header.Get("x-vibe-user-id")

	vbl.Stdout.Warn(vbl.Id("vid/c3d7aaa78502"), "x-vibe-user-id:", s)
	val, err := primitive.ObjectIDFromHex(s)

	if err != nil {
		panic(vbu.JoinArgs("82e88577-3edb-458e-a6b2-1bdb641611b3", err.Error()))
	}

	return &LoggedInUser{
		UserId: val,
	}

	v, ok := ptr.(LoggedInUser)

	if !ok {
		panic(vbu.JoinArgs("0a6770ee-c635-44fb-b3a8-595974d390e2:", "could not cast to logged-in-user"))
	}

	return &v
}

func (c *VibeCtx) GetLoggedInUser(req *http.Request) (*LoggedInUser, error) {
	ptr := muxctx.Get(req, "logged-in-user")
	v, ok := ptr.(*LoggedInUser)

	if !ok {
		return nil, vbu.ErrorFromArgs("8a316766-c0bd-45bd-b978-1fe6e0bff55c:", "could not cast to logged-in-user")
	}
	return v, nil
}
