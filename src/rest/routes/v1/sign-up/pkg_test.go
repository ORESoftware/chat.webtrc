// source file path: ./src/rest/routes/v1/sign-up/pkg_test.go
package sign_up

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/oresoftware/chat.webrtc/v1/common/cptypes"
	th "github.com/oresoftware/chat.webrtc/v1/test/helpers"
)

func TestLogin(t *testing.T) {

	var randomEmailAddress = fmt.Sprintf("test+%s@vibeirl.com", uuid.New().String())
	var randomEmailAddress2 = fmt.Sprintf("test+%s@vibeirl.com", uuid.New().String())

	// foo
	var randomPassword = uuid.New().String()
	var testUserName = "cp test user"
	var randRefferedBy = "0de923d5-9375-47f6-8021-23ab4045b494"

	t.Run("signup", func(t *testing.T) {

		{
			// Invalid JSON
			res := th.CMRequest{
				Endpoint: fmt.Sprintf("/cp/sign_up"),
				Method:   "POST",
				Body:     "{\"broken\": \"json\"",
			}.Run()

			expectedStatusCode := 500
			if res.StatusCode != expectedStatusCode {
				t.Fatalf("49a62657-d2ce-4c61-8861-5b587a75eb54: Signing up with broken JSON didn't receive 500 response")
			}

			// Email required
			res = th.CMRequest{
				Endpoint: fmt.Sprintf("/cp/sign_up"),
				Method:   "POST",
				Body: cptypes.SignUpInfo{
					Email:    "",
					Password: randomPassword,
					Name:     testUserName,
				},
			}.Run()

			expectedStatusCode = 422
			if res.StatusCode != expectedStatusCode {
				t.Fatalf("dec9ca87-3267-411a-a682-e76720a0fdcc: Signing up with empty email didn't receive 422 response")
			}

			// Valid Signup
			res = th.CMRequest{
				Endpoint: fmt.Sprintf("/cp/sign_up"),
				Method:   "POST",
				Body: cptypes.SignUpInfo{
					Email:    randomEmailAddress,
					Password: randomPassword,
					Name:     testUserName,
				},
			}.Run()

			expectedStatusCode = 202
			if res.StatusCode != expectedStatusCode {
				t.Fatalf("500b1bb2-c6b8-4dec-98b7-778580060fde: Signing up with broken JSON didn't receive 202 ")
			}

			// Valid Signup with refferal
			res = th.CMRequest{
				Endpoint: fmt.Sprintf("/cp/sign_up"),
				Method:   "POST",
				Body: cptypes.SignUpInfo{
					Email:      randomEmailAddress2,
					Password:   randomPassword,
					Name:       testUserName,
					RefferedBy: randRefferedBy,
				},
			}.Run()

			expectedStatusCode = 202
			if res.StatusCode != expectedStatusCode {
				t.Fatalf("45313e50-a2f4-4d7f-9a66-c50c705266b8: Signing up with broken JSON didn't receive 202 ")
			}
			defer res.Body.Close()
		}

		res := th.CMRequest{
			Endpoint: fmt.Sprintf("/cp/login"),
			Method:   "POST",
			Body: cptypes.LoginInfo{
				Email:    randomEmailAddress,
				Password: randomPassword,
			},
		}.Run()

		expectedStatusCode := 200
		if res.StatusCode != expectedStatusCode {
			t.Fatalf(th.GetErrorMsg(expectedStatusCode, res.StatusCode))
		}

		// All other possible errors are due to Postgre errors which can't be simulated
	})

	t.Run("signup do NOT hit rate limit", func(t *testing.T) {

		return

		wg := sync.WaitGroup{}

		wg.Add(1)
		go func() {

			for i := 0; i < 5; i++ {

				// we should be able to make 5 requests with 3 seconds in between without issue

				time.Sleep(3 * time.Second) // should not hit rate limit

				// Invalid JSON
				res := th.CMRequest{
					Endpoint: fmt.Sprintf("/cp/sign_up"),
					Method:   "POST",
					Body:     `{"broken":"json"`,
				}.Run()

				expectedStatusCode := 500

				if res.StatusCode != expectedStatusCode {
					t.Fatalf("cm:ee5f7240-2c92-444f-86e0-a24c2ea3cae3: Signing up with broken JSON didn't receive 500 response")
				}
			}

			wg.Done()

		}()

		wg.Wait()

	})

	t.Run("signup do NOT hit rate limit", func(t *testing.T) {

		return

		wg := sync.WaitGroup{}

		time.Sleep(time.Second * 120)

		wg.Add(1)
		go func() {

			for i := 0; i < 15; i++ {

				time.Sleep(9 * time.Second) // should not hit rate limit

				// Invalid JSON
				res := th.CMRequest{
					Endpoint: fmt.Sprintf("/cp/sign_up"),
					Method:   "POST",
					Body:     `{"broken":"json"`,
				}.Run()

				expectedStatusCode := 500

				if res.StatusCode != expectedStatusCode {
					t.Fatalf("cm:b3fd30a0-e14b-4c6f-9b46-277d4914733f: Signing up with broken JSON didn't receive 500 response")
				}
			}

			wg.Done()

		}()

		wg.Wait()

	})

	t.Run("signup *should hit* rate limit", func(t *testing.T) {

		return

		wg := sync.WaitGroup{}

		time.Sleep(time.Second * 120)

		wg.Add(1)

		go func() {

			var atLeastOneHttp429 = false

			for i := 0; i < 15; i++ {

				time.Sleep(7 * time.Second) // should hit the rate limit

				// Invalid JSON
				res := th.CMRequest{
					Endpoint: fmt.Sprintf("/cp/sign_up"),
					Method:   "POST",
					Body:     `{"broken":"json"`,
				}.Run()

				expectedStatusCode := 429

				if res.StatusCode == expectedStatusCode {
					atLeastOneHttp429 = true
					break
				}
			}

			if !atLeastOneHttp429 {
				t.Fatalf("7d0d6099-dc18-41b0-b792-3926f2fcf0e0: should have got at least one http 429 response")
			}

			wg.Done()

		}()

		wg.Wait()

	})

}
