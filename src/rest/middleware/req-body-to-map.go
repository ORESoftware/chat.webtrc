// source file path: ./src/rest/middleware/req-body-to-map.go
package mw

import (
	"encoding/json"
	"github.com/gorilla/context"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	common "github.com/oresoftware/chat.webrtc/src/rest/middleware/dupecheck"
	"io"
	"net/http"
)

func isJSON(req *http.Request) bool {
	return req.Header.Get("Content-Type") == "application/json" ||
		req.Header.Get("content-type") == "application/json"
}

func BodyToMap(c *ctx.VibeCtx) Adapter {

	common.DupeCheck.CheckUuid("cm:68868a51-18e1-4269-9dd4-4af3f10e9815")

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {

			var d = vibe_types.BodyMap{}

			if !isJSON(req) {
				context.Set(req, "req-body-map", &d)
				next(w, req)
				return
			}

			if req.ContentLength == 0 {
				// empty body
				context.Set(req, "req-body-map", &d)
				next(w, req)
				return
			}

			if req.Body == nil {
				vbl.Stdout.Warn(vbl.Id("74b177aa-278c-4004-95b0-7dae5e630445"), "Body was nil")
				context.Set(req, "req-body-map", &d)
				next(w, req)
				return
			}

			err := json.NewDecoder(req.Body).Decode(&d)

			if err != nil {
				if err != io.EOF {
					vbl.Stdout.Error(vbl.Id("b8bc68a5-5ee4-4b3b-9e17-72db761219ad"), err)
					panic(vbu.ErrorFromArgsList("9a4d0c29-0813-426c-ac09-51ac387c9135", []string{err.Error()}))
				} else {
					vbl.Stdout.Warn(vbl.Id("fe965a68-c944-4b82-854e-a29d1adc02ff"), "request body was empty?:", err)
				}
			}

			vbl.Stdout.Info(vbl.Id("a71de0fa-5b74-431a-b056-ee67bdf64fed"), "req-body-map:", d)
			context.Set(req, "req-body-map", &d)
			next(w, req)
		}
	}
}
