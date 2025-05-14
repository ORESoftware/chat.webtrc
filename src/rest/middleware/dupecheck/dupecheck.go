// source file path: ./src/rest/middleware/dupecheck/dupecheck.go
package dupecheck

import (
	au "github.com/oresoftware/chat.webrtc/src/common/v-aurora"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"os"
)

/*

 this ditty is for checking middleware that's registered more than once in a chain.
 if it's registered more than once an error will be thrown at startup
 for example:

	r.Methods("POST").Path("/cp/login").HandlerFunc(
		mw.Middleware(
		  mw.AsJSON(),
      mw.AsJSON(),  // will throw an error
		  ctr.Login(c)),
	)

*/

type dupecheck struct {
	mwDupeMap     map[string]int
	isWithinStack bool
}

var DupeCheck = dupecheck{
	mwDupeMap:     map[string]int{},
	isWithinStack: false,
}

func (c *dupecheck) ResetMap() {
	c.mwDupeMap = make(map[string]int)
}

func (c *dupecheck) IsWithinStack() bool {
	return c.isWithinStack
}

func (c *dupecheck) PotentiallyEnterNewStack() bool {
	var isWithinStack = c.isWithinStack
	if !isWithinStack {
		c.ResetMap()
	}
	c.isWithinStack = true
	return isWithinStack
}

func (c *dupecheck) PotentiallyCleanStack(v bool) {
	if !v {
		c.isWithinStack = false
		c.ResetMap()
	}
}

func (c *dupecheck) CheckUuid(uuid string) {
	// this checks to see if a middleware func has been registered more than once
	if v, ok := c.mwDupeMap[uuid]; !ok {
		c.mwDupeMap[uuid] = 1
	} else if v > 0 {
		vbl.Stdout.Error(vbl.Id("vid/e6895f8bb738"), "the duplicate middleware is registered here:", au.Col.Magenta(au.Col.Bold(uuid)))
		vbu.PrintLocalStackTrace()
		vbl.Stdout.Critical("a5e2b493-867f-4a1e-b4b3-01290b86f0c6:", "duplicate middleware registered.")
		os.Exit(1)
	}
}
