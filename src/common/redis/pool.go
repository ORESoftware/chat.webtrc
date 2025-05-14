// source file path: ./src/common/redis/pool.go
package virl_redis

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"net"
	"sync"
	"time"
)

var bgCtx = context.Background()

type RedisConnection struct {
	isClosed bool
	conn     *redis.Client
	mu       *sync.RWMutex
}

type RedisConnectionPool struct {
	mu          *sync.Mutex
	counter     int
	connections []*RedisConnection
	connString  string
	// other fields as necessary, like a channel for signaling, mutex for synchronization, etc.
}

func createConnection(pool *RedisConnectionPool) (*redis.Client, error) {
	// create conn with tcp-no-delay set to true
	conn := redis.NewClient(&redis.Options{
		Addr:     pool.connString, // Redis server address
		Password: "",              // no password set
		DB:       0,               // use default DB
		PoolSize: 3,               // Size of the connection pool
		// Setting retry backoff options
		MinRetryBackoff: 3 * time.Second, // minimum backoff duration
		MaxRetryBackoff: 7 * time.Second,
		MinIdleConns:    1,
		PoolTimeout:     30, // in seconds
		Dialer: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.DialTimeout(network, addr, 5*time.Second)
			if err != nil {
				return nil, err
			}
			tcpConn, ok := conn.(*net.TCPConn)
			if !ok {
				vbl.Stdout.Warn("2660ab35-37b1-4d6b-bb43-6fc5af139a41", "could not get tcp conn type", err)
				return nil, err // Optionally, replace with a more specific error
			}
			if err := tcpConn.SetNoDelay(true); err != nil {
				vbl.Stdout.Warn("36e0b187-1886-4091-b53e-dd2d1c2cc887", "could not set no-delay for redis - ", err)
				return nil, err
			}
			return tcpConn, nil
		},
	})

	rsCtx, cancel := context.WithTimeout(context.Background(), time.Second*9)

	defer func() {
		cancel()
	}()

	_, err := conn.Ping(rsCtx).Result()
	return conn, err

}

func NewRedisConnectionPool(size int, connectionString string) (*RedisConnectionPool, error) {

	pool := &RedisConnectionPool{
		connections: []*RedisConnection{},
		connString:  connectionString,
		mu:          &sync.Mutex{},
	}

	for i := 0; i < size; i++ {

		if err := func(i int) error {

			conn, err := createConnection(pool)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/12c29d0ea7b3"), err)
				return err // handle failed connection appropriately
			}

			pool.connections = append(pool.connections, &RedisConnection{
				conn: conn,
				mu:   &sync.RWMutex{},
			})

			return nil

		}(i); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/a8a6939d79fc"), err)
			return nil, err
		}
	}

	return pool, nil
}

func (p *RedisConnectionPool) GetConnection() (*redis.Client, int) {
	//
	for {
		//
		p.mu.Lock()
		p.counter = (p.counter + 1) % len(p.connections)
		var counter = p.counter
		connWrapper := p.connections[counter]
		p.mu.Unlock()

		connWrapper.mu.RLock()
		if connWrapper.conn != nil {
			connWrapper.mu.RUnlock()
			return connWrapper.conn, counter
		}
		connWrapper.mu.RUnlock()

		// Connection is nil or closed, try to re-establish it
		connWrapper.mu.Lock()
		// Re-check to avoid race condition

		connWrapper.conn = nil // connWrapper.conn.IsClosed() {

		if true { // later we can check to see if conn is nil/closed
			newConn, err := createConnection(p)
			if err != nil {
				connWrapper.mu.Unlock()
				vbl.Stdout.Warn(vbl.Id("vid/6bce929fb5d6"), err)
				time.Sleep(3 * time.Second)
				continue
			}
			connWrapper.conn = newConn
		}

		conn := connWrapper.conn // Store the connection in a local variable to return after unlocking
		connWrapper.mu.Unlock()
		return conn, counter // Return the new or existing valid connection
	}
}
