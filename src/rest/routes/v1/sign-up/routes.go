// source file path: ./src/rest/routes/v1/sign-up/routes.go
package sign_up

import (
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
	"github.com/oresoftware/chat.webrtc/common/conf"
	"github.com/oresoftware/chat.webrtc/common/throttle"
	"github.com/oresoftware/chat.webrtc/v1/common"
	"github.com/oresoftware/chat.webrtc/v1/middleware/mw"
	"time"
)

var log = logging.MustGetLogger("API")

type Controller struct {
	*common.CP
}

func init() {
	common.AddController(&Controller{})
}

func (ctr *Controller) Mount(r *mux.Router, c *common.CPContext) {

	var asJSON = mw.AsJSON()
	var rateLimitMiddleware mux.MiddlewareFunc = nil

	if conf.GetConf().IN_PROD {
		// in prod we rate limit sign-up to 15x in 2 minute span
		rateLimitMiddleware = throttle.MustCreateThrottle(c, "cp-signup", 2*time.Minute, 15)
	} else {
		rateLimitMiddleware = mw.PassThroughHandler()
	}

	r.Methods("POST").Path("/cp/sign_up").HandlerFunc(
		mw.Middleware(
			asJSON,
			rateLimitMiddleware,
			ctr.signUp(c),
		),
	)

	r.Methods("POST").Path("/cp/resend_verification").HandlerFunc(
		mw.Middleware(
			asJSON,
			mw.LoadUser(c),
			ctr.resendVerification(c),
		))
}
