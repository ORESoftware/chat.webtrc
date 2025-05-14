// source file path: ./src/rest/routes/v1/meta/routes.go
package v1_routes_meta

import (
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"sync"
)

var once = sync.Once{}

func Mount(r *mux.Router, c *ctx.VibeCtx) {

	once.Do(func() {

		r.Methods("GET").Path("/v1/meta/shutdown").HandlerFunc(
			mw.Middleware(
				createCleanShutdownHandler(c),
			),
		)

		r.Methods("GET").Path("/v1/meta/download/trace").HandlerFunc(
			mw.Middleware(
				createMetaDownloadHandler(c),
			),
		)

		r.Methods("GET").Path("/v1/meta/download/pprof").HandlerFunc(
			mw.Middleware(
				createMetaDownloadPprofHandler(c),
			),
		)

		c.CreateReg().
			QueryStringMatch(). // S
			HeadersMatch().
			Register(func(h *ctx.Hold) *mux.Route {
				return r.Methods("GET").Path("/v1/meta").HandlerFunc(
					mw.Middleware(
						mw.AsJSON(),
						createMetaHandler(c),
					),
				)
			})

	})

}
