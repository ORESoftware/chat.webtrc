// source file path: ./src/common/vibelog/vibelog.go
package vbl

import (
	ll "github.com/oresoftware/json-logging/jlog/level"
	jlog "github.com/oresoftware/json-logging/jlog/lib"
	"os"
)

var getLogLevel = func() ll.LogLevel {
	switch os.Getenv("vibe_in_prod") {
	case "yes":
		return ll.WARN
	default:
		return ll.WARN
	}
}

func getHostName() string {
	var hn, _ = os.Hostname()
	if hn == "" {
		hn = "<unknown>"
	}
	return hn
}

var Stdout = jlog.CreateLogger("Vibe:Chat").
	SetLogLevel(getLogLevel()).
	SetHighPerf(true).
	SetEnvPrefix("vibe_log_").
	SetOutputFile(os.Stdout).
	AddMetaField("host_name", getHostName())

var Stderr = jlog.CreateLogger("Vibe:Chat/Stderr").
	SetLogLevel(ll.WARN).
	SetHighPerf(true).
	SetEnvPrefix("vibe_log_").
	SetOutputFile(os.Stderr).
	AddMetaField("host_name", getHostName())

func InfoWithReq(req struct{ Id string }, args ...interface{}) {
	Stdout.Info(req.Id, args)
}

var Id = jlog.Id

func init() {
	Stdout.Debug("48a084a4-0ac5-43e2-880a-8ceed903ee79", "vibe logging initialized...")
}
