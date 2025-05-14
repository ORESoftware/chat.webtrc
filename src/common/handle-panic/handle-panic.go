// source file path: ./src/common/handle-panic/handle-panic.go
package vhp

import (
	vapm "github.com/oresoftware/chat.webrtc/src/common/apm"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/rest/user"
	vuc "github.com/oresoftware/chat.webrtc/src/ws/uc"
)

func HandlePanic(id string) interface{} {
	if r := recover(); r != nil {
		vbl.Stdout.Error("0835374a-8525-440b-b9db-cf79f2371b4a", r)
		vapm.SendTrace(id, r)
		return r
	}
	return nil
}

func HandlePanicWithUserCtxFromWebsocket(id string, uc *vuc.UserCtx) interface{} {
	if r := recover(); r != nil {
		vbl.Stdout.Error("389171d6-4547-419d-b39f-1ab3e55a6dba", r)
		vapm.SendTrace(id, r)
		return r
	}
	return nil
}

func HandlePanicWithRESTUserCtx(id string, uc *user.AuthUser) interface{} {
	if r := recover(); r != nil {
		vbl.Stdout.Error("42023aaa-8e98-4842-b302-ea92b24a09ba", r)
		vapm.SendTrace(id, r)
		return r
	}
	return nil
}
