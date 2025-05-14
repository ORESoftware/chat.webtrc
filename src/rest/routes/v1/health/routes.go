// source file path: ./src/rest/routes/v1/health/routes.go
package v1_routes_health

import (
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"net/http"
	"sync"
)

var once = sync.Once{}

func Mount(r *mux.Router, c *ctx.VibeCtx) {

	once.Do(func() {

		// we don't want to encounter errors from
		// accidentally calling Mount more than once

		c.CreateReg().
			QueryStringMatch(). // S
			HeadersMatch().
			Register(func(h *ctx.Hold) *mux.Route {
				return r.Methods("GET").Path("/v1/health/3000").HandlerFunc(
					mw.Middleware(
						mw.AsJSON(),
						createHealthCheck(c),
					),
				)
			})

		r.Methods("GET").PathPrefix("/v1/health?").HandlerFunc(
			mw.Middleware(
				mw.AsJSON(),
				createHealthCheck(c),
			),
		)

		r.Methods("GET").PathPrefix("/v1/health/").HandlerFunc(
			mw.Middleware(
				mw.AsJSON(),
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					w.Header().Set("x-vibe-error-id", "b81f3e10-92b6-4d0e-96a1-a1261940fce3")
					http.Error(w, "1768d657-fbf8-4069-b25b-ae0c4576d5ff - Invalid request format.", http.StatusBadRequest)
				}),
			),
		)

	})

}
