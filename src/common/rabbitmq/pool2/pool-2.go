// source file path: ./src/common/rabbitmq/pool2/pool-2.go
package pool2

import (
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	amqp "github.com/rabbitmq/amqp091-go"
	"sync"
	"time"
)

type RabbitMQConnection struct {
	conn *amqp.Connection
	mu   sync.RWMutex
}

type RabbitMQConnectionPool struct {
	mu          sync.Mutex
	counter     int
	connections []*RabbitMQConnection
	connString  string
	// other fields as necessary, like a channel for signaling, mutex for synchronization, etc.
}

func NewRabbitMQConnectionPool(size int, connectionString string) (*RabbitMQConnectionPool, error) {

	pool := &RabbitMQConnectionPool{
		connections: make([]*RabbitMQConnection, 0, size),
		connString:  connectionString,
		mu:          sync.Mutex{},
	}

	for i := 0; i < size; i++ {

		if err := func(i int) error {

			defer pool.mu.Unlock()
			pool.mu.Lock()

			conn, err := amqp.Dial(connectionString)
			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/c9d45f1f8402"), err)
				return err // handle failed connection appropriately
			}

			pool.connections = append(pool.connections, &RabbitMQConnection{
				conn: conn,
				mu:   sync.RWMutex{},
			})

			notifyClose := make(chan *amqp.Error)
			conn.NotifyClose(notifyClose)

			// Handle connection close event
			go func(i int) {
				for {
					err := <-notifyClose
					if err != nil {
						vbl.Stdout.Warn(vbl.Id("vid/4178f938fab1"), "amqp connection closed:", err)
						// TODO: fix this - needs to have notifyClose attached to this newConn - needs to share code with above
						newConn, err := amqp.Dial(pool.connString)
						if err == nil {
							pool.connections[i].mu.Lock()
							pool.connections[i].conn = newConn
							pool.connections[i].mu.Unlock()
						} else {
							// can't establish a connection now, don't bother retrying yet
							vbl.Stdout.Warn(vbl.Id("vid/300f3f52d104"), err)
							// pool.connections[i] = nil
						}
					}
				}
			}(i)

			return nil

		}(i); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/dda005782478"), err)
			return nil, err
		}
	}

	return pool, nil
}

func (p *RabbitMQConnectionPool) GetConnection() (*amqp.Connection, int) {
	//
	for {
		//
		p.mu.Lock()
		p.counter = (p.counter + 1) % len(p.connections)
		connWrapper := p.connections[p.counter]
		p.mu.Unlock()

		connWrapper.mu.RLock()
		if connWrapper.conn != nil && !connWrapper.conn.IsClosed() {
			connWrapper.mu.RUnlock()
			return connWrapper.conn, p.counter
		}
		connWrapper.mu.RUnlock()

		// Connection is nil or closed, try to re-establish it
		connWrapper.mu.Lock()
		// Re-check to avoid race condition
		if connWrapper.conn == nil || connWrapper.conn.IsClosed() {
			conn, err := amqp.Dial(p.connString)
			if err != nil {
				// Handle error (log, retry with backoff, etc.)
				connWrapper.mu.Unlock()
				vbl.Stdout.Warn(vbl.Id("vid/1ea6eb9af775"), err)
				// wait 2 seconds before retrying
				time.Sleep(4 * time.Second)
				continue
			}
			connWrapper.conn = conn
		}
		conn := connWrapper.conn // Store the connection in a local variable to return after unlocking
		connWrapper.mu.Unlock()
		return conn, p.counter // Return the new or existing valid connection
	}
}
