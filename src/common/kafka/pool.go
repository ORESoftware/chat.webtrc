// source file path: ./src/common/kafka/pool.go
package virl_kafka

import (
	"fmt"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	vapm "github.com/oresoftware/chat.webrtc/src/common/apm"
	virl_cleanup "github.com/oresoftware/chat.webrtc/src/common/cleanup"
	vhp "github.com/oresoftware/chat.webrtc/src/common/handle-panic"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"sync"
	"time"
)

type VirlKafkaProducer struct {
	conn *kafka.Producer
	mu   *sync.RWMutex
}

type KafkaProducerConnPool struct {
	mu          *sync.Mutex
	counter     int
	connections []*VirlKafkaProducer
	config      *kafka.ConfigMap
	// other fields as necessary, like a channel for signaling, mutex for synchronization, etc.
}

func NewKafkaProducerConnPool(size int, conf *kafka.ConfigMap) (*KafkaProducerConnPool, error) {

	pool := &KafkaProducerConnPool{
		connections: make([]*VirlKafkaProducer, 0, size),
		config:      conf,
		mu:          &sync.Mutex{},
	}

	virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {

		go func() {
			defer func() {
				if r := vhp.HandlePanic("d8753991e896"); r != nil {
					vapm.SendTrace("7f83cdd8-a33f-4643-b9b0-cdabbacb6fc0", fmt.Sprintf("%v", r))
					vbl.Stdout.Error(vbl.Id("vid/2a5d279f0c35"), fmt.Sprintf("%v", r))
					vapm.SendTrace("0be9418c-d06d-4c17-93f8-a2a9df0bdf65", fmt.Sprintf("%v", r))
				}
			}()

			vbl.Stdout.Info("83266556-1710-4ea3-9ab7-c38e2c282a48", "flushing kafka messages")
			for _, x := range pool.connections {
				x.mu.Lock()
				x.conn.Flush(5 * 1000)
				x.mu.Unlock()
			}
			wg.Done()
		}()

	})

	defer pool.mu.Unlock()
	pool.mu.Lock()

	for i := 0; i < size; i++ {

		if err := func(i int) error {

			// Create a new producer
			// TODO ping Kafka?
			producer, err := kafka.NewProducer(pool.config)

			if err != nil {
				vbl.Stdout.Warn(vbl.Id("vid/fc79a93c6cd9"), err)
				return err
			}

			pool.connections = append(pool.connections, &VirlKafkaProducer{
				conn: producer,
				mu:   &sync.RWMutex{},
			})

			return nil

		}(i); err != nil {
			vapm.SendTrace("b2360bc8-cd09-4656-aefa-f894283b33af", err)
			vbl.Stdout.Error(vbl.Id("vid/66283270ed38"), err)
			vapm.SendTrace("9e9505bb-d602-4cc8-bce7-af81586f90df", err)
			return nil, err
		}
	}

	return pool, nil
}

func (p *KafkaProducerConnPool) GetConnection() (*kafka.Producer, int) {
	//
	for {
		// loop until valid - see 'continue' statement below
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

		if true {
			p, err := kafka.NewProducer(p.config)
			if err != nil {
				// // Handle error - log, and retry with backoff
				connWrapper.mu.Unlock()
				vbl.Stdout.Warn(vbl.Id("vid/33efee80c475"), err)
				vapm.SendTrace("18a6c952-e80b-405a-acf2-585e28336598", err)
				// wait before retrying
				time.Sleep(3 * time.Second)
				continue
			}
			connWrapper.conn = p
		}
		conn := connWrapper.conn // Store the connection in a local variable to return after unlocking
		connWrapper.mu.Unlock()
		return conn, p.counter // Return the new or existing valid connection
	}
}
