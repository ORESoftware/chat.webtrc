// source file path: ./src/common/mongo/vctx/vctx.go
package vctx

import (
	"context"
	vutils "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"go.mongodb.org/mongo-driver/mongo"
	"time"
)

func NewStdMgoCtx() *MgoCtx {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*8)
	return &MgoCtx{
		ctx:    &ctx,
		Cancel: &cancel,
	}
}

func NewMgoCtxWTo(to time.Duration) *MgoCtx {
	ctx, cancel := context.WithTimeout(context.Background(), to)
	return &MgoCtx{
		ctx:    &ctx,
		Cancel: &cancel,
	}
}

func NewMgoSessionCtx(ctx *mongo.SessionContext) *MgoCtx {
	return &MgoCtx{
		sessionCtx: ctx,
		ctx:        nil,
		Cancel:     nil,
	}
}

func NewMgoCtx(ctx *context.Context) *MgoCtx {
	return &MgoCtx{
		ctx:    ctx,
		Cancel: nil,
	}
}

type canceler interface {
	Cancel()
}

type MgoCtx struct {
	sessionCtx     *mongo.SessionContext
	ctx            *context.Context
	Cancel         *context.CancelFunc
	AllowNoMatches bool
}

func (m *MgoCtx) GetCtx() context.Context {

	if m.sessionCtx != nil {
		return *m.sessionCtx
	}

	if m.ctx != nil {
		return *m.ctx
	}

	panic(vutils.JoinArgs("errId:", "44716631-3712-44c8-91ed-038dca862005", "this is not good"))
}

func (m *MgoCtx) DoCancel() {

	if m.ctx != nil {
		if canceller, ok := any(m.ctx).(context.CancelFunc); ok {
			canceller()
		}
		if canceller, ok := interface{}(m.ctx).(canceler); ok {
			canceller.Cancel()
		}
	}

	if m.Cancel != nil {
		(*m.Cancel)() // Dereference the pointer and call the function
	}

}
