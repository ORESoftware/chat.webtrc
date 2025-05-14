// source file path: ./src/rest/middleware/hooks.go
package mw

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	"net/http"
)

func LogInfoUponBadRequest(c *ctx.VibeCtx) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			defer func() {

				var errId = w.Header().Get("cp-error-id")

				statusCode := -1
				errResponseBody := ""

				if v, ok := w.(*LoggingResponseWriter); ok {
					statusCode = v.StatusCode
					errResponseBody = v.ErrResponseBody
				}

				if errResponseBody == "" {
					return
				}

				if errId == "" {
					var m struct {
						ErrorId string `json:"error_id"`
					}
					if err := json.Unmarshal([]byte(errResponseBody), &m); err == nil {
						errId = m.ErrorId
					}
				}

				if errId == "" && statusCode < 400 {
					return
				}

				var u, err = c.GetLoggedInUser(r)

				if err != nil {
					return
				}

				if u == nil {
					return
				}

				var userId = u.UserId

				if errId != "" {
					errResponseBody = "(intentionally omitted)"
				}

				errMessage := fmt.Sprintf("the error id: '%s' (see logged in user id in metadata)", errId)
				vbl.Stdout.Warn(errMessage)

				// rollbar.Log("ERROR", errMessage, map[string]interface{}{
				//	"user_id": userId,
				// })

				vbl.Stdout.WarnF(
					"%v %s %s => logged-in user id with id = '%s', experienced error id: '%s', response body: '%s'",
					statusCode, r.Method, r.URL.Path, userId, errId, errResponseBody,
				)

			}()

			next.ServeHTTP(w, r)

		})
	}

}
