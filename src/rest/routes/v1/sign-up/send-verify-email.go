// source file path: ./src/rest/routes/v1/sign-up/send-verify-email.go
package sign_up

import (
	"bytes"
	"github.com/dgrijalva/jwt-go"
	"github.com/oresoftware/chat.webrtc/common/conf"
	"github.com/oresoftware/chat.webrtc/common/config"
	"github.com/oresoftware/chat.webrtc/v1/common"
	"github.com/oresoftware/chat.webrtc/v1/common/cptypes"
	"github.com/oresoftware/chat.webrtc/v1/model"
	htmlTemplate "html/template"
	"net/url"
	"strings"
	textTemplate "text/template"
	"time"
)

var verifyTextTmpl = textTemplate.Must(textTemplate.New("verify-account").Parse(`

Welcome to Creator Cash!

To unlock all the features in your new account, please confirm your email.
Simply click on the link below or paste it into your browser.	

{{.ResetLink}}

We're excited to have you!
The Creator Cash Team

`))

// note we split up the email alias and domain so it doesn't get a hyperlink in the email body
// to add an inline image, use the following html:
//
//	<img src="https://s.marketwatch.com/public/resources/images/MW-HT282_dollar_ZG_20191014195738.jpg">
var verifyHTMLTmpl = htmlTemplate.Must(htmlTemplate.New("verify-account-html").Parse(`
 <html>
  <head>
    <meta content="text/html; charset=UTF-8" http-equiv="Content-Type"/>
  </head>
  <body>
    <p>Hello <span>{{.EmailAlias}}@</span><span>{{.EmailDomainBeforeDot}}</span><span>{{.EmailDomainAfterDot}}</span>!</p>
    <p>Congrats on signing up with Creator Cash! To finish your account verification, please click on the link.</p>
    <p><a href="{{.ResetLink}}">Verify your account</a><br>
    <p>Verifying your account will allow you to use all the features of Creator Cash. See you soon!</p>
    <p>
  </body>
</html>
`))

func sendVerifyEmail(user model.CPUser, c *common.CPContext) *cptypes.ErrorResponseData {
	cnf := conf.GetConf()

	nowInSeconds := time.Now().Unix()
	query := url.Values{}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    user.ID,
		"email":      user.Email,
		"created_at": int(nowInSeconds),
	})

	if tokenString, err := token.SignedString([]byte(config.JWTSecretKey)); err != nil {
		return &cptypes.ErrorResponseData{
			ErrId:   "12b00eeb-3784-45c5-baa0-f6c409015886",
			Err:     err,
			Message: "Error creating token",
		}
	} else {
		query.Add("token", tokenString)
	}

	var webServerAddress = ""

	if cnf.IN_PROD {
		webServerAddress = cnf.WEB_SERVER_HOST // in prod we don't want to show a port, aka port 80
	} else {
		webServerAddress = cnf.WEB_SERVER_ADDRESS
	}

	resetLink := url.URL{
		Host:     webServerAddress,
		Scheme:   cnf.WEB_SERVER_PROTOCOL,
		Path:     "/verify_account",
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
		// create plain text string from plain text template
		var doc bytes.Buffer

		if err := verifyTextTmpl.Execute(&doc, tmplData); err != nil {
			return &cptypes.ErrorResponseData{
				ErrId:   "40e1899e-5809-4cf6-aa43-a8506cbab400",
				Err:     err,
				Message: "Error creating plain text template",
			}
		}

		templates.PlainTextStr = doc.String()
	}

	{
		// create html text string from html template
		var doc bytes.Buffer

		if err := verifyHTMLTmpl.Execute(&doc, tmplData); err != nil {
			return &cptypes.ErrorResponseData{
				ErrId:   "48ed56d6-a347-4da0-b33a-e04bc94f32b6",
				Err:     err,
				Message: "Error creating HTML template",
			}
		}

		templates.HTMLStr = doc.String()
	}

	m := c.MailGun.NewMessage(
		"hello@creator.cash",
		"Please Verify Your Creator Cash Account",
		templates.PlainTextStr,
		user.Email,
	)

	// this call will tell email client to display html if html display is possible
	m.SetHtml(templates.HTMLStr)

	// strangely the inline image doesn't work, it adds it as an attachment
	// but using the <img> tag with html does work
	// m.AddInline("v1/images/emails/creator-pay-logo.jpeg")

	if _, _, err := c.MailGun.Send(m); err != nil {
		return &cptypes.ErrorResponseData{
			ErrId:   "1d0f7778-a343-444f-a6ce-e2c91bde853c",
			Err:     err,
			Message: "Mailgun Error: " + err.Error(),
		}
	}

	return nil
}
