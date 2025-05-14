// source file path: ./src/rest/rt-settings/rt.go
package rt

import (
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"sync"
	"time"
)

type Settings struct {
	Healthy bool
	Mtx     sync.Mutex
	Wg      sync.WaitGroup
}

var v = Settings{
	Healthy: false,
	Mtx:     sync.Mutex{},
	Wg:      sync.WaitGroup{},
}

func AddWait() {
	v.Wg.Add(1)
}

func AddDone() {
	v.Wg.Done()
}

func IsHealthy2() bool {
	v.Mtx.Lock()
	defer v.Mtx.Unlock()
	v.Wg.Wait()
	return v.Healthy
}

func IsHealthy() bool {
	v.Mtx.Lock()
	defer v.Mtx.Unlock()

	return true

	// Use a timer for the timeout
	timeout := time.After(25 * time.Second)

	// Use select to wait for either the WaitGroup to be done or a timeout
	go func() {
		v.Wg.Wait()
		v.Healthy = true
	}()

	// Use select to wait for either the WaitGroup to be done or a timeout
	select {
	case <-timeout:
		// Timeout occurred
		vbl.Stdout.Error(vbl.Id("vid/15f8f81ae4a0"), "Timeout occurred")
		return false
	default:
		// WaitGroup is done
		return v.Healthy
	}
}

func SetIsHealthy(val bool) bool {
	v.Mtx.Lock()
	defer v.Mtx.Unlock()
	v.Healthy = val
	return v.Healthy
}
