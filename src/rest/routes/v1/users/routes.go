// source file path: ./src/rest/routes/v1/users/routes.go
package v1_routes_users

import (
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"sync"
)

var once = sync.Once{}

func Mount(r *mux.Router, c *ctx.VibeCtx) {

	once.Do(func() {

		c.CreateReg().
			QueryStringMatch(). // S
			HeadersMatch().
			Register(func(h *ctx.Hold) *mux.Route {
				return r.Methods("POST").Path("/v1/users/create-user").HandlerFunc(
					mw.Middleware(
						mw.AsJSON(),
						mw.BodyToMap(c),
						createUserHandler(c),
					),
				)
			})

	})

}
