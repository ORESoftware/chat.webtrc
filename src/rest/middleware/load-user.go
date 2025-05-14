// source file path: ./src/rest/middleware/load-user.go
package mw

import (
	"context"
	"encoding/json"
	"fmt"
	muxctx "github.com/gorilla/context"
	vjwt "github.com/oresoftware/chat.webrtc/src/common/jwt"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	common "github.com/oresoftware/chat.webrtc/src/rest/middleware/dupecheck"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"strings"
)

// /// help
func LoadUser(ctx *ctx.VibeCtx, permissions ...string) Adapter {

	common.DupeCheck.CheckUuid("cm:0effc9f3-9450-4fef-83bd-7fec73182231")

	var permissionsMap = make(map[string]bool)

	for _, p := range permissions {
		// create a nice lookup map for use for lifetime of process
		// reading from this map concurrently should be ok
		permissionsMap[strings.ToUpper(p)] = true
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

			v := req.Header.Get("x-vibe-user-id")
			vbl.Stdout.Warn(vbl.Id("vid/39e6955f1e0e"), "user-id-raw 1", v)
			val, err := primitive.ObjectIDFromHex(v)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/e9474eacf9a6"), err)
				res.WriteHeader(500)
				res.Write([]byte(fmt.Sprintf("Error:'%s'", err.Error())))
				return
			}

			if val.IsZero() {
				vbl.Stdout.Warn(vbl.Id("vid/d14bff451b41"), err)
				res.WriteHeader(500)
				res.Write([]byte(fmt.Sprintf("Error:'%s'", err.Error())))
				return
			}

			var u = vibe_types.LoggedInUser{
				UserObjId: val,
				UserId:    val.Hex(),
			}

			muxctx.Set(req, "logged-in-user", &u)
			next(res, req)

		})

	}
}

type ResponseBody struct {
	Status  int
	Message interface{}
	Vid     string
}

func earlyResponse(status int, vid string, err interface{}, res http.ResponseWriter, req *http.Request) {
	vbl.Stdout.Warn(vbl.Id(vid), err)
	res.WriteHeader(status)
	if v, err := json.Marshal(ResponseBody{
		Status:  status,
		Message: err,
		Vid:     vid,
	}); err != nil {
		// happy path - to send standard response
		res.Write([]byte(v))
	} else {
		vbl.Stdout.Error("vid/cd2c10b3ea1c", err)
		res.Write([]byte(`unexpected server error`))
	}
}

func LoadUserFromJWT(ctx *ctx.VibeCtx, permissions ...string) Adapter {

	common.DupeCheck.CheckUuid("cm:0effc9f3-9450-4fef-83bd-7fec73182231")

	var permissionsMap = make(map[string]bool)

	for _, p := range permissions {
		// create a nice lookup map for use for lifetime of process
		// reading from this map concurrently should be ok
		permissionsMap[strings.ToUpper(p)] = true
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {

			userId := req.Header.Get("x-vibe-user-id")
			deviceId := req.Header.Get("x-vibe-device-id")
			jwtToken := req.Header.Get("x-vibe-jwt-token")

			claims, err := vjwt.ReadJWT(jwtToken)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/e9474eacf9a6"), err)
				res.WriteHeader(401)
				res.Write([]byte(fmt.Sprintf("Error:'%s'", err.Error())))
				earlyResponse(401, "vid/e9474eacf9a6", err, res, req)
				return
			}

			if claims.UserID != userId {
				vbl.Stdout.Warn(vbl.Id("vid/e9474eacf9a6"), err)
				res.WriteHeader(500)
				res.Write([]byte(fmt.Sprintf("vid/e9474eacf9a6: Mismatched user-ids:'%s'", userId)))
				return
			}

			if claims.DeviceID != deviceId {
				vbl.Stdout.Warn(vbl.Id("vid/391591fda720"), err)
				res.WriteHeader(500)
				res.Write([]byte(fmt.Sprintf("vid/391591fda720: Mismatched device-ids:'%s'", userId)))
				return
			}

			val, err := primitive.ObjectIDFromHex(claims.UserID)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/e9474eacf9a6"), err)
				res.WriteHeader(500)
				res.Write([]byte(fmt.Sprintf("vid/e9474eacf9a6: Error:'%s'", err.Error())))
				return
			}

			if val.IsZero() {
				vbl.Stdout.Warn(vbl.Id("vid/d14bff451b41"), "zero object id")
				res.WriteHeader(500)
				res.Write([]byte(fmt.Sprintf("vid/d14bff451b41: Error - zero object id.")))
				return
			}

			if _, err := func() (bool, error) {

				bgCtx := context.Background()
				key := fmt.Sprintf("token:data:%s:%s", userId, deviceId)
				client, _ := ctx.IO.RedisConnPool.GetConnection()
				result, err := client.HGetAll(bgCtx, key).Result()
				if err != nil {
					vbl.Stdout.Error("vid/9fca07da2839", err)
					return false, err
				}

				// Check if the session is valid based on result content
				isSessionValid := len(result) > 0

				if !isSessionValid {
					return false, fmt.Errorf("vid/9c5474605d3b: redis session was invalid (does not exist)")
				}

				if strings.ToUpper(result["SessionState"]) == "INVALID" {
					return false, fmt.Errorf("vid/d471067a649c: redis session was invalid (SessionState --> Invalid)")
				}

				return true, nil

			}(); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/00ae1c1d0e70"), "token in redis was invalid:", err)
				res.WriteHeader(401)
				if _, err := res.Write([]byte(`{"Status":"Not OK"}`)); err != nil {
					vbl.Stdout.Error(vbl.Id("vid/24e270abaa1c"), err)
				}
				return
			}

			var u = vibe_types.LoggedInUser{
				UserObjId: val,
				UserId:    val.Hex(),
			}

			newToken, err := vjwt.CreateToken(userId, deviceId)

			if err != nil {
				vbl.Stdout.Warn("vid/deff98e24fa1", "could not create new token.")
			} else {
				res.Header().Set("x-vibe-new-jwt", newToken)
			}

			muxctx.Set(req, "logged-in-user", &u)
			next(res, req)

		})

	}
}
