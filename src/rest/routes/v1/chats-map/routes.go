// source file path: ./src/rest/routes/v1/chats-map/routes.go
package v1_routes_chat_map

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"sync"
)

var once = sync.Once{}

func Mount(r *mux.Router, c *ctx.VibeCtx) {

	once.Do(func() {

		c.CreateReg().
			QueryStringMatch(). // S
			HeadersMatch().
			Register(func(h *ctx.Hold) *mux.Route {
				return r.Methods("POST").Path("chat/v1/add_user_to_chat").HandlerFunc(
					mw.Middleware(
						mw.AsJSON(),
						mw.LoadUser(c),
						addUserToChat(c),
					),
				)
			})

		c.CreateReg().
			QueryStringMatch(). // S
			HeadersMatch().
			Register(func(h *ctx.Hold) *mux.Route {
				return r.Methods("POST").Path("/v1/remove_user_from_chat").HandlerFunc(
					mw.Middleware(
						mw.AsJSON(),
						mw.LoadUser(c),
						mw.BodyToMap(c),
						removeUserFromChat(c),
					),
				)
			})

		r.PathPrefix("/v1/conversations?").Handler(
			http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				http.Error(w, "Most likely you are missing a query/header parameter.", http.StatusBadRequest)
				return
			}),
		)

		r.PathPrefix("/v1/conversations/").Handler(
			http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
				http.Error(w, "Most likely you are missing a query/header parameter.", http.StatusBadRequest)
				return
			}),
		)

		r.Handle("/v1/conversations", http.HandlerFunc(
			func(w http.ResponseWriter, request *http.Request) {
				http.Error(w, "Most likely you are missing a query/header parameter.", http.StatusBadRequest)
				return
			}),
		)

	})

}
