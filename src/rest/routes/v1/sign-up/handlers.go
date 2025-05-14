// source file path: ./src/rest/routes/v1/sign-up/handlers.go
package sign_up

import (
	"encoding/json"
	"errors"

	"github.com/oresoftware/chat.webrtc/v1/common/cmpwd"
	"github.com/oresoftware/chat.webrtc/v1/common/utils"
	v1types "github.com/oresoftware/chat.webrtc/v1/types"
	"net/http"
	"regexp"
	"strings"

	"github.com/lib/pq"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"github.com/oresoftware/chat.webrtc/v1/common"
	"github.com/oresoftware/chat.webrtc/v1/common/cptypes"
	"github.com/oresoftware/chat.webrtc/v1/middleware/mw"
	"github.com/oresoftware/chat.webrtc/v1/model"
	"github.com/volatiletech/sqlboiler/v4/boil"
	"golang.org/x/crypto/bcrypt"
)

var insertErrorRgx = regexp.MustCompile(`duplicate key value violates unique constraint "cp_users_email_key"`)

func (ctr *Controller) signUp(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(w http.ResponseWriter, req *http.Request) (int, []byte) {

			var newUserInfo cptypes.SignUpInfo

			if err := json.NewDecoder(req.Body).Decode(&newUserInfo); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:   "f0befe5a-5aae-47d1-9326-0696b9c1f626",
					Err:     err,
					Message: "Could not decode request body.",
				})
			}

			if newUserInfo.Email == "" {
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					ErrId:   "7fe0f842-00e6-4edf-9f25-52e93b697116",
					Message: "Email was empty.",
				})
			}

			if err, errId := cmpwd.CheckIfPasswordLegit(newUserInfo.Email, newUserInfo.Password); err != nil {
				// check if password is valid
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					ErrId: errId,
					Err:   err,
				})
			}

			// we do not want to coerce email to lowercase!!
			newUserInfo.Email = strings.TrimSpace(newUserInfo.Email) // we do not want to coerce email to lowercase!!

			// check to see if their email matches an expected format
			if b, err := utils.ValidateEmailWithMessage(newUserInfo.Email); !b {

				log.Warningf(
					"cm:b12e0ec8-622c-402a-ae33-b464ee1252de: The user attempted to sign up with an email address with an unrecognized format: '%s'",
					newUserInfo.Email,
				)

				// perhaps we don't want to risk losing customers, but for now return an error
				return ctr.WriteJSONErrorResponse(422, cptypes.ErrorResponseData{
					ErrId:   "88dfe06d-f842-4116-a9ae-0d2daa7de9d1",
					Err:     errors.New(err),
					Reason:  err,
					Message: "Email address format is not valid according to regex.",
				})

			}

			newUser, err := common.CreateUser(newUserInfo.Email, newUserInfo.Name, newUserInfo.RefferedBy, newUserInfo.SignUpSource, newUserInfo.ReferrerSource)

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:   "b215161d-4d37-4d4c-b947-ccb3bbba7434",
					Err:     err,
					Message: "Could not create user.",
				})
			}

			var dbTransactionFinish = false
			tx, err := c.Database.Db.Begin()

			defer func() {
				if dbTransactionFinish {
					return
				}
				if err := tx.Rollback(); err != nil {
					log.Warningf("41acc27f-b9ed-4c5a-a3ac-6e99542951af:", err)
				}
			}()

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:   "18262504-865f-4dd9-bfe3-e4a39951a543",
					Err:     err,
					Message: "Could not begin transaction.",
				})
			}

			if err := newUser.Insert(c.CtxBg, tx, boil.Infer()); err != nil {

				if pgerr, ok := err.(*pq.Error); ok {
					if pgerr.Code == "23505" {
						return ctr.WriteJSONErrorResponse(409, cptypes.ErrorResponseData{
							ErrId:  "f281a6de-a8b9-44c8-93bd-ffe0388a0c3e",
							Err:    err,
							Reason: "User already exists.",
						})
					}
				}

				if insertErrorRgx.MatchString(err.Error()) {
					return ctr.WriteJSONErrorResponse(409, cptypes.ErrorResponseData{
						ErrId:  "1426c041-05a5-4b45-806b-4fde4445f039",
						Err:    err,
						Reason: "User already exists.",
					})
				}

				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:   "5b7ddd7e-cef8-41aa-9572-d69381429761",
					Err:     err,
					Message: "Failed to insert user.",
				})
			}

			var newUserAuth model.CPUsersAuth
			newUserAuth.UserID = newUser.ID
			newUserAuth.AuthMethod = "password"

			var pwdStruct cptypes.PasswordValue
			if newpass, err := bcrypt.GenerateFromPassword([]byte(newUserInfo.Password), 12); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:   "54e46865-0d49-4bc4-9361-41ae28d6e4bb",
					Err:     err,
					Message: "Failed to generate encoding",
				})
			} else {
				pwdStruct.EncodedPassword = newpass
			}

			if err := newUserAuth.AuthValue.Marshal(pwdStruct); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "a2580ba8-4677-4000-9162-004e05b6d337",
					Err:    err,
					Reason: "Failed to marshal password",
				})
			}

			if err := newUserAuth.Insert(c.CtxBg, tx, boil.Infer()); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "12447414-8079-4a23-a0a2-484931beb196",
					Err:    err,
					Reason: "Failed to insert user auth: " + err.Error(),
				})
			}

			var emailModel = model.CPUserEmail{
				UserID: newUser.ID,
				Email:  newUser.Email,
			}

			if err := emailModel.Insert(c.CtxBg, tx, boil.Infer()); err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "00373fff-9a7e-4b82-a6c4-557fc354ec51",
					Err:    err,
					Reason: "Failed to insert user auth: " + err.Error(),
				})
			}

			if err := tx.Commit(); err != nil {
				if err := tx.Rollback(); err != nil {
					log.Error("cm:aae4195a-e725-4347-8b31-c60e2492a3c7:", err)
				}
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId:  "0425a9f8-0d5a-4b90-ad33-2c08c54f93d2",
					Err:    err,
					Reason: "Could not commit transactions.",
				})
			}

			dbTransactionFinish = true

			// TODO- Sending an email verifaction is not part of the initual release.

			go func() {
				// we can send email asynchronously so the user does not have to wait
				if err := sendWelcomeEmail(newUser, c); err != nil {
					log.Warning("69906611-f892-4f83-aa9c-d76aa826c64b:", err)
				}
			}()

			// we need to respond with a 202 not a 204, responding w/ the user ID is a good idea,
			// since it's new info for the front-end in the chance the front-end wants it

			// go func() {
			// 	// we can send email asynchronously so the user does not have to wait
			// 	if err := sendVerifyEmail(newUser, c); err != nil {
			// 		log.Error("8e037e4b-36ef-4d48-958f-e95458a5e1c1:", err)
			// 	}
			// }()

			return ctr.GenericMarshal(202, v1types.SignupRespData{
				NewUserId: newUser.ID.String(),
			})
		}))

}

func (ctr *Controller) resendVerification(c *common.CPContext) http.HandlerFunc {
	return mw.Middleware(
		ctr.DoResponse(func(w http.ResponseWriter, req *http.Request) (int, []byte) {

			mwUser := c.MustGetLoggedInUser(req)
			user, err := c.GetUserByID(mwUser.Model.ID)

			if err != nil {
				return ctr.WriteJSONErrorResponse(500, cptypes.ErrorResponseData{
					ErrId: "d1435ac6-df05-45f3-b7c7-471d594d5ea6",
					Err:   err,
				})
			}

			if err := sendVerifyEmail(*user, c); err != nil {
				return ctr.WriteJSONErrorResponse(500, *err)
			}

			return 200, nil

		}))
}
