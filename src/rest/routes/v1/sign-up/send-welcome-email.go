// source file path: ./src/rest/routes/v1/sign-up/send-welcome-email.go
package sign_up

import (
	"bytes"
	"github.com/oresoftware/chat.webrtc/common/utils"
	"github.com/oresoftware/chat.webrtc/v1/common"
	"github.com/oresoftware/chat.webrtc/v1/common/cptypes"
	"github.com/oresoftware/chat.webrtc/v1/model"
	htmlTemplate "html/template"
	"strings"
	textTemplate "text/template"
)

// note we split up the email alias and domain so it doesn't get a hyperlink in the email body
// to add an inline image, use the following html:
//  <img src="https://s.marketwatch.com/public/resources/images/MW-HT282_dollar_ZG_20191014195738.jpg">

var welcomeEmailTemplate = utils.MustReadTemplateFile("welcome-email-template.html")
var welcomeHTMLTmpl = htmlTemplate.Must(htmlTemplate.New("welcome-email-html").Parse(string(welcomeEmailTemplate)))
var welcomeEmailTxtTemplate = utils.MustReadTemplateFile("welcome-email-template.txt")
var welcomeTextTmpl = textTemplate.Must(textTemplate.New("welcome-template-text").Parse(string(welcomeEmailTxtTemplate)))

func sendWelcomeEmail(user model.CPUser, c *common.CPContext) *cptypes.ErrorResponseData {

	var tokens = strings.Split(user.Email, "@")
	var emailAlias, emailDomain = tokens[0], tokens[1]
	var firstDot = strings.Index(emailDomain, ".")
	var beforeFirstDot = emailDomain[0:firstDot]
	var afterFirstDot = emailDomain[firstDot:len(emailDomain)]

	tmplData := struct {
		Email                string
		EmailAlias           string
		EmailDomainBeforeDot string
		EmailDomainAfterDot  string
	}{user.Email, emailAlias, beforeFirstDot, afterFirstDot}

	var templates = struct {
		PlainTextStr string
		HTMLStr      string
	}{}

	{
		// create plain text string from plain text template
		var doc bytes.Buffer

		if err := welcomeTextTmpl.Execute(&doc, tmplData); err != nil {
			return &cptypes.ErrorResponseData{
				ErrId:   "d5f3383e-941d-476d-b94d-406e879c1eb2",
				Err:     err,
				Message: "Error creating plain text template",
			}
		}

		templates.PlainTextStr = doc.String()
	}

	{
		// create html text string from html template
		var doc bytes.Buffer

		if err := welcomeHTMLTmpl.Execute(&doc, tmplData); err != nil {
			return &cptypes.ErrorResponseData{
				ErrId:   "2777a2f5-3d30-4afc-922d-a8076f8837ad",
				Err:     err,
				Message: "Error creating HTML template",
			}
		}

		templates.HTMLStr = doc.String()
	}

	m := c.MailGun.NewMessage(
		"hello@creator.cash",
		"Welcome to Creator Cash! 💸 ", // there is a cash emoji/unicode, just might be invisible :)
		templates.PlainTextStr,
		// "alex@vibeirl.com",
		user.Email,
	)

	// this call will tell email client to display html if html display is possible
	m.SetHtml(templates.HTMLStr)

	// strangely the inline image doesn't work, it adds it as an attachment
	// but using the <img> tag with html does work
	// m.AddInline("v1/images/emails/creator-pay-logo.jpeg")

	if _, _, err := c.MailGun.Send(m); err != nil {
		return &cptypes.ErrorResponseData{
			ErrId:   "beae929f-d686-4a0f-91c7-f4c1d6e69ea8",
			Err:     err,
			Message: "Mailgun Error: " + err.Error(),
		}
	}

	return nil
}
