// source file path: ./src/common/apm/apm.go
package vapm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/mailru/easyjson"
	virl_cleanup "github.com/oresoftware/chat.webrtc/src/common/cleanup"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	vbu "github.com/oresoftware/chat.webrtc/src/common/v-utils"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	virl_conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/user"
	vuc "github.com/oresoftware/chat.webrtc/src/ws/uc"
	"net/http"
	"runtime"
	"sync"
	"time"
)

type Item struct {
	Value     int
	Timestamp time.Time
}

type SafeMap struct {
	sync.RWMutex
	items     map[string]*Item
	maxSize   int
	maxRepeat int
}

func NewSafeMap(maxSize int) *SafeMap {
	return &SafeMap{
		items:     make(map[string]*Item),
		maxSize:   maxSize,
		maxRepeat: 3,
	}
}

func captureStackTrace(skip int) []string {
	var stackTrace []string

	// Loop to capture each level of the stack trace
	for i := skip; ; i++ { // Skip the first few callers to get to the interesting part of the stack
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break // No more callers
		}

		// Retrieve the details of the function call
		funcName := runtime.FuncForPC(pc).Name()

		// Create a string representing the current stack frame
		frame := fmt.Sprintf("%s:%d %s", file, line, funcName)

		// Append this frame's string to the slice
		stackTrace = append(stackTrace, frame)
	}

	return stackTrace
}

func (m *SafeMap) Set(key string) bool {
	m.Lock()
	defer m.Unlock()

	now := time.Now()

	// If the key exists, increment its value and update its timestamp
	if item, exists := m.items[key]; exists {
		if now.Sub(item.Timestamp).Hours() < 36 {
			if item.Value > m.maxRepeat {
				return false
			}
		}

		item.Value++
		item.Timestamp = time.Now() // Update timestamp to keep the recently accessed item
		return true
	}

	// Check and maintain the maximum size constraint
	keysToDelete := []string{}
	if len(m.items) >= m.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range m.items {
			if oldestTime.IsZero() || v.Timestamp.Before(oldestTime) {
				oldestTime = v.Timestamp
				oldestKey = k
			}
			if now.Sub(v.Timestamp).Hours() > 36 {
				keysToDelete = append(keysToDelete, k)
			}
		}
		delete(m.items, oldestKey)
		for _, k := range keysToDelete {
			// delete keys that are older than 36 hours
			delete(m.items, k)
		}
	}

	// Add the new key with an initial value of 1
	m.items[key] = &Item{
		Value:     1,
		Timestamp: time.Now(),
	}
	return true
}

var cfg = virl_conf.GetConf()
var traceChan = make(chan *vibe_types.Trace)
var queueChan = make(chan *[]QueueItem)

var sm = SafeMap{
	// used to prevent dupes (dupe hashes) from being sent to apm server!
	RWMutex: sync.RWMutex{},
	items:   make(map[string]*Item),
	maxSize: 500,
}

// Define the struct with an embedded channel
// type QueueItem struct {
//   Trace *vibe_types.Trace
// }

type QueueItem *[]byte

// Define the queue struct with a slice of QueueItems
type SafeQueue struct {
	items []QueueItem
	mu    sync.Mutex // A mutex to protect access to the queue
}

// NewSafeQueue creates a new instance of SafeQueue
func NewSafeQueue() *SafeQueue {
	return &SafeQueue{
		items: make([]QueueItem, 0),
	}
}

// Enqueue adds an item to the end of the queue
func (q *SafeQueue) Enqueue(item QueueItem) {
	q.mu.Lock()         // Lock the mutex before accessing the slice
	defer q.mu.Unlock() // Unlock the mutex when the function returns

	q.items = append(q.items, item)
}

// Dequeue removes and returns an item from the front of the queue
// Returns nil if the queue is empty
func (q *SafeQueue) Dequeue() *QueueItem {
	q.mu.Lock()         // Lock the mutex before accessing the slice
	defer q.mu.Unlock() // Unlock the mutex when the function returns

	if len(q.items) == 0 {
		return nil // Return nil if the queue is empty
	}

	item := q.items[0]    // Get the item at the front of the queue
	q.items = q.items[1:] // Remove the item from the queue
	return &item          // Return the item
}

func init() {
	// Start a goroutine that reads from the channel.

	virl_cleanup.AddCleanupFunc(func(g *sync.WaitGroup) {
		defer g.Done()
		// send a final message, so we don't exit until message is processed
		// TODO this won't work since it processes messages async
		// send a channel or callback instead
		traceChan <- &vibe_types.Trace{
			ID: "break",
		}
	})

	var q = NewSafeQueue()

	var apmUrl = cfg.APM_API_URL

	if false && !vbu.IsURL(apmUrl) {
		panic(vbu.ErrorFromArgs("f9397a47-5d42-4ff7-aaa8-59253d15e32d", "not a url"))
	}

	var client = &http.Client{
		Timeout: 5 * time.Second,
	}

	go func() {

		for {
			items, ok := <-queueChan
			if !ok {
				vbl.Stdout.Warn("fe5f6009-886e-472c-ae03-6e1e6048563e", "ok received for channel that should not be finished")
				continue
			}
			if items == nil {
				continue
			}

			m, err := json.Marshal(items)

			if err != nil {
				vbl.Stdout.Error("3d4c6e40-9d79-44f3-874c-c5d07dcf7757", err)
				continue
			}

			req, err := http.NewRequest("POST", apmUrl, bytes.NewBuffer(m))
			if err != nil {
				// handle error
				vbl.Stdout.Warn("fab218ad-81be-48a7-9fc8-dafa523e3214", err)
				continue
			}

			// Set the Content-Type header, adjust as needed based on your API requirements.
			req.Header.Set("Content-Type", "application/json")

			// Perform the HTTP request using the default client.
			// Consider using a custom HTTP client with timeouts set for production code.
			resp, err := client.Do(req)
			if err != nil {
				// handle error
				vbl.Stdout.Warn("5054b552-1a07-4b28-bef7-ea4a29c284aa", err)
				return
			}

			defer func() {
				if err := resp.Body.Close(); err != nil {
					vbl.Stdout.Warn("46dd94ec-7162-49ba-a58b-20304f487ac5", err)
				}
			}()
		}
	}()

	go func() {
		// This read operation will block until a message is sent on the channel.
		for {
			trc, ok := <-traceChan
			if trc == nil {
				continue
			}
			if trc != nil && trc.ID == "break" {
				vbl.Stdout.Warn("6460a9c1-4d76-4fb4-b322-9eeb56107961", "the 'break' msg received in apm handler.")
				break
			}

			if !ok {
				vbl.Stdout.Warn("c9648eb2-99d2-4153-9ee8-1a2f61c0d64c", "ok received for channel that should not be finished")
				continue
			}

			// if v := sm.Set(trc.Hash); !v {
			//   vbl.Stdout.Info("437063a6-fde2-42f8-abdf-c6798352bc60", "skipping trace:", trc)
			//   return
			// }

			jsonData, err := easyjson.Marshal(trc)

			if err != nil {
				// handle error, e.g., log it or send it to an error handling channel
				vbl.Stdout.Warn("a91aa442-6f14-4c30-bafc-8ee70c14b448", err)
				continue
			}

			q.Enqueue(&jsonData)

			// TODO: put the requests on a queue, every 3 seconds, send it
			// Create a new POST request with the serialized JSON body.

		}

	}()

}

func SendTraceWithUserCtxFromWebsocket(id string, uc *vuc.UserCtx, values ...interface{}) {
	vbl.Stdout.Warn(vbl.Id("23457314-1187-4802-826b-384da68c9538"), "sending info/error to apm.vibe:", id, values)

	if v := sm.Set(id); !v {
		vbl.Stdout.Debug("5e4db600-db02-4f10-8d54-63e5232632a3", "skipping trace:", id)
		return
	}

	traceChan <- &vibe_types.Trace{
		Hash:            id,
		UserID:          uc.UserId,
		DeviceID:        uc.DeviceId,
		DeviceType:      "DEVICE+TYPE",
		DeviceVersion:   "DEVICE+VERSION",
		TriggeredNotifs: false,
		RepoName:        "repo-name",
		CommitId:        "commit-id",
		FileName:        "file-name",
		LineNumber:      0,
		EventName:       "EVENT+NAME",
		Env:             "STAGE",
		PriorityLevel:   1,
		EventData:       values,
		ErrorTrace:      values,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		CreatedBy:       "gms.vibe",
		UpdatedBy:       uc.UserId,
	}
}

func SendTraceWithRESTUserCtx(id string, uc *user.AuthUser, values ...interface{}) {
	vbl.Stdout.Warn(vbl.Id("b6f3cd63-502d-414f-836d-c4b91d4db3eb"), "sending info/error to apm.vibe:", id, values)

	if v := sm.Set(id); !v {
		vbl.Stdout.Warn("80b03a95-ac34-43d8-9765-4966fecb0c72", "skipping trace:", id)
		return
	}

	traceChan <- &vibe_types.Trace{
		Hash:            id,
		UserID:          uc.UserId,
		DeviceID:        uc.DeviceId,
		DeviceType:      "DEVICE+TYPE",
		DeviceVersion:   "DEVICE+VERSION",
		TriggeredNotifs: false,
		RepoName:        "repo-name",
		CommitId:        "commit-id",
		FileName:        "file-name",
		LineNumber:      0,
		EventName:       "EVENT+NAME",
		Env:             "STAGE",
		PriorityLevel:   1,
		EventData:       values,
		ErrorTrace:      values,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		CreatedBy:       "gms.vibe",
		UpdatedBy:       uc.UserId,
	}
}

func SendTrace(id string, values ...interface{}) {
	vbl.Stdout.Warn(vbl.Id("476e6fc8-7ca6-4241-b18c-2fed9f19fd2f"), id, "sending info/error to apm.vibe:", id, values)

	if v := sm.Set(id); !v {
		vbl.Stdout.Warn("47b9fa31-5f2a-4ef1-91e3-d900bcbfbe5c", "skipping trace:", id)
		return
	}

	traceChan <- &vibe_types.Trace{
		Hash:            id,
		UserID:          "gms.vibe-server",
		DeviceID:        "gms.vibe-server",
		DeviceType:      "gms.vibe-server",
		DeviceVersion:   "DEVICE+VERSION", // TODO add git version
		TriggeredNotifs: false,
		RepoName:        "repo-name",
		CommitId:        "commit-id",
		FileName:        "file-name",
		LineNumber:      0,
		EventName:       "EVENT+NAME",
		Env:             "STAGE",
		PriorityLevel:   1,
		EventData:       values,
		ErrorTrace:      values,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		CreatedBy:       "gms.vibe-server",
		UpdatedBy:       "gms.vibe-server",
	}
}
