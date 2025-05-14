// source file path: ./src/rest/controller/controllers.go
package controller

import (
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	v1_routes_chat_map "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/chats-map"
	v1_routes_conversations "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/conversations"
	v1_routes_health "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/health"
	v1_routes_kafka "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/kafka"
	v1_routes_messages "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/messages"
	v1_routes_meta "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/meta"
	v1_routes_sequences "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/sequences"
	v1_routes_users "github.com/oresoftware/chat.webrtc/src/rest/routes/v1/users"
)

func RegisterRoutes(r *mux.Router, c *ctx.VibeCtx) {

	v1_routes_messages.Mount(r, c)
	v1_routes_conversations.Mount(r, c)
	v1_routes_users.Mount(r, c)
	v1_routes_health.Mount(r, c)
	v1_routes_meta.Mount(r, c)
	v1_routes_chat_map.Mount(r, c)
	v1_routes_kafka.Mount(r, c)
	v1_routes_sequences.Mount(r, c)

}
