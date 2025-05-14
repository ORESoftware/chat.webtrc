// source file path: ./src/rest/routes/v1/health/handlers.go
package v1_routes_health

import (
	"fmt"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"github.com/oresoftware/chat.webrtc/src/rest/rt-settings"
	"math"
	"net/http"
	"os"
	"time"
)

func createHealthCheck(c *ctx.VibeCtx) http.HandlerFunc {

	var startTime = time.Now()
	hostname, err := os.Hostname()

	if err != nil {
		hostname = ""
	}

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			if !rt.IsHealthy() {
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "2f71d0c2-15e9-4b62-a171-9a636a5d3baa",
					Err:   fmt.Errorf("Server not healthy"),
				})
			}

			var duration = time.Now().Sub(startTime)

			return c.GenericMarshal(200, struct {
				OK            bool
				Hostname      string
				UptimeMillis  int64
				UptimeSeconds float64
				UptimeHours   float64
				UptimeDays    float64
			}{
				true,
				hostname,
				duration.Milliseconds(),
				duration.Seconds(),
				duration.Hours(),
				math.Floor(duration.Hours() / 24)})
		}))
}
