// source file path: ./src/rest/middleware/mw.go
package mw

import (
	"bytes"
	"encoding/json"
	"fmt"
	muxctx "github.com/gorilla/context"
	"github.com/gorilla/mux"
	vhp "github.com/oresoftware/chat.webrtc/src/common/handle-panic"
	au "github.com/oresoftware/chat.webrtc/src/common/v-aurora"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	common "github.com/oresoftware/chat.webrtc/src/rest/middleware/dupecheck"
	"github.com/oresoftware/chat.webrtc/src/rest/user"
	jlog "github.com/oresoftware/json-logging/jlog/lib"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
)

var cfg = vbcf.GetConf()

func BodyLogging(c *ctx.VibeCtx) mux.MiddlewareFunc {

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

			if cfg.IN_PROD {
				// we don't want to log this potentially sensitive info in prod
				next.ServeHTTP(res, req)
				return
			}

			if req.Body == nil {
				vbl.Stdout.Warn(vbl.Id("vid/8fcce5ac54ee"), "Body was nil")
				next.ServeHTTP(res, req)
				return
			}

			bodyBytes, _ := ioutil.ReadAll(req.Body)
			if err := req.Body.Close(); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/696edb8b488a"), err)
				// don't return here since we need to restore the request body
			}

			// Restore the io.ReadCloser to its original state
			req.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))

			var d = map[string]interface{}{}

			if err := json.Unmarshal(bodyBytes, &d); err != nil {

				if len(bodyBytes) > 0 {
					vbl.Stdout.WarnF("8c5d267f-6d49-45f2-b0cf-beefec8cd0b9: error unmarshing req body: %v", err)
				}
				next.ServeHTTP(res, req)
				return
			}

			vbl.Stdout.Info(vbl.Id("vid/d908f50f1ed3"), au.Col.Gray(10, "json body key/values for:"), au.Col.Bold(req.Method), au.Col.Gray(6, req.URL))

			// TODO: potentially just use vibelog for this
			for key, val := range d {

				if v, ok := val.(string); ok {
					val = "'" + au.Col.Green(v).String() + "'"
				} else {
					v, err := json.Marshal(val)

					if err != nil {
						val = au.Col.Red(fmt.Sprintf("%v", val)).String()
					} else {
						val = au.Col.Yellow(string(v)).String()
					}
				}

				vbl.Stdout.Info(
					vbl.Id("vid/7193dbe1ee9c"),
					"'"+au.Col.Green(key).Bold().String()+"':", val,
				)
			}
			vbl.Stdout.Info(vbl.Id("vid/e9a02ec8bb3b"), "(end json body)")
			next.ServeHTTP(res, req)
		})
	}

}

func AsJSON() mux.MiddlewareFunc {

	common.DupeCheck.CheckUuid("cm:a7417192-434d-46ff-ae1b-98c18a600b00")

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			h.ServeHTTP(w, r)
		})
	}
}

func allowOrigin(res http.ResponseWriter, origin string, custHead string) {
	res.Header().Set("Access-Control-Allow-Origin", origin)
	res.Header().Set("Access-Control-Allow-Methods", "GET,PUT,POST,DELETE,PATCH")
	if custHead != "" {
		res.Header().Set("Access-Control-Allow-Headers", custHead)
	} else {
		res.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept")
	}
	res.Header().Set("Access-Control-Expose-Headers", "Content-Type, Results-Count, Query-Skip, Query-Limit, Next-Set, Prev-Set, Content-Length")
	res.Header().Set("Access-Control-Allow-Credentials", "true")
}

type OriginDomain = func(origin string) string

func originContains(origin string, acceptableOrigins []string) bool {
	for _, v := range acceptableOrigins {
		if strings.Contains(strings.ToLower(origin), v) {
			return true
		}
	}
	return false
}

func checkOriginDomain(origin string) bool {
	if cfg.IN_PROD {
		return originContains(origin, []string{"creator.cash", "adm1.vibeirl.com"})
	}
	return originContains(origin, []string{"creator.cash", "adm1.vibeirl.com", "localhost"})
}

func MakeOrigin(originDomain OriginDomain, c *ctx.VibeCtx) mux.MiddlewareFunc {

	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			origin := originDomain(req.Header.Get("Origin"))
			custHead := req.Header.Get("Access-Control-Request-Headers")

			if req.Method != "OPTIONS" {
				// temporarily changing !productionMode to  localhost
				if checkOriginDomain(origin) {
					allowOrigin(res, req.Header.Get("Origin"), custHead)
				}
			}

			h.ServeHTTP(res, req)
		})
	}

}

func doResponse(x func(*Ctx, http.ResponseWriter, *http.Request) (int, []byte), uc *Ctx, w http.ResponseWriter, r *http.Request) {

	responseCode, bytesToWrite := x(uc, w, r)

	if responseCode < 1 {
		// if responseCode is less than 1 we do not write it
		return
	}

	if v, ok := w.(*LoggingResponseWriter); ok {
		v.ErrResponseBody = string(bytesToWrite)
	}

	w.WriteHeader(responseCode)

	if len(bytesToWrite) < 1 {
		// don't need to call write in this case and we can avoid the unnecessary warning being logged
		return
	}

	_, err := w.Write(bytesToWrite)
	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/0f16d4540d89"), err)
	}
}

type Ctx struct {
	User      *user.AuthUser
	Log       *jlog.Logger
	FilterMap *map[string]interface{}
	BodyMap   *map[string]interface{}
}

func (c *Ctx) GetUser() *user.AuthUser {
	return c.User
}

func DoResponse(x func(*Ctx, http.ResponseWriter, *http.Request) (int, []byte)) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//
		var uc = muxctx.Get(r, "req-res-ctx")
		var bodyMapRaw = muxctx.Get(r, "req-body-map")
		var filterMapRaw = muxctx.Get(r, "req-query-param-filter-map")
		// TODO: can optimize this so that we don't always create 2 maps for every request
		var bodyMap, ok1 = bodyMapRaw.(*map[string]interface{})
		if !ok1 || bodyMap == nil {
			bodyMap = &map[string]interface{}{}
		}
		var filterMap, ok2 = filterMapRaw.(*map[string]interface{})
		if !ok2 || filterMap == nil {
			filterMap = &map[string]interface{}{}
		}

		if uc == nil {
			uc = &Ctx{}
		}

		if z, ok := uc.(Ctx); ok {
			// unlikely event that uc points to value not pointer
			uc = &z
		}

		if z, ok := uc.(*Ctx); ok {
			z.BodyMap = bodyMap
			z.FilterMap = filterMap

			if z.User == nil {
				z.User = &user.AuthUser{
					RMQMtx: sync.Mutex{},
					Mtx:    sync.Mutex{},
					Log:    vbl.Stdout.Child(&map[string]interface{}{}),
					UserId: "<unknown>",
				}
			}

			if z.Log == nil {
				z.Log = vbl.Stdout.Child(&map[string]interface{}{
					"UserId": z.User.UserId,
				})
			}

			doResponse(x, z, w, r)
			return
		}

		panic(vbu.ErrorFromArgs("0f4dcdb2-a565-43d3-b920-5092f28a332f", "should be a ptr to Ctx"))
	})
}

func isObjectIdEmpty(id primitive.ObjectID) bool {
	return id == primitive.NilObjectID
}

func Recovery(c *ctx.VibeCtx) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				// TODO: use different ids (for different users?) for handlePanic not only "8bd2f069c78e"
				if rc := vhp.HandlePanic("8bd2f069c78e"); rc != nil {

					vbl.Stdout.Error(vbl.Id("vid/3af391deb908"), "Caught error in defer/recover middleware: ", rc)

					var localTrace = vbu.GetLocalStackTrace()
					vbl.Stdout.Error(vbl.Id("vid/7a552c6b679e"), rc, "stack trace:", localTrace)

					var u, _ = c.GetLoggedInUser(req)

					if u != nil && !isObjectIdEmpty(u.UserId) {
						var userId = u.UserId

						vbl.Stdout.WarnF(
							"%s %s => logged-in user id with id = '%s', experienced error: '%v'",
							req.Method, req.URL.Path, userId, rc,
						)

						// rollbar.Log("ERROR", rc, map[string]interface{}{
						//	"user_id":     userId,
						//	"stack_trace": localTrace,
						// })

					} else {
						// no user available, so just log error
						// rollbar.Log("ERROR", rc, map[string]interface{}{
						//	"stack_trace": localTrace,
						// })
					}

					if cfg.FULL_STACK_TRACES {
						vbl.Stdout.Info(vbl.Id("vid/fdcfc07aa376"), "long stack trace:")
						vbl.Stdout.Error(vbl.Id("vid/c3813f2b8256"), rc)
					}

					message := ""
					statusCode := 500

					if v, ok := rc.(struct{ StatusCode int }); ok {
						statusCode = v.StatusCode
					}

					if statusCode > 0 {
						vbl.Stdout.Error(
							vbl.Id("vid/928974da8473"),
							"Sending status response with status code:",
							au.Col.Red(statusCode).Bold().String(),
						)
						w.WriteHeader(statusCode)
					} else {
						vbl.Stdout.Error(
							vbl.Id("vid/278397d14c3a"),
							"Sending status response with status code:",
							au.Col.Red(http.StatusInternalServerError).Bold().String(),
						)
						w.WriteHeader(http.StatusInternalServerError)
					}

					if _, ok := rc.(error); ok {
						json.NewEncoder(w).Encode(struct {
							Message string
						}{
							rc.(error).Error(),
						})
						return
					}

					if message, ok := rc.(string); ok {
						json.NewEncoder(w).Encode(struct {
							Message string
						}{
							message,
						})
						return
					}

					if v, ok := rc.(struct{ OriginalError error }); ok {
						vbl.Stdout.Error(vbl.Id("vid/f87abf107b79"), "Caught error in defer/recover middleware: ", rc)
						originalError := v.OriginalError

						if originalError != nil {
							vbl.Stdout.Error(vbl.Id("vid/347cc49a3fff"), "Original error in defer/recover middleware: ", originalError)
						}
					}

					if v, ok := rc.(struct{ Message string }); ok {
						message = v.Message
					}

					if message == "" {
						message = "Unknown error message."
					}

					json.NewEncoder(w).Encode(struct {
						Message string
					}{
						message,
					})

				}

			}()

			next.ServeHTTP(w, req)

		})
	}

}

func Options(originDomain OriginDomain, c *ctx.VibeCtx) http.Handler {
	return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		origin := originDomain(req.Header.Get("Origin"))
		custHead := req.Header.Get("Access-Control-Request-Headers")
		// temporarily changing !productionMode to  localhost
		if checkOriginDomain(origin) {
			allowOrigin(res, req.Header.Get("Origin"), custHead)
		}
		res.Header().Set("Access-Control-Max-Age", "3600")
		res.WriteHeader(200)
		if _, err := res.Write([]byte("")); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/6b753fe9c112"), err)
		}
		// return 200, ""
	})
}

func DebugOptions() mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
			if strings.ToUpper(req.Method) == "OPTIONS" {
				panic(vbu.ErrorFromArgs("322ecec0-1697-482f-9f7a-3429b171de84", "options req should not make it here"))
			}
			h.ServeHTTP(res, req)
		})
	}
}

func MakeOptions(originDomain OriginDomain, c *ctx.VibeCtx) mux.MiddlewareFunc {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

			if strings.ToUpper(req.Method) != "OPTIONS" {
				h.ServeHTTP(res, req)
				return
			}

			origin := originDomain(req.Header.Get("Origin"))
			custHead := req.Header.Get("Access-Control-Request-Headers")
			// temporarily changing !productionMode to  localhost
			if checkOriginDomain(origin) {
				allowOrigin(res, req.Header.Get("Origin"), custHead)
			}
			res.Header().Set("Access-Control-Max-Age", "3600")
			res.WriteHeader(200)
			if _, err := res.Write([]byte("")); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/deb47cc078a7"), err)
			}
			// we do not call h.ServeHTTP etc, b/c we are done
			// return 200, ""
		})
	}
}
