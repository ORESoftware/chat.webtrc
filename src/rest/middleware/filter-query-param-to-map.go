// source file path: ./src/rest/middleware/filter-query-param-to-map.go
package mw

import (
	"encoding/json"
	"github.com/gorilla/context"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	common "github.com/oresoftware/chat.webrtc/src/rest/middleware/dupecheck"
	"net/http"
)

func FilterQueryParamToMap() func(http.HandlerFunc) http.HandlerFunc {
	// //
	common.DupeCheck.CheckUuid("vid/ae677eaa7fa6")

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			// Initialize a map to hold the decoded filter
			var d map[string]interface{}

			// Extract 'filter' query param, assume it is URL-encoded and JSON stringified
			filterStr := req.URL.Query().Get("filter")
			if filterStr == "" {
				// No filter provided, use an empty map
				// context.WithValue(req.Context(), "req-body-map", d)
				context.Set(req, "req-query-param-filter-map", &d)
				next(w, req)
				return
			}

			// Decode the JSON string
			err := json.Unmarshal([]byte(filterStr), &d)
			if err != nil {
				// Log and handle errors if the JSON is invalid
				vbl.Stdout.Warn(vbl.Id("vid/72db761219ad"), err)
				return
			}

			// Log the successfully decoded map
			vbl.Stdout.Info(vbl.Id("a71de0fa-5b74-431a-b056-ee67bdf64fed"), "req-query-param-filter-map:", d)

			// Store the map in the request context and proceed
			// ctx := context.WithValue(req.Context(), "req-body-map", d)
			context.Set(req, "req-query-param-filter-map", &d)
			// next(w, req.WithContext(ctx))
			next(w, req)
		}
	}
}
