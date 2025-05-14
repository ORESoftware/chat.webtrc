// source file path: ./src/rest/routes/v1/sequences/handlers.go
package v1_routes_sequences

import (
	conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"net/http"
)

func createSequenceHandler(c *ctx.VibeCtx) http.HandlerFunc {

	// var gid = os2.Getegid()
	var _ = conf.GetConf()

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			// c.M.FindAndUpdateOne

			return c.GenericMarshal(200, struct {
				GitCommit string
				Hostname  string
			}{
				"gitCommit",
				"hostname",
			})
		}))
}
