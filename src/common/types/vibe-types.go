// source file path: ./src/common/types/vibe-types.go
package vibe_types

import (
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

//easyjson:json
type Trace struct {
	ID              string                 `json:"id"`
	Hash            string                 `json:"hash"`
	UserID          string                 `json:"userId"`
	DeviceID        string                 `json:"deviceId"`
	DeviceType      string                 `json:"deviceType"`
	DeviceVersion   string                 `json:"deviceVersion"`
	TriggeredNotifs bool                   `json:"triggeredNotifs"`
	RepoName        string                 `json:"repoName"`
	CommitId        string                 `json:"commitId"`
	FileName        string                 `json:"fileName"`
	LineNumber      int                    `json:"lineNumber"`
	EventName       string                 `json:"eventName"`
	Env             string                 `json:"env"`
	PriorityLevel   int                    `json:"priorityLevel"`
	EventData       []interface{}          `json:"eventData"`
	ErrorTrace      []interface{}          `json:"errorTrace"`
	CreatedAt       time.Time              `json:"createdAt"`
	UpdatedAt       time.Time              `json:"updatedAt"`
	CreatedBy       string                 `json:"createdBy"`
	UpdatedBy       string                 `json:"updatedBy"`
	Tags            map[string]interface{} `json:"tags"`
}

func (r *Trace) Error() string {
	val, err := json.Marshal(r)
	if err != nil {
		return fmt.Sprintf(`{"ErrorId":"%s","Values":"%v"}`, r.Hash, r.ErrorTrace)
	}
	return string(val)
}

type BodyMap = map[string]interface{}
type QueryMap = map[string]interface{}

// these are the types

//easyjson:json
type KafkaTopicsPollPayload struct {
	TopicIds []string
}

//easyjson:json
type WsClientAck struct {
	Ack       bool
	Reason    string
	ConvId    string
	MessageId string
}

//easyjson:json
type ErrorResponseData struct {
	ErrId          string `json:"error_id"`
	Err            error  `json:"-"`
	PrivateMessage string `json:"-"`
	Message        string `json:"message"`
	Reason         string `json:"reason"`
	StatusCode     int    `json:"status_code"`
}

//easyjson:json
type LoggedInUser struct {
	UserId    string
	UserObjId primitive.ObjectID
	Model     interface{}
	Roles     []interface{}
	JWTClaims map[string]interface{}
}

//easyjson:json
type RabbitPayload struct {
	Rabbit    bool // struct marker
	Topic     string
	TopicType string
	Meta      interface{}
	Data      interface{}
}

func (r *RabbitPayload) BumpReplayCount() {

}

func (r *RabbitPayload) GetReplayCount() int {
	return 0
}

//easyjson:json
type KafkaPayload struct {
	Kafka bool // struct marker
	Meta  struct {
		IsList      bool
		TimeCreated string
	}
	Data interface{}
}

func (r *KafkaPayload) BumpReplayCount() {

}

func (r *KafkaPayload) GetReplayCount() int {
	return 0
}

//easyjson:json
type RedisPayload struct {
	Redis bool // struct marker
	Meta  struct {
		IsList      bool
		TimeCreated string
	}
	Data interface{}
}

func (r *RedisPayload) BumpReplayCount() {

}

func (r *RedisPayload) GetReplayCount() int {
	return 0
}
