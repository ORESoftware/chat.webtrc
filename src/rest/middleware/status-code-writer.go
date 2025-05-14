// source file path: ./src/rest/middleware/status-code-writer.go
package mw

import (
	"github.com/gorilla/mux"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	"net/http"
)

type LoggingResponseWriter struct {
	http.ResponseWriter
	ErrResponseBody string
	StatusCode      int
}

func (lrw *LoggingResponseWriter) WriteHeader(code int) {
	lrw.StatusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func NewLoggingResponseWriter(w http.ResponseWriter) *LoggingResponseWriter {
	// WriteHeader(int) is not called if our response implicitly returns 200 OK, so
	// we default to that status code.
	return &LoggingResponseWriter{w, "", http.StatusOK}
}

func AddStatusCodeWriter(c *ctx.VibeCtx) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// we wrap the response write in a struct to add metadata
			// for more info see: https://gist.github.com/Boerworz/b683e46ae0761056a636
			lrw := NewLoggingResponseWriter(w)
			next.ServeHTTP(lrw, r)
		})
	}
}
