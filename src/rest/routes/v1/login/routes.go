// source file path: ./src/rest/routes/v1/login/routes.go
package login

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
		// in prod we rate limit login to 15x in 2 minute span
		rateLimitMiddleware = throttle.MustCreateThrottle(c, "cp-login", 2*time.Minute, 15)
	} else {
		rateLimitMiddleware = mw.PassThroughHandler()
	}

	r.Methods("POST").Path("/cp/login").HandlerFunc(
		mw.Middleware(
			asJSON,
			rateLimitMiddleware,
			ctr.loginHandler(c)),
	)

	r.Methods("POST").Path("/cp/users/forgot").HandlerFunc(
		mw.Middleware(
			asJSON,
			ctr.forgotPasswordHandler(c)),
	)

	r.Methods("POST").Path("/cp/users/forgot/reset").HandlerFunc(
		mw.Middleware(
			asJSON,
			ctr.resetPasswordHandler(c)),
	)

	r.Methods("GET").Path("/cp/login/logout").HandlerFunc(
		mw.Middleware(
			asJSON,
			ctr.logoutHandler(c)),
	)

}
