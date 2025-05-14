// source file path: ./src/common/mongo/runtime-validation/validators.go
package runtime_validation

import (
	mngo_types "github.com/oresoftware/chat.webtrc/src/common/mongo/types"
	vibe_types "github.com/oresoftware/chat.webtrc/src/common/types"
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog"
)

type InputValidator struct {
}

func (v *InputValidator) validateRabbitPayload(vp []string, m vibe_types.RabbitPayload) []string {
	// vp is validationProblems
	return vp
}

func (v *InputValidator) validateRedisPayload(vp []string, payload vibe_types.RedisPayload) []string {
	return vp
}

func (v *InputValidator) validateMongoChatConv(vp []string, conv mngo_types.MongoChatConv) []string {
	return vp
}

func (v *InputValidator) validateMongoChatMessage(vp []string, message mngo_types.MongoChatMessage) []string {
	return vp
}

func (v *InputValidator) validateMongoMessageAck(vp []string, ack mngo_types.MongoMessageAck) []string {
	return vp
}

func (v *InputValidator) validateMongoChatUser(vp []string, user mngo_types.MongoChatUser) []string {
	return vp
}

func (v *InputValidator) validateMongoChatUserDevice(vp []string, device mngo_types.MongoChatUserDevice) []string {
	return vp
}

type InsertValidator struct {
}

func (v *InsertValidator) validateRabbitPayload(vp []string, m vibe_types.RabbitPayload) []string {
	return vp
}

var inputValidator = InputValidator{}
var insertValidator = InsertValidator{}

func ValidateBeforeInsert(m interface{}) []string {

	// TODO: let's not use this

	if m == nil {
		return []string{"errid:", "347085a8-1bd2-497b-86c0-d7f51fe9a8e6", "nil pointer"}
	}

	var vp = []string{} // no problems yet!

	switch m.(type) {

	case vibe_types.RabbitPayload:
		return inputValidator.validateRabbitPayload(vp, m.(vibe_types.RabbitPayload))

	case vibe_types.RedisPayload:
		return inputValidator.validateRedisPayload(vp, m.(vibe_types.RedisPayload))

	case mngo_types.MongoChatConv:
		return inputValidator.validateMongoChatConv(vp, m.(mngo_types.MongoChatConv))

	case mngo_types.MongoChatMessage:
		return inputValidator.validateMongoChatMessage(vp, m.(mngo_types.MongoChatMessage))

	case mngo_types.MongoMessageAck:
		return inputValidator.validateMongoMessageAck(vp, m.(mngo_types.MongoMessageAck))

	case mngo_types.MongoChatUser:
		return inputValidator.validateMongoChatUser(vp, m.(mngo_types.MongoChatUser))

	case mngo_types.MongoChatUserDevice:
		return inputValidator.validateMongoChatUserDevice(vp, m.(mngo_types.MongoChatUserDevice))

	default:
		vbl.Stdout.Debug(vbl.Id("vid/4380285e04dc"), "switch statement fall-through")
		return []string{}
		// return []string{"error:", "6a1c6fad-e60b-41e2-b201-168f082d527b"}

	}
}

func ValidateClientInput(m interface{}) []string {

	var validationProblems = []string{} // no problems yet!

	switch m.(type) {

	case vibe_types.RabbitPayload:
		return insertValidator.validateRabbitPayload(
			validationProblems,
			m.(vibe_types.RabbitPayload),
		)

	default:
		vbl.Stdout.Warn(vbl.Id("vid/228deb74e4b1"), "switch statement fall-through")
		return []string{"error:", "feed2e82-6e18-4956-8cd4-8b452fa1e414"}
	}

}
