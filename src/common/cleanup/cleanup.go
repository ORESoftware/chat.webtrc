// source file path: ./src/common/cleanup/cleanup.go
package virl_cleanup

import (
	vbl "github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"os"
	"sync"
)

var cleanUpFuncs = []func(*sync.WaitGroup){}
var once = sync.Once{}
var mu = sync.Mutex{}

func AddCleanupFunc(x func(*sync.WaitGroup)) {
	defer mu.Unlock()
	mu.Lock()
	cleanUpFuncs = append(cleanUpFuncs, x)
}

func RunCleanUpFuncs(err error) {

	vbl.Stdout.Warn(vbl.Id("vid/c3356753b131"), "starting clean up funcs")

	if err != nil {
		vbl.Stdout.Warn("ffafb8d4-59d5-479d-9e3c-582516157a72", err)
	}

	mu.Lock() // IMPORTANT - want to make all the other calls wait
	// IMPORTANT BECAUSE: ^^ the lock call above -
	// if the other calls aren't forced to wait, then they will explicitly call os.Exit(...)

	once.Do(func() {
		vbl.Stdout.Warn(vbl.Id("vid/50030c9556ea"), "running clean up funcs")
		var wg sync.WaitGroup
		for i := len(cleanUpFuncs) - 1; i >= 0; i-- {
			// just like defer statement execution, we want to execute these in the reverse order of registration
			fnc := cleanUpFuncs[i]
			wg.Add(1)
			vbl.Stdout.Warn("940b811a-58b3-4e50-a858-f489fbd4b4e4", "running next cleanup func..")
			fnc(&wg)
		}
		wg.Wait() // Block here until all workers complete
		if err == nil {
			os.Exit(0)
		} else {
			os.Exit(1)
		}

	})
}
