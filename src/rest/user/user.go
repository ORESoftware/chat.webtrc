// source file path: ./src/rest/user/user.go
package user

import (
	jlog "github.com/oresoftware/json-logging/jlog/lib"
	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sync"
)

type AuthUser struct {
	DeviceId    string
	DeviceObjId *primitive.ObjectID
	ClientObjId *primitive.ObjectID // clientId = userId + deviceId
	ClientId    string              // clientId = userId + deviceId
	UserObjId   *primitive.ObjectID
	UserId      string
	Log         *jlog.Logger
	RMQChannel  *amqp.Channel
	RMQMtx      sync.Mutex
	Mtx         sync.Mutex // control writes to websocket
	ConvIds     []string   // dynamic list of conv-ids that are subscribed to
	Tags        map[string]interface{}
}

func (uc *AuthUser) GetRMQChannel(conn *amqp.Connection) (*amqp.Channel, error) {

	uc.RMQMtx.Lock()
	defer uc.RMQMtx.Unlock()

	if uc.RMQChannel != nil {
		if !uc.RMQChannel.IsClosed() {
			return uc.RMQChannel, nil
		}
	}

	var ch, err = conn.Channel()

	if err != nil {
		return nil, err
	}

	// // TODO: disable confirm mode in prod, etc
	// if err := ch.Confirm(false); err != nil {
	//   vbl.Stdout.Error(vbl.Id("vid/adae764a111e"), fmt.Sprintf("Error enabling Confirm mode: %v", err))
	// }

	uc.RMQChannel = ch
	return ch, nil
}

func (v *AuthUser) Validate() []error {

	if v.ClientId != v.ClientObjId.Hex() {

	}

	if v.UserId != v.UserObjId.Hex() {

	}

	return nil

}
