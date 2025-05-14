// source file path: ./src/common/verrors/runtime-errs.go
package virl_err

import (
	"fmt"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"sync"
)

// ThreadSafeQueue is a basic queue implementation that is safe for concurrent use.
type ThreadSafeQueue struct {
	items []interface{}
	mu    sync.Mutex
	cond  *sync.Cond
	cap   int
}

var ErrorQueue *ThreadSafeQueue = nil

// NewThreadSafeQueue creates a new instance of ThreadSafeQueue with the given capacity.
func NewThreadSafeQueue(capacity int) *ThreadSafeQueue {
	q := &ThreadSafeQueue{
		items: make([]interface{}, 0),
		cap:   capacity,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Enqueue adds an item to the end of the queue. If the queue is at capacity, the oldest item is dequeued automatically.
func (q *ThreadSafeQueue) Enqueue(item interface{}) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// If the queue is at capacity, remove the oldest item.
	if len(q.items) >= q.cap {
		// Optionally, handle the dequeued item before discarding it.
		v := q.items[0] // Example: Do something with the dequeued item, if needed.
		vbl.Stdout.Error("a7820d66-365c-4123-828c-8a34a88363f5", "error queue at capacity, oldest error:", v)
		q.items = q.items[1:]
	}

	q.items = append(q.items, item)
	// Signal to one waiting goroutine that there is a new item.
	q.cond.Signal()
}

// Dequeue removes and returns the first item from the queue. If the queue is empty, it blocks until an item is available.
func (q *ThreadSafeQueue) Dequeue() interface{} {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Wait while the queue is empty.
	for len(q.items) == 0 {
		q.cond.Wait()
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item
}

func (q *ThreadSafeQueue) Iterate(action func(item interface{})) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, item := range q.items {
		action(item)
	}
}

func main() {
	queue := NewThreadSafeQueue(100)

	// Example usage
	go func() {
		for i := 0; i < 200; i++ {
			queue.Enqueue(i)
			fmt.Println("Enqueued:", i)
		}
	}()

	go func() {
		for i := 0; i < 100; i++ { // Only dequeue 100 times for demonstration.
			item := queue.Dequeue()
			fmt.Println("Dequeued:", item)
		}
	}()

	// Wait for a while to observe the processing (not the best practice for real applications)
	select {}
}

func init() {
	ErrorQueue = NewThreadSafeQueue(100)
}
