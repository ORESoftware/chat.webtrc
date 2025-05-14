// source file path: ./src/common/verrors/v-errors.go
package virl_err

import (
	vapm "github.com/oresoftware/chat.webrtc/src/common/apm"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
)

type ErrorWithCode struct {
	Code    int
	Message string
	// other fields
}

func (e ErrorWithCode) Error() string {
	return e.Message
}

func HandleErr(id string, msg interface{}, messages ...interface{}) {
	handleErr(id, msg, messages)
}

func handleErr(messages ...interface{}) {
	// /

	if len(messages) < 1 {
		vbl.Stdout.Error("87ccde76-ddab-4f65-8498-a7c8870f63a2", "empty messages list.")
		return
	}

	ErrorQueue.Enqueue(messages)

	if v, ok := messages[0].(string); ok {
		// TODO: extract id
		vbl.Stdout.Error(messages...)
		vapm.SendTrace(v, messages...)
	} else {
		vbl.Stdout.Error(messages...)
		vapm.SendTrace("f1d3beca-582c-44db-aa36-309222969845", messages...)
	}

}
