// source file path: ./src/rest/middleware/adapters.go
package mw

import (
	"github.com/gorilla/mux"
	vutils "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	common "github.com/oresoftware/chat.webrtc/src/rest/middleware/dupecheck"
	"net/http"
	"sync"
)

func MakeRunParallel(c *ctx.VibeCtx) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			var wg = sync.WaitGroup{}
			wg.Add(1)
			go func() {
				next.ServeHTTP(w, r)
				wg.Done()
			}()
			wg.Wait()

		})
	}
}

type Adapter = func(http.HandlerFunc) http.HandlerFunc

func Adapt(h http.Handler, adapters ...mux.MiddlewareFunc) http.Handler {
	for _, adapter := range adapters {
		h = adapter(h)
	}
	return h
}

func AdaptFuncs(h http.HandlerFunc, adapters ...Adapter) http.HandlerFunc {

	for _, adapter := range adapters {
		h = adapter(h)
	}
	return h
}

func List(adapters ...Adapter) []Adapter {
	return adapters
}

func Middleware(adaptorsIn ...interface{}) http.HandlerFunc {

	var isWithinStack = common.DupeCheck.PotentiallyEnterNewStack()

	var adapters = vutils.FlattenDeep(adaptorsIn)

	if len(adapters) < 1 {
		panic(vutils.ErrorFromArgs("c24da1a4-0766-45fa-911f-f4bdb4bba308", "Adapter list needs to have length > 0."))
	}

	last := adapters[len(adapters)-1] // ref the last element
	h, ok := last.(http.HandlerFunc)  // cast the last element as an http.HandlerFunc

	if !ok {
		// if the last element is not an http.HandlerFunc we are SOL
		panic(vutils.ErrorFromArgs(
			"edfb1b8e-e112-45d1-8896-5e55b3589fbd",
			"the last adaptor in the middleware list must be an http.HandlerFunc"))
	}

	adapters = adapters[:len(adapters)-1] // remove the last element

	for i := len(adapters) - 1; i >= 0; i-- {

		adapt := adapters[i]

		if adapt, ok := adapt.(Adapter); ok {
			h = adapt(h) // NOTE: formerly: h = (adapt.(Adapter))(h)
			continue
		}

		if adapt, ok := adapt.(mux.MiddlewareFunc); ok {
			h = adapt(h).ServeHTTP // NOTE: formerly: h = adapt(http.Handler(h)).ServeHTTP
			continue
		}

		vbl.Stdout.InfoF("value type %T", adapt)
		panic(vutils.ErrorFromArgs(
			"d674f0a7-d7f9-4a5c-99ad-aaebd567e770",
			"could not adapt the adaptor, no bueno. this is a simple fix. ask alex."))
	}

	common.DupeCheck.PotentiallyCleanStack(isWithinStack)

	return h

}
