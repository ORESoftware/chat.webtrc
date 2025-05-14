// source file path: ./src/rest/middleware/mw-auth/auth.go
package mw_auth

import (
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/gorilla/context"
	config "github.com/oresoftware/chat.webrtc/src/common/auth/rsa"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	v_const "github.com/oresoftware/chat.webrtc/src/constants/v-constants"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	common "github.com/oresoftware/chat.webrtc/src/rest/middleware/dupecheck"
	"net/http"
	"strings"
	"time"
)

type ErrorResponseData struct {
	ErrId          string `json:"error_id"`
	Err            error  `json:"-"`
	PrivateMessage string `json:"-"`
	Message        string `json:"message"`
	Reason         string `json:"reason"`
	StatusCode     int    `json:"status_code"`
}

func GetBytes(code int, s ErrorResponseData) (int, []byte) {

	v, err := json.Marshal(s)

	if err != nil {
		vbl.Stdout.Warn("e775f0b1-8dc5-4e71-b75f-4cbd8b79876f:", err)
		v = []byte(`{"success": false, "reason": "could not marshal intended original payload"}`)
		code = 500
	}

	if s.StatusCode == 0 {
		s.StatusCode = code
	}

	vbl.Stdout.ErrorF("cm:ce18b77c-520c-486b-8d56-0f1762e25179: auth error: %v", s)
	return code, v
}

func RequirePermissions(c *ctx.VibeCtx, permissions ...string) mw.Adapter {

	var permissionsMap = make(map[string]bool)

	for _, p := range permissions {
		// create a nice lookup map for use for lifetime of process
		// reading from this map concurrently should be ok
		permissionsMap[strings.ToUpper(p)] = true
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return mw.DoResponse(func(ctx *mw.Ctx, res http.ResponseWriter, req *http.Request) (int, []byte) {

			mwUser := c.MustGetLoggedInUser(req)

			if mwUser == nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "e66b5a01-19be-4ff4-bfe5-76e997eadefd",
					Reason: fmt.Sprintf("no logged in user in middleware that requires a logged in user"),
				})
			}

			var userId = mwUser.UserId
			var db = c.Database.Db

			roles, err := model.CPUserRoles(qm.Where("user_id = ?", userId)).All(c.CtxBg, db)

			if err != nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "3d25a007-4b6a-4998-b23f-a555550e3134",
					Err:    err,
					Reason: "Could not read from the user_roles table.",
				})
			}

			for _, r := range roles {
				if !permissionsMap[strings.ToUpper(r.RoleName)] {
					// the user is missing the required role!
					return GetBytes(http.StatusForbidden, ErrorResponseData{
						ErrId:  "a8f489a0-ce92-494b-91d4-1994cc61bff4",
						Reason: fmt.Sprintf("user is missing the required role: '%s'", r.RoleName),
					})
				}
			}

			next(res, req)
			// negative response code means we don't write the response here
			return -1, nil
		})
	}
}

func checkRoles(roles *model.CPUserRoleSlice, permissionsMap map[string]bool) (string, error) {

	if roles == nil {
		return "", fmt.Errorf("0d43fd43-4c8c-4c76-b720-2b573299da78: roles slice was nil")
	}

	var rolesMap = map[string]bool{}

	for _, r := range *roles {
		// create a nice lookup map for use for lifetime of process
		// reading from this map concurrently should be ok
		rolesMap[strings.ToUpper(r.RoleName)] = true
	}

	for k, _ := range permissionsMap {
		if !rolesMap[strings.ToUpper(k)] {
			// the user is missing the required role!
			return k, fmt.Errorf("user is missing the required role: '%s'", k)
		}
	}

	return "", nil
}

func AdminAuth(c *ctx.VibeCtx) mw.Adapter {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return mw.DoResponse(func(ctx *mw.Ctx, res http.ResponseWriter, req *http.Request) (int, []byte) {
			// /
			access_token := req.Header.Get("Authorization")
			access_token = strings.Replace(access_token, "Bearer ", "", 1)
			token, err := cmJWT.Parse(access_token, func(token *cmJWT.Token) (interface{}, error) {
				if _, ok := token.Method.(*cmJWT.SigningMethodRSA); !ok {
					return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
				}
				return config.JwtRSAKeyPub, nil
			})

			if err != nil || !token.Valid {
				return GetBytes(http.StatusUnprocessableEntity, ErrorResponseData{
					ErrId:  "380e3b92-c3e4-4db4-8b35-be38cc210e4e",
					Reason: fmt.Sprintf("Invalid token"),
				})
			}

			if userIDStr, ok := token.Claims["user_id"].(string); ok {
				userID, err := uuid.Parse(userIDStr)

				if err != nil {
					return GetBytes(http.StatusUnprocessableEntity, ErrorResponseData{
						ErrId:  "50f01484-a2a0-449e-a63b-d6fbc224a9df",
						Reason: fmt.Sprintf("Invalid token values"),
					})
				}

				adminUser, err := model.FindCMAdmin(c.CtxBg, c.Database.Db, userID)
				if err != nil {
					return GetBytes(http.StatusNotFound, ErrorResponseData{
						ErrId:  "a6d98448-f959-446a-b7fe-5b1202f223fb",
						Reason: fmt.Sprintf("No admin user found"),
					})
				}

				if adminUser.Authentication.String != token.Claims["authenication"].(string) {
					return GetBytes(http.StatusUnauthorized, ErrorResponseData{
						ErrId:  "db8d5f92-2e44-47b7-9a89-209a6584d739",
						Reason: fmt.Sprintf("Invalid authentication"),
					})
				}
			} else {
				return GetBytes(http.StatusNotFound, ErrorResponseData{
					ErrId:  "a99d5d1d-000f-4844-854e-0b5ef354c635",
					Reason: fmt.Sprintf("No user id provided"),
				})
			}

			next(res, req)
			return -1, nil // negative resp code means response isn't sent by this middleware
		})
	}
}

func LoadUser(c *ctx.VibeCtx, permissions ...string) mw.Adapter {

	common.DupeCheck.CheckUuid("cm:31055708-3939-40e5-a2d5-35348a31464b")

	var permissionsMap = make(map[string]bool)

	for _, p := range permissions {
		// create a nice lookup map for use for lifetime of process
		// reading from this map concurrently should be ok
		permissionsMap[strings.ToUpper(p)] = true
	}

	return func(next http.HandlerFunc) http.HandlerFunc {
		return mw.DoResponse(func(ctx *mw.Ctx, res http.ResponseWriter, req *http.Request) (int, []byte) {

			{
				// let's check if this middleware was already called by this same request
				user, err := c.GetLoggedInUser(req)

				if err == nil {

					// the logged-in user is already, loaded, so most likely
					// we already called this middleware, probably was registered twice

					// however, perhaps we have more roles to check, so let's make sure
					// we check any new required roles
					if k, err := checkRoles(user.Roles, permissionsMap); err != nil {
						return GetBytes(http.StatusForbidden, ErrorResponseData{
							ErrId:  "3926a1b2-b57a-446c-b2da-b594255d87f5",
							Err:    err,
							Reason: fmt.Sprintf("user is missing the required role: '%s'", k),
						})
					}

					next(res, req)
					return -1, nil // negative resp code means response isn't sent by this middleware
				}
			}

			if req.Header.Get("cp-auth-token") == "" {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "48e04623-8aaf-4854-8dfb-a358618d5b7b",
					Reason: fmt.Sprintf("missing token header in request to: '%s'", req.URL.Path),
				})
			}

			var u = v1types.LoggedInUser{}

			token, err := jwt.Parse(req.Header.Get("cp-auth-token"), func(token *jwt.Token) (interface{}, error) {

				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				// hmacSampleSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
				return []byte(config.JWTSecretKey), nil
			})

			if err != nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "b5da56cc-0ad6-4e49-8dd9-682a35fcce02",
					Err:    err,
					Reason: "error parsing token:" + err.Error(),
				})
			}

			claims, ok := token.Claims.(jwt.MapClaims)

			if !(ok && token.Valid) {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "2228d34b-512e-47b0-895a-1a5f9b9bed2b",
					Reason: "token is invalid",
				})
			}

			//  NOTE: unsure if we want/need this
			// if err :=claims.Valid(); err != nil {
			//  return GetBytes(http.StatusForbidden, ErrorResponseData{
			//    ErrId:  "b0bfa59f-d6df-47d7-b64f-624edada46b5",
			//    Reason: "token is invalid",
			//  })
			// }

			u.JWTClaims = claims

			userId, ok := claims["user_id"].(string)

			if !ok {
				vbl.Stdout.Info("the problematic token claims were:", claims)
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "b6ba272e-ad0e-4432-9049-6c4869b27109",
					Reason: "user_id in token claims could not be cast to a string.",
				})
			}

			email, ok := claims["email"].(string)

			if !ok {
				vbl.Stdout.Info("the problematic token claims were:", claims)
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "cfc490c4-cf39-4eb9-a299-41cc6a97d3ea",
					Reason: "the 'email' field in token claims could not be cast to a string.",
				})
			}

			createdAt, ok := claims["created_at"].(float64)

			if !ok {
				vbl.Stdout.Info("the problematic token claims were:", claims)
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "e80cbce4-f518-4103-a685-c6debfe30d09",
					Reason: "the 'created_at' field in token claims could not be cast to a float64.",
				})
			}

			shouldExpireAt, ok := claims["should_expire_at"].(float64)

			if !ok {
				vbl.Stdout.Info("the problematic token claims were:", claims)
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "525d8d05-c354-42e2-bf3d-c19903151168",
					Reason: "The 'should_expire_at' field in token claims could not be cast to a float64.",
				})
			}

			requestHistory, ok := claims["request_history"].([]interface{})

			if !ok {
				vbl.Stdout.Info("the problematic token claims were:", claims)
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "9388fb8d-64a0-4bb7-a2c4-7c1a2d4eb801",
					Reason: "request_history could not be cast to slice in token claims.",
				})
			}

			myLen := len(requestHistory)

			newRequestHistory := []float64{}

			for i := myLen - 1; i >= 0; i-- {

				if myLen-i > MAX_REQS {
					// we only copy the final 30 elements
					break
				}

				val, ok := requestHistory[i].(float64)

				if !ok {
					vbl.Stdout.Info("the problematic token claims were:", claims)
					return GetBytes(http.StatusForbidden, ErrorResponseData{
						ErrId:  "9c544df7-fc96-42ab-95ca-06bfdacd90b1",
						Reason: "request_history element could not be cast to float64.",
					})
				}

				newRequestHistory = append(newRequestHistory, val)
			}

			timeNow := time.Now().UnixNano() / 1000000 // time since epoch in millis

			if len(newRequestHistory) > MAX_REQS {
				// if there are fewer than MAX_REQS, then we can skip this
				// there may be fewer than MAX_REQS in two cases 1. new user or 2. old requests were removed from the list
				if timeNow-MAX_REQS_WINDOW_MILLIS < int64(newRequestHistory[0]) {
					// if the time that's transpired between the oldest request and now is smaller than the window
					// then we have had too many requests in the window!
					return GetBytes(http.StatusTooManyRequests, ErrorResponseData{
						ErrId:  "eba55487-0035-4621-b705-043611f2ed03",
						Reason: "429: Too many requests during time period.",
					})
				}
			}

			mostRecentTimestamp := float64(timeNow)
			newRequestHistory = append(newRequestHistory, mostRecentTimestamp)

			if createdAt == 0 {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "a1bbbfa8-c373-4c2d-852d-382b4ad26055",
					Reason: "missing 'created_at' value in jwt claims.",
				})
			}

			// commenting out until we can make sure front end doesn't cause problems

			// if timeNow-int64(createdAt) > 86400*5*1000 { // 5 days worth of millis-seconds
			// 	// token expires after 5 days
			// 	return GetBytes(http.StatusForbidden, ErrorResponseData{
			// 		ErrId:  "0a6d249f-cfe7-45a6-8451-08e221ebf694",
			// 		Reason: "token has expired.",
			// 	})
			// }
			// if timeNow > int64(shouldExpireAt) {
			// 	// token has expired
			// 	return GetBytes(http.StatusForbidden, ErrorResponseData{
			// 		ErrId:  "a8228103-5cd1-4d46-b45e-25a0ade0eb5b",
			// 		Reason: "token has expired - by comparing the 'should expire at' field.",
			// 	})
			// }

			if email == "" {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "0a69f97b-b273-4d70-8061-f5eb85277d15",
					Reason: "missing email in jwt claims.",
				})
			}

			if userId == "" {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "6cf76109-bae1-42cf-9472-3a111665d1a3",
					Reason: "missing user_id in jwt claims.",
				})
			}

			var db = c.Database.Db

			user, err := model.CPUsers(qm.Where("id = ?", userId)).One(c.CtxBg, db)

			if err != nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "5338d770-f0d1-4ee8-87a4-8e305a5872dd",
					Err:    err,
					Reason: "could not find user.",
				})
			}

			if *user.Disabled.Ptr() {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "a776461f-5102-4c9c-9451-2826d0661ee6",
					Reason: "User is disabled",
				})
			}

			// TODO: deal with updating email
			// if user.Email != email {
			// 	return GetBytes(http.StatusForbidden, ErrorResponseData{
			// 		ErrId:  "32fb4576-e83e-43c3-9d18-c5ddf8f31e15",
			// 		Reason: "emails do not match.",
			// 	})
			// }

			if user.ID.String() != userId {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "2517d52a-91da-4851-85b7-4b602ae8856a\n",
					Reason: "user ids do not match.",
				})
			}

			roles, err := model.CPUserRoles(qm.Where("user_id = ?", userId)).All(c.CtxBg, db)

			if err != nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "d8d5e9bb-49e7-4ddd-adae-4dd9047f18a7",
					Err:    err,
					Reason: "could not retrieve user roles.",
				})
			}

			u.Roles = &roles

			if k, err := checkRoles(&roles, permissionsMap); err != nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "4d778648-ce8f-4448-92f6-44304ed546f2",
					Err:    err,
					Reason: fmt.Sprintf("user is missing the required role: '%s'", k),
				})
			}

			nowInMillis := time.Now().UnixNano() / 1000000 // time since epoch in millis

			// expire it 1 hour later than now, so we avoid user getting bounced out of app while using it
			var newShouldExpireAt int64

			if shouldExpireAt-float64(nowInMillis) < 1000*60*60 {
				// extend the expiration by 60 minutes
				newShouldExpireAt = nowInMillis + 1000*60*60
			} else {
				// keep the original expiration date
				newShouldExpireAt = int64(shouldExpireAt)
			}

			newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id":           user.ID,
				"email":             user.Email,
				"created_at":        float64(nowInMillis), // since we are creating a new token
				"latest_request_at": mostRecentTimestamp,
				"request_history":   newRequestHistory,
				// ⤵️ ️expiration time should always be at least an hour from now, if this is reached ⤵️
				"should_expire_at": float64(newShouldExpireAt),
			})

			// Sign and get the complete encoded token as a string using the secret
			tokenStr, err := newToken.SignedString([]byte(v_const.JWTSecretKey))

			if err != nil {
				return GetBytes(http.StatusForbidden, ErrorResponseData{
					ErrId:  "75cd12c2-96d6-4435-bf5a-20169b79e6da",
					Reason: "Could not create new JWT from existing one.",
				})
			}

			res.Header().Set("cp-auth-token", tokenStr)

			u.Model = user
			context.Set(req, "logged-in-user", &u)
			next(res, req)
			return -1, nil // negative resp code means response isn't sent by this middleware
		})
	}
}
