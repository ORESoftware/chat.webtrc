// source file path: ./src/rest/routes/v1/messages/routes.go
package v1_routes_messages

import (
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	"sync"
)

var once = sync.Once{}

func Mount(r *mux.Router, c *ctx.VibeCtx) {

	once.Do(func() {

	})

}
