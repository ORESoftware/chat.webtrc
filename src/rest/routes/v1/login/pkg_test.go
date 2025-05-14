// source file path: ./src/rest/routes/v1/login/pkg_test.go
package login

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/oresoftware/chat.webrtc/v1/common/cptypes"
	th "github.com/oresoftware/chat.webrtc/v1/test/helpers"
	"sync"
	"testing"
	"time"
)

// bump1

func TestLogin(t *testing.T) {

	var randomEmailUser = uuid.New().String()

	var randomEmailAddress = fmt.Sprintf("alex+%s@vibeirl.com", randomEmailUser)
	var randomPassword = uuid.New().String()
	var testUserName = "cp test user"

	t.Run("login", func(t *testing.T) {

		{

			res := th.CMRequest{
				Endpoint: fmt.Sprintf("/cp/sign_up"),
				Method:   "POST",
				Body: cptypes.SignUpInfo{
					Email:    randomEmailAddress,
					Password: randomPassword,
					Name:     testUserName,
				},
			}.Run()

			defer res.Body.Close()

			expectedStatusCode := 202
			if res.StatusCode != expectedStatusCode {
				t.Fatalf(th.GetErrorMsg(expectedStatusCode, res.StatusCode))
			}

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

	})

	t.Run("logout", func(t *testing.T) {

		// not implemented yet
		return

		res := th.CMRequest{
			Endpoint: fmt.Sprintf("/cp/login/logout"),
		}.Run()

		defer res.Body.Close()

		expectedStatusCode := 501
		if res.StatusCode != expectedStatusCode {
			t.Fatalf(th.GetErrorMsg(expectedStatusCode, res.StatusCode))
		}

	})

	t.Run("login do NOT hit rate limit", func(t *testing.T) {

		return

		wg := sync.WaitGroup{}

		time.Sleep(time.Second * 120)

		wg.Add(1)

		go func() {

			for i := 0; i < 5; i++ {

				// we should be able to make 5 requests with 3 seconds each

				time.Sleep(3 * time.Second) // should not hit rate limit

				// Invalid JSON
				res := th.CMRequest{
					Endpoint: fmt.Sprintf("/cp/login"),
					Method:   "POST",
					Body:     `{"broken":"json"`,
				}.Run()

				expectedStatusCode := 500

				if res.StatusCode != expectedStatusCode {
					t.Fatalf("e9a559ef-2044-4f3f-aa05-ffc9f86bb3a4: login with broken JSON didn't receive 500 respons: %v", res.StatusCode)
				}
			}

			wg.Done()

		}()

		wg.Wait()

	})

	t.Run("login do NOT hit rate limit", func(t *testing.T) {

		return

		wg := sync.WaitGroup{}

		time.Sleep(time.Second * 120)

		wg.Add(1)

		go func() {

			for i := 0; i < 15; i++ {

				time.Sleep(9 * time.Second) // should not hit rate limit

				// Invalid JSON
				res := th.CMRequest{
					Endpoint: fmt.Sprintf("/cp/login"),
					Method:   "POST",
					Body:     `{"broken":"json"`,
				}.Run()

				expectedStatusCode := 500

				if res.StatusCode != expectedStatusCode {
					t.Fatalf("17ce628a-72c5-4581-9308-1ae8dc669eba: login with broken JSON didn't receive 500 respons: %v", res.StatusCode)
				}
			}

			wg.Done()

		}()

		wg.Wait()

	})

	t.Run("login should hit rate limit", func(t *testing.T) {

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
					Endpoint: fmt.Sprintf("/cp/login"),
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
				t.Fatalf("c7af7fa1-f08a-46f3-ba83-5a4c8429770a: Should have got at least one http 429 response")
			}

			wg.Done()

		}()

		wg.Wait()

	})

}
