// source file path: ./src/rest/routes/v1/login/handlers.go
package login

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/oresoftware/chat.webrtc/v1/common/cmpwd"

	v1types "github.com/oresoftware/chat.webrtc/v1/types"

	"github.com/oresoftware/chat.webrtc/src/rest/middleware"

	"github.com/dgrijalva/jwt-go"
	"github.com/golang-jwt/jwt"
	"github.com/google/uuid"
	"github.com/oresoftware/chat.webrtc/common/conf"
	"github.com/oresoftware/chat.webrtc/common/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	"github.com/oresoftware/chat.webrtc/v1/common"
	"github.com/oresoftware/chat.webrtc/v1/common/cptypes"
	"github.com/oresoftware/chat.webrtc/v1/model"
	"github.com/volatiletech/null/v8"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"github.com/volatiletech/sqlboiler/v4/queries/qm"
	"golang.org/x/crypto/bcrypt"

	v_const "github.com/oresoftware/chat.webrtc/src/constants/v-constants"
)

func (ctr *Controller) logoutHandler(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(ctx *mw.Ctx, res http.ResponseWriter, req *http.Request) (int, []byte) {
			return 501, []byte("not implemented")
		}))
}

func (ctr *Controller) loginHandler(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(ctx *mw.Ctx, res http.ResponseWriter, req *http.Request) (int, []byte) {

			// destructuring request info
			var loginInfo cptypes.LoginInfo

			if err := json.NewDecoder(req.Body).Decode(&loginInfo); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "ae88caef-a904-4aae-b4ef-db01a72240c0",
					Err:   err,
				})
			}

			if loginInfo.Email == "" || loginInfo.Password == "" {
				return ctr.GenericMarshal(401, v1types.LoginRespData{
					Message: "Invalid/missing email/password.",
				})
			}

			user, err := c.GetUserByEmail(strings.ToLower(loginInfo.Email))
			if err != nil {
				return ctr.GenericMarshal(401, v1types.LoginRespData{
					Message: "Could not find user.",
				})
			}

			if user.LoginAttempts > config.MAX_LOGIN_ATTEMPTS_BEFORE_FORCE_PWD_RESET {
				return ctr.GenericMarshal(403, v1types.LoginRespData{
					Message:          "Too many login attempts. Must reset password.",
					Email:            user.Email,
					LoginAttemptUser: *user,
				})
			}

			var ctxBg = context.Background()
			var db = c.Database.Db

			var AuthMethodCol = model.CPUsersAuthColumns.AuthMethod
			var UserIdCol = model.CPUsersAuthColumns.UserID

			// get user auths
			userAuths, err := user.UserCPUsersAuths(
				qm.Where(UserIdCol+" = ?", user.ID),
				qm.Where(AuthMethodCol+" = ?", "password"),
			).All(ctx, db)

			if err != nil {
				// TODO different error here?
				log.Error("287d467a-9b0d-466c-b577-434db4725ae0:", err)
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "3f02d1fd-c5f2-44d2-bcc4-546345d5b32c",
					Err:   err,
				})
			}

			var pwdStruct cptypes.PasswordValue
			var passwordFieldCount = 0 // count how many records have "password" as the AuthMethod

			for _, auth := range userAuths {

				if auth.AuthMethod != "password" {
					continue
				}

				passwordFieldCount++

				if err := auth.AuthValue.Unmarshal(&pwdStruct); err != nil {
					return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
						ErrId: "d3405aa2-e9f9-49c5-ac64-6ac05aafa157",
						Err:   err,
					})
				}

			}

			if passwordFieldCount > 1 {
				log.Warning("14a33cd1-3eb0-428e-a05b-a0384ec121c3:", "Too many password records, there should only be one.")
			}

			if passwordFieldCount < 1 {
				log.Warning("4dd7c63f-3874-463a-b088-c8da6c48fc89:", "No password record could be found.")
			}

			if pwdStruct.EncodedPassword == nil {
				// when the user resets password, we set it to "null", so it could be nil here
				log.Error("6c610bfb-9396-4226-b049-60be3d3465c0: No password record could be found.")
				return ctr.WriteJSONErrorResponse(410, cptypes.ErrorResponseData{
					ErrId:  "1b2903df-7fa5-4320-af32-3c67b46bc998",
					Reason: "Requires reset.",
				})
			}

			if len(pwdStruct.EncodedPassword) == 0 {
				return ctr.WriteJSONErrorResponse(410, cptypes.ErrorResponseData{
					ErrId:  "559efdea-546d-4076-a8eb-fcc5a34d76e6",
					Reason: "Requires reset.",
				})
			}

			if conf.GetConf().ALLOW_LOGIN_AS_ANYONE == false {
				// if the --unsafe-login-as flag is passed, then we don't check the password
				// but if we are in prod, we always check the password!
				if !common.CheckPassword(pwdStruct.EncodedPassword, []byte(loginInfo.Password)) {
					// update password failures
					user.LoginAttempts = user.LoginAttempts + 1
					_, err := user.Update(ctx, db, boil.Infer())
					if err != nil {
						log.Warningf("could not save/update user model: %v", err)
					}
					return ctr.WriteJSONErrorResponse(401, cptypes.ErrorResponseData{
						ErrId:  "f60dcead-a5ce-45f2-9b6d-1867c4f74684",
						Reason: "Invalid email/password.",
					})
				}
			} else {
				log.Warningf("cm:0200be0e-85ec-4bc7-9085-ebb10fae654f: anyone can log in with any password")
			}

			if user.Disabled.Bool {
				return ctr.WriteJSONErrorResponse(401, cptypes.ErrorResponseData{
					ErrId:  "bf418aa8-6e8a-4d7c-add6-047c3208b18e",
					Reason: "User is disabled",
				})
			}

			nowInMillis := time.Now().UnixNano() / 1000000            // time since epoch in millis
			shouldExpireInMillis := nowInMillis + int64(84000*1000*5) // 5 days from now

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id":          user.ID,
				"email":            user.Email,
				"request_history":  []float64{float64(time.Now().Unix())},
				"created_at":       float64(nowInMillis),
				"should_expire_at": float64(shouldExpireInMillis), // should expire in 5 days
			})

			// Sign and get the complete encoded token as a string using the secret
			tokenStr, err := token.SignedString([]byte(v_const.JWTSecretKey))

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "69e501a3-4b74-4900-a898-cceda7321655",
					Err:   nil,
				})
			}

			if false {
				{
					cookie := http.Cookie{
						Name:     "cp-auth-token",
						Value:    tokenStr,
						Expires:  time.Now().Add(time.Second * 84000 * 3),
						Domain:   "http://localhost",
						Secure:   false,
						HttpOnly: false,
						SameSite: 0,
					}

					http.SetCookie(res, &cookie)
				}

				{
					cookie := http.Cookie{
						Name:     "Authorization",
						Value:    "Bearer " + tokenStr,
						Expires:  time.Now().Add(time.Second * 84000 * 3),
						Domain:   "http://localhost",
						Secure:   false,
						HttpOnly: false,
						SameSite: 0,
					}

					http.SetCookie(res, &cookie)
				}
			}

			// user.PasswordFailures = 0
			// user.SetPasswordFailures(c)

			res.Header().Set("cp-auth-token", tokenStr)

			nextState := getNextState(user.ID, c)

			respPayload := v1types.LoginRespData{
				Message:          "Login success",
				Email:            user.Email,
				Name:             user.Name,
				ID:               user.ID,
				APIKey:           user.APIKey,
				CPAuthToken:      tokenStr,
				LoginAttemptUser: *user,
				NextState:        nextState,
			}

			user.LoginAttempts = 0
			_, err = user.Update(ctx, db, boil.Infer())
			if err != nil {
				log.Warningf("could not save/update user model: %v", err)
			}

			return ctr.GenericMarshal(200, respPayload)

		}))
}

var forgotPasswordTextTmpl = template.Must(template.New("forgot-password").Parse(`

Hello {{.Email}},

We received a request to reset your password.  If you made this request, click here to choose a new password:

{{.ResetLink}}

If you didn't mean to reset your password, you can ignore this email.  Your password will not change.

- The Creator Cash Team

`))

// to add an inline image, use html like so:
//
//	<img src="https://s.marketwatch.com/public/resources/images/MW-HT282_dollar_ZG_20191014195738.jpg">
var forgotPasswordHtmlTempl = template.Must(template.New("forgot-password-html").Parse(`
<html>
  <head>
    <meta content="text/html; charset=UTF-8" http-equiv="Content-Type"/>
  </head>
  <body>
    <p>Hello <span>{{.EmailAlias}}@</span><span>{{.EmailDomainBeforeDot}}</span><span>{{.EmailDomainAfterDot}}</span>!</p>
    <p>Someone has requested a link to change your password. You can do this through the link below.</p>
    <p><a href="{{.ResetLink}}">Change Your Password</a><br>
    <p>If you didn't request this, please ignore this email.</p>
    <p>Your password won't change until you access the link above and create a new one.</p>
  </body>
</html>
`))

func (ctr *Controller) forgotPasswordHandler(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(ctx *mw.Ctx, res http.ResponseWriter, req *http.Request) (int, []byte) {

			var cnf = conf.GetConf()
			var args struct {
				Email string `json:"email"`
			}
			decoder := json.NewDecoder(req.Body)
			if err := decoder.Decode(&args); err != nil {
				if req.FormValue("email") != "" {
					args.Email = req.FormValue("email")
				} else {
					return ctr.WriteJSONErrorResponse(404, cptypes.ErrorResponseData{
						ErrId:  "64c29b76-17af-49eb-8fda-74ac9f8a4d9b",
						Reason: "An email must be provided.",
					})
				}
			}

			user, err := c.GetUserByEmail(args.Email)

			if err == sql.ErrNoRows {
				// we may not wish to reveal if an email exists or not
				// formerly 204, but we need a way to tell the front-end there was an error
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "645a5861-c025-454b-8bd4-3423fcfc8c22",
					Err:    err,
					Reason: "Email does not exist",
				})
			}

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "5d1df09b-fe3b-4cac-8044-aa94336560d9",
					Err:   err,
				})
			}

			var db = c.Database.Db
			var AuthMethodCol = model.CPUsersAuthColumns.AuthMethod
			var UserIdCol = model.CPUsersAuthColumns.UserID

			userAuth, err := user.UserCPUsersAuths(
				qm.Where(UserIdCol+" = ?", user.ID),
				qm.Where(AuthMethodCol+" = ?", "password"),
			).One(c.CtxBg, db)

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "cf119658-5986-4f97-b92b-6b6703062caa",
					Err:   err,
				})
			}

			var pwdStruct cptypes.PasswordValue
			if err := userAuth.AuthValue.Unmarshal(&pwdStruct); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "f49406f4-9165-4657-b433-261c84e45e92",
					Err:   err,
				})
			}

			{
				// we overwrite the existing password field so as to avoid a delete
				// aka, their current password must no longer be valid
				userAuth.AuthValue = null.JSON{
					JSON:  []byte(`null`),
					Valid: true,
				}

				_, err = userAuth.Update(c.CtxBg, db, boil.Infer())

				if err != nil {
					log.Error("b687f6b2-8702-488f-b553-cb137d022b80: Could not update current password.")
				}
			}

			query := url.Values{}
			query.Add("email", user.Email)

			nowInSeconds := time.Now().Unix()
			shouldExpireInSeconds := nowInSeconds + int64(84000*3) // expire in 3 days

			token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
				"user_id":          user.ID.String(),
				"email":            user.Email,
				"created_at":       int(nowInSeconds),
				"should_expire_at": int(shouldExpireInSeconds), // should expire in 3 days
			})

			if tokenString, err := token.SignedString([]byte(config.JWTSecretKey)); err == nil {
				query.Add("token", tokenString)
			} else {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "727ba551-65d8-47db-be6a-3c092f086e7f",
					Err:   err,
				})
			}

			var webServerAddress = ""

			if cnf.IN_PROD {
				webServerAddress = cnf.WEB_SERVER_HOST // in prod we don't want to show a port, aka port 80
			} else {
				webServerAddress = cnf.WEB_SERVER_ADDRESS
			}

			//
			resetLink := url.URL{
				Scheme:   cnf.WEB_SERVER_PROTOCOL,
				Host:     webServerAddress, // api.creator.cashlocalhost:2020
				Path:     "reset",
				RawQuery: query.Encode(),
			}

			var tokens = strings.Split(user.Email, "@")
			var emailAlias, emailDomain = tokens[0], tokens[1]
			var firstDot = strings.Index(emailDomain, ".")
			var beforeFirstDot = emailDomain[0:firstDot]
			var afterFirstDot = emailDomain[firstDot:len(emailDomain)]

			tmplData := struct {
				ResetLink            string
				Email                string
				EmailAlias           string
				EmailDomainBeforeDot string
				EmailDomainAfterDot  string
			}{resetLink.String(), user.Email, emailAlias, beforeFirstDot, afterFirstDot}

			var templates = struct {
				PlainTextStr string
				HTMLStr      string
			}{}

			{
				var doc bytes.Buffer

				if err := forgotPasswordTextTmpl.Execute(&doc, tmplData); err != nil {
					return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
						ErrId: "9b53269a-51f1-4d4d-ba02-a3cd856c0637",
						Err:   err,
					})
				}

				templates.PlainTextStr = doc.String()
			}

			{
				var doc bytes.Buffer

				if err := forgotPasswordHtmlTempl.Execute(&doc, tmplData); err != nil {
					return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
						ErrId: "bb602e95-d1b6-429d-ba12-e92f70693b36",
						Err:   err,
					})
				}

				templates.HTMLStr = doc.String()
			}

			// this call will tell email client to display html if html display is possible
			m := c.MailGun.NewMessage("hello@creator.cash", "Reset Your Creator Cash Password", templates.PlainTextStr, user.Email)
			m.SetHtml(templates.HTMLStr)

			// strangely the inline image doesn't work, it adds it as an attachment
			// but using the <img> tag with html does work
			// m.AddInline("v1/images/emails/creator-pay-logo.jpeg")

			if _, _, err := c.MailGun.Send(m); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "afe72185-ad07-4d18-950e-b1cc6abf9170",
					Err:    err,
					Reason: "Mailgun error",
				})
			}

			return 200, []byte("")
		}))
}

func (ctr *Controller) resetPasswordHandler(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(ctx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			// todo are we comparing the password in the token? not sure
			var reqBodyFields struct {
				Password string `json:"password"`
				Token    string `json:"token"`
				Email    string `json:"email"` // user resupplies email from front-end
			}

			type RespData struct {
				Message        string
				PrivateMessage string
				StatusCode     int
			}

			if err := json.NewDecoder(req.Body).Decode(&reqBodyFields); err != nil {
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{Err: err})
			}

			// get password token from request
			token, err := jwt.Parse(reqBodyFields.Token, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return []byte(config.JWTSecretKey), nil
			})

			if err != nil || !token.Valid {
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{Err: err, Message: "Invalid token"})
			}

			claims := token.Claims.(jwt.MapClaims)

			createdAt, ok := claims["created_at"].(float64)

			if !ok {
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					Err:     errors.New("the 'created_at' field could not be cast to float64"),
					Message: "Invalid token",
				})
			}

			if time.Now().Unix()-int64(createdAt) > 60*60 {
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					Err:     errors.New("token expired after an hour"),
					Message: "Token Expired",
				})
			}

			userID, _ := uuid.Parse(claims["user_id"].(string))

			// get current password value
			userAuth, err := c.GetUserAuthByUserID(userID)

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					Err:     err,
					ErrId:   "0a1476a7-edce-4890-aeb0-eaee91e53e39",
					Message: "Could not find user auth info.",
				})
			}

			email, ok := claims["email"].(string)

			if !ok {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					Err:     errors.New("missing email field in token"),
					ErrId:   "57c3770f-f7cf-49ef-a53d-f0d2736d6172",
					Message: "Token invalid - missing email field",
				})
			}

			if email != reqBodyFields.Email {
				// we don't want the client to be trying any funny business, and if they try to change the email
				// then this won't match. but in any case, we only update the information that's from the original token.
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					Err:     errors.New("emails must match"),
					ErrId:   "9d6a7a37-fd93-492f-b364-4c0cb813f247",
					Message: "Invalid token",
				})
			}

			var db = c.Database.Db

			var pwdStruct cptypes.PasswordValue
			if err := userAuth.AuthValue.Unmarshal(&pwdStruct); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "c15b6241-da6e-4940-b5e7-a6b7ddb61191",
					Err:    err,
					Reason: "failed to unmarshal",
				})
			}

			// validate the password!!
			if err, errId := cmpwd.CheckIfPasswordLegit(email, reqBodyFields.Password); err != nil {
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					Err:    err,
					ErrId:  errId,
					Reason: "Password string is invalid, please use a different password",
				})
			}

			// set new password
			var newPwdStruct cptypes.PasswordValue
			if newpass, err := bcrypt.GenerateFromPassword([]byte(reqBodyFields.Password), 12); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					Err:            err,
					ErrId:          "655ea369-6592-4a41-a24e-b16ca8781c15",
					PrivateMessage: "Failed to generate password",
					Message:        "Failed to generate encrypted password",
				})
			} else {
				newPwdStruct.EncodedPassword = newpass
			}

			if err := userAuth.AuthValue.Marshal(newPwdStruct); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					Err:            err,
					ErrId:          "c30a083d-0cdb-4a06-8570-0e345857b353",
					PrivateMessage: "Failed to marshal password",
					Message:        "Failed to generate encrypted password",
				})
			}

			rowsAff, err := userAuth.Update(c.CtxBg, db, boil.Infer())

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "f4befdaf-a19b-46a1-8547-e9add8a8e812",
					Err:    err,
					Reason: "Failed to insert user auth",
				})
			}

			if rowsAff != 1 {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "7d0f44a4-efc2-4285-a903-fe09d476b126",
					Reason: "Meant to update 1 record, instead updated: " + strconv.Itoa(int(rowsAff)),
				})
			}

			user, err := c.GetUserByID(userID)

			if err != nil {
				log.Warning("could not get user model: ", err)
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "37428b16-7d66-40f2-a835-6b668b3c0652",
					Err:   err,
				})
			}

			user.LoginAttempts = 0
			result, err := user.Update(c.CtxBg, db, boil.Infer())
			if err != nil {
				log.Warning("c3d6dd61-4e63-43cf-8e0c-b7b6b7a7ce27: could not save/update user model: ", result, err)
			}

			return 200, ctr.EasyMarshal("")

		}))
}
