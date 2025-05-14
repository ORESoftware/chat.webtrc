// source file path: ./src/ws/conn/conn-counter.go
package ws_conn

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
	vbl.Stdout.InfoF("Active connections for wss: %d", c.n)
}

func (c *ConnectionCounter) Decrease() {
	c.mu.Lock()
	c.n--
	c.mu.Unlock()
	vbl.Stdout.InfoF("Active connections for wss: %d", c.n)
}
