// source file path: ./src/rest/middleware/tracer.go
package mw

import (
	"fmt"
	"github.com/gorilla/mux"
	au "github.com/oresoftware/chat.webrtc/src/common/v-aurora"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	"golang.org/x/crypto/ssh/terminal"
	"net/http"
	"os"
	"strconv"
	"time"
)

var isTerminal = terminal.IsTerminal(int(os.Stdout.Fd()))

func GetStylizedStatusCode(code int) string {

	if !isTerminal {
		// if stdio is not attached to a terminal we should not be using colors
		return strconv.Itoa(code)
	}

	if conf.GetConf().IN_PROD == true {
		return strconv.Itoa(code)
	}

	if code < 200 {
		return au.Col.BgBrightMagenta(strconv.Itoa(code)).String()
	}

	if code < 300 {
		return au.Col.BrightBlue(strconv.Itoa(code)).String()
	}

	if code < 400 {
		return au.Col.Green(strconv.Itoa(code)).String()
	}

	if code < 500 {
		return au.Col.Yellow(strconv.Itoa(code)).String()
	}

	return au.Col.Red(strconv.Itoa(code)).String()
}

func Tracer(c *ctx.VibeCtx) mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {

		var healthExactPath = []string{
			"/v1/health",
		}

		var healthPathPrefixes = []string{
			"/v1/health?",
			"/v1/health/",
			"/v1/health#",
		}

		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			start := time.Now()

			addr := req.Header.Get("X-Real-IP")
			if addr == "" {
				addr = req.Header.Get("X-Forwarded-For")
				if addr == "" {
					addr = req.RemoteAddr
				}
			}

			h.ServeHTTP(res, req)

			statusCode := -1

			if v, ok := res.(*LoggingResponseWriter); ok {
				statusCode = v.StatusCode
			}

			ttms := time.Since(start).Seconds() * 1000

			if vbu.DoesNotEqualAny(req.URL.Path, healthExactPath) &&
				vbu.DoesNotStartWithAny(req.URL.Path, healthPathPrefixes) {
				// we don't want to fill up the logs with health check stuff
				vbl.Stdout.Info(
					vbl.Id("vid/fcfb891ccd69"),
					fmt.Sprintf(
						"%v %s - %s %s %v, in millis: [ms=%0.2f]",
						GetStylizedStatusCode(statusCode),
						http.StatusText(statusCode),
						req.Method,
						req.URL.Path,
						time.Since(start),
						ttms,
					),
				)
			}

			if time.Since(start) > 750*time.Millisecond {
				vbl.Stdout.Warn(
					vbl.Id("vid/68f567b6b24e"),
					fmt.Sprintf(
						"%s %v %s %s %s %v for %s [ms=%0.2f]",
						"Warning - slow request/response:",
						GetStylizedStatusCode(statusCode),
						http.StatusText(statusCode),
						req.Method,
						req.URL.Path,
						time.Since(start),
						addr,
						ttms,
					),
				)
			}
		})
	}

}
