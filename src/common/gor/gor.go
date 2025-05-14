// source file path: ./src/common/gor/gor.go
package gor

import (
	"fmt"
	vhp "github.com/oresoftware/chat.webrtc/src/common/handle-panic"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"runtime"
	"sync"
	"time"
)

type GorInfo struct {
	keepGoing  bool
	mtx        *sync.Mutex
	LastActive time.Time
	Name       string
	StartFile  string
	StartLine  int
	MetaInfo   interface{}
	Timer      *time.Timer // Changed from Ctx to Timer for clarity
}

var goroutinesMap = make(map[*GorInfo]*GorInfo)
var mtx = sync.Mutex{}

type LastActiveInfo struct {
	Info string
}

type Updater interface {
	GetTimer() *time.Timer
	UpdateLastActive(time.Time, LastActiveInfo)
	CloseOut()
}

// Implements the Updater interface for GorInfo.
func (g *GorInfo) UpdateLastActive(t time.Time, info LastActiveInfo) {
	defer g.mtx.Unlock()
	g.mtx.Lock()
	g.LastActive = t
	g.MetaInfo = info
	// Stop the old timer if it exists
	if g.Timer != nil {
		g.Timer.Reset(300 * time.Second)
		return
	}
	// Create a new timer that times out after 10 seconds
	g.Timer = time.NewTimer(300 * time.Second)
}

// Implements the Updater interface for GorInfo.
func (g *GorInfo) GetTimer() *time.Timer {
	defer g.mtx.Unlock()
	g.mtx.Lock()

	if g.Timer == nil {
		g.Timer = time.NewTimer(300 * time.Second)
	}

	return g.Timer
}

// Implements the Updater interface for GorInfo.
func (g *GorInfo) CloseOut() {
	defer g.mtx.Unlock()
	g.mtx.Lock()

	g.keepGoing = false

	if g.Timer != nil {
		if !g.Timer.Stop() {
			<-g.Timer.C
		}
		g.Timer = nil
	}

	return
}

func Gor(f func(z Updater)) {

	_, file, line, ok := runtime.Caller(1)

	if !ok {
		file = "unknown"
		line = -1
	}

	info := &GorInfo{
		keepGoing:  true,
		mtx:        &sync.Mutex{},
		LastActive: time.Now(),
		StartFile:  file,
		StartLine:  line,
		MetaInfo:   nil,
		Timer:      time.NewTimer(time.Second * 30),
	}

	mtx.Lock()
	goroutinesMap[info] = info
	mtx.Unlock()

	go func() {

		go func() {
			for {
				func() { // use this self-invoking func so we can use defer with unlock call
					time.Sleep(time.Second * 15)
					defer mtx.Unlock()
					mtx.Lock()

					var inf = goroutinesMap[info]
					if inf == nil {
						return
					}

					if !inf.keepGoing {
						return
					}

					if time.Now().Sub(inf.LastActive) > 10*time.Minute {
						vbl.Stdout.Debug(vbl.Id("vid/067f294a300c"), "this goroutine is inactive:", inf)
					}
				}()
			}
		}()

		defer func() {
			if r := vhp.HandlePanic("784b391210b0"); r != nil {
				vbl.Stdout.Error(vbl.Id("vid/730a4aca8423"), fmt.Sprintf("%v", r))
			}
		}()

		defer func() {
			mtx.Lock()
			info.CloseOut()
			delete(goroutinesMap, info)
			mtx.Unlock()
		}()

		// call original func
		// when defer is called, we know f is done
		f(info)

	}()
}
