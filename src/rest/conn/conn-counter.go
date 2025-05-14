// source file path: ./src/rest/conn/conn-counter.go
package rest_conn

import (
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"sync"
)

type ConnectionCounter struct {
	mu sync.Mutex // protects n
	n  int        // number of active connections
}

func (c *ConnectionCounter) Increase() {
	c.mu.Lock()
	c.n++
	c.mu.Unlock()
	vbl.Stdout.InfoF("Active connections for rest server: %d", c.n)
}

func (c *ConnectionCounter) Decrease() {
	c.mu.Lock()
	c.n--
	c.mu.Unlock()
	vbl.Stdout.InfoF("Active connections for rest server: %d", c.n)
}
