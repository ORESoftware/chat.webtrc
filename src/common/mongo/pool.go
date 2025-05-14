// source file path: ./src/common/mongo/pool.go
package virl_mongo

import (
	"context"
	virl_cleanup "github.com/oresoftware/chat.webtrc/src/common/cleanup"
	"github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"sync"
	"time"
)

type MongoConnectionPool struct {
	mu            sync.Mutex
	counter       int
	connections   []*VirlMongoConnection
	connString    string
	clientOptions *options.ClientOptions
	// other fields as necessary, like a channel for signaling, mutex for synchronization, etc.
}

var bgCtx = context.Background()

type VirlMongoConnection struct {
	conn *mongo.Client
	mu   *sync.RWMutex
}

func NewMongoConnPool(size int, opts *options.ClientOptions) (*MongoConnectionPool, error) {

	pool := &MongoConnectionPool{
		connections:   make([]*VirlMongoConnection, 0, size),
		clientOptions: opts,
	}

	pool.mu.Lock()
	defer pool.mu.Unlock()

	for i := 0; i < size; i++ {

		if err := func(i int) error {

			mongoClient, err := mongo.Connect(bgCtx, opts)

			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/3ae94f112fa8"), "Error connecting to MongoDB:", err)
			}

			// rsCtx, cancel := context.WithTimeout(context.Background(), time.Second*6)
			//
			// defer func() {
			//   cancel()
			// }()
			//
			// if err := mongoClient.Ping(rsCtx, &readpref.ReadPref{}); err != nil {
			//   vbl.Stdout.Warn(vbl.Id("vid/6f2f8c0e4216"), err)
			//   return err
			// }

			virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
				defer wg.Done()
				vbl.Stdout.Info("b4f501bf-7c6e-453f-920c-364ea644ae55", "virl cleanup - disconnecting from mongodb...")
				if err := mongoClient.Disconnect(context.Background()); err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/6b206d958a2b"), err)
				}
			})

			pool.connections = append(pool.connections, &VirlMongoConnection{
				mongoClient, &sync.RWMutex{},
			})

			return nil

		}(i); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/971675403944"), err)
			return nil, err
		}
	}

	return pool, nil
}

func (p *MongoConnectionPool) GetConnection() (*mongo.Client, int) {
	//
	for {
		//
		p.mu.Lock()
		p.counter = (p.counter + 1) % len(p.connections)
		var counter = p.counter
		connWrapper := p.connections[counter]
		p.mu.Unlock()

		connWrapper.mu.RLock()
		if connWrapper.conn != nil { // && !connWrapper.conn.IsClosed() {
			connWrapper.mu.RUnlock()
			return connWrapper.conn, counter
		}
		connWrapper.mu.RUnlock()
		// Connection is nil or closed, try to re-establish it
		connWrapper.mu.Lock()
		// Re-check to avoid race condition
		mongoClient, err := mongo.Connect(context.Background(), p.clientOptions)

		if err != nil {
			// // Handle error - log, and retry with backoff
			connWrapper.mu.Unlock()
			vbl.Stdout.Warn(vbl.Id("vid/e1ca958eb7e0"), err)
			// wait before retrying
			time.Sleep(2 * time.Second)
			continue
		}

		virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
			defer wg.Done()
			vbl.Stdout.Info("8be084b6-4e22-4cac-9f22-6c5d3b8ad473", "disconnecting from mongodb")
			if err := mongoClient.Disconnect(context.Background()); err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/e6126b91e063"), err)
			}
		})

		connWrapper.conn = mongoClient
		connWrapper.mu.Unlock()
		return mongoClient, counter // Return the new or existing valid connection
	}
}
