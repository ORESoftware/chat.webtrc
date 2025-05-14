// source file path: ./src/common/rabbitmq/pool.go
package rabbitmq

import (
	"github.com/oresoftware/chat.webtrc/src/common/vibelog"
	amqp "github.com/rabbitmq/amqp091-go"
	"net"
	"sync"
	"time"
)

// CustomDialer connects to RabbitMQ using a custom net.Dialer to disable Nagle's algorithm.
func CustomDialer(pool *RabbitMQConnectionPool) (*amqp.Connection, error) {
	// // custom dialer

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	conn, err := amqp.DialConfig(pool.connString, amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			c, err := dialer.Dial(network, addr)
			if err != nil {
				return nil, err
			}

			// Assuming c is a *net.TCPConn, disable Nagle's algorithm.
			if tcpConn, ok := c.(*net.TCPConn); ok {
				if err := tcpConn.SetNoDelay(true); err != nil {
					vbl.Stdout.WarnF("Failed to disable Nagle's algorithm: %v", err)
					// Handle error or proceed, depending on your error handling strategy.
				}
			}

			return c, nil
		},
	})
	return conn, err
}

type RabbitMQConnection struct {
	conn *amqp.Connection
	mu   sync.RWMutex
}

type RabbitMQConnectionPool struct {
	delivs      chan amqp.Delivery
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

	defer pool.mu.Unlock()
	pool.mu.Lock()

	for i := 0; i < size; i++ {

		if err := func(i int) error {

			// conn, err := amqp.Dial(connectionString)
			conn, err := CustomDialer(pool)
			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/489950f680ca"), err)
				return err // handle failed connection appropriately
			}

			pool.connections = append(pool.connections, &RabbitMQConnection{
				conn: conn,
				mu:   sync.RWMutex{},
			})

			return nil

		}(i); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/35d178cdb472"), err)
			return nil, err
		}
	}

	return pool, nil
}

func (p *RabbitMQConnectionPool) Read(f func()) (*amqp.Connection, int) {
	d := make(chan amqp.Delivery)
	for {
		conn, _ := p.GetConnection()
		ch, _ := conn.Channel()
		x, _ := ch.Consume("", "", false, false, false, false, amqp.Table{})
		for v := range x {
			d <- v
		}

	}

}

func (p *RabbitMQConnectionPool) GetConnection() (*amqp.Connection, int) {
	//
	for {
		// we loop until valid conn (see the continue statement below)
		p.mu.Lock()
		p.counter = (p.counter + 1) % len(p.connections)
		var counter = p.counter
		connWrapper := p.connections[counter]
		p.mu.Unlock()

		connWrapper.mu.RLock()
		if connWrapper.conn != nil && !connWrapper.conn.IsClosed() {
			connWrapper.mu.RUnlock()
			return connWrapper.conn, counter
		}
		connWrapper.mu.RUnlock()

		// Connection is nil or closed, try to re-establish it
		connWrapper.mu.Lock()
		// Re-check to avoid race condition
		if connWrapper.conn == nil || connWrapper.conn.IsClosed() {
			conn, err := CustomDialer(p)
			if err != nil {
				// // Handle error - log, and retry with backoff
				connWrapper.mu.Unlock()
				vbl.Stdout.Warn(vbl.Id("vid/ce634611e837"), err)
				// wait before retrying
				time.Sleep(3 * time.Second)
				continue
			}
			connWrapper.conn = conn
		}
		conn := connWrapper.conn // Store the connection in a local variable to return after unlocking
		connWrapper.mu.Unlock()
		return conn, counter // Return the new or existing valid connection
	}
}
