// source file path: ./src/rest/routes/v1/login/model_helpers.go
package login

import (
	"github.com/google/uuid"
	"github.com/oresoftware/chat.webrtc/v1/common"
	dh "github.com/oresoftware/chat.webrtc/v1/common/db-helpers"
	"github.com/oresoftware/chat.webrtc/v1/routes/users"
)

func getNextState(userID uuid.UUID, c *common.CPContext) string {
	doneWithSurvey := users.IsDoneWithSurvey(userID, c)
	if !doneWithSurvey {
		return "survey"
	}
	isConnected := dh.IsConnected(userID, c)
	if !isConnected {
		return "connect"
	}
	return "bucket"
}
