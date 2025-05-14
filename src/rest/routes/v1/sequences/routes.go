// source file path: ./src/rest/routes/v1/sequences/routes.go
package v1_routes_sequences

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
				return r.Methods("GET").Path("/v1/sequence").HandlerFunc(
					mw.Middleware(
						mw.AsJSON(),
						createSequenceHandler(c),
					),
				)
			})

	})

}
