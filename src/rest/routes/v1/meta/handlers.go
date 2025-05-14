// source file path: ./src/rest/routes/v1/meta/handlers.go
package v1_routes_meta

import (
	"fmt"
	vapm "github.com/oresoftware/chat.webrtc/src/common/apm"
	virl_cleanup "github.com/oresoftware/chat.webrtc/src/common/cleanup"
	vibe_types "github.com/oresoftware/chat.webrtc/src/common/types"
	vbl "github.com/oresoftware/chat.webrtc/src/common/vibelog"
	conf "github.com/oresoftware/chat.webrtc/src/config"
	"github.com/oresoftware/chat.webrtc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webrtc/src/rest/middleware"
	"io"
	"math"
	"net/http"
	"os"
	"path"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"time"
)

func createCleanShutdownHandler(c *ctx.VibeCtx) http.HandlerFunc {

	if err := c.IORead.ConsumeFromRedisChannel("clean-shutdown", 0, []string{}, func(err error, data interface{}) {

		if err != nil {
			vbl.Stdout.Error(vbl.Id("vid/ed27e07464b3"), err)
			return
		}
		// do shutdown
		virl_cleanup.RunCleanUpFuncs(nil)

	}); err != nil {
		vbl.Stdout.Critical("95deff8b-9096-4003-9c64-c60ae5f2aba0", err)
		virl_cleanup.RunCleanUpFuncs(err)
	}

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			type RedisPayload struct {
				Redis bool // struct marker
				Meta  struct {
					IsList      bool
					TimeCreated string
				}
				Data interface{}
			}

			go func() {
				// tell other servers we shutting down too
				err := c.IO.PublishMessageToRedis("clean-shutdown", cx.User, vibe_types.RedisPayload{
					Redis: true,
					Meta: struct {
						IsList      bool
						TimeCreated string
					}{},
					Data: struct{ ShutDown bool }{true},
				}, 0, []string{})

				if err != nil {
					vbl.Stdout.Warn(vbl.Id("vid/f083ef8a3233"), err)
				}

				virl_cleanup.RunCleanUpFuncs(err)
			}()

			return c.GenericMarshal(200, struct{ OK bool }{true})

		}))
}

func createMetaDownloadPprofHandler(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			wd, err := os.Getwd()

			if err != nil {
				vapm.SendTrace("4501d7ea-d2db-499d-ba2a-ff1f269d1ebb", err)
				vbl.Stdout.Error(vbl.Id("vid/f5a7c120051f"), err)
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "f68f1ba3-889d-40e6-b3ff-24224d72c3b2",
					Err:   err,
				})
			}

			pprof.StopCPUProfile()

			filePath := path.Join(wd, "logs/cpu.pprof")

			// Open the file
			file, err := os.Open(filePath)

			if err != nil {
				vapm.SendTrace("d819cb18-1a8e-47ec-b7e7-172265557c40", err)
				vbl.Stdout.Error(vbl.Id("vid/8a34fdd66044"), err)
				return http.StatusNotFound, []byte("File not found")
			}

			defer file.Close()

			// trace is referenced in src/main/main.go
			trace.Stop()

			// Set the Content-Disposition header to trigger a download
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", "go-pprof.bin"))

			_, err = io.Copy(w, file)
			if err != nil {
				vapm.SendTrace("383a7d73-4614-4bc2-b3be-615b4c058883", err)
				vbl.Stdout.Error(vbl.Id("vid/70c454aa1abb"), err)
				return 500, []byte("Error copying file")
			}

			// we don't need to write more, we are done
			return -1, []byte("")

		}))
}

func createMetaDownloadHandler(c *ctx.VibeCtx) http.HandlerFunc {
	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			wd, err := os.Getwd()

			if err != nil {
				vapm.SendTrace("8f686105-e781-4ad2-acfc-0d3a5c72ddce", err)
				vbl.Stdout.Error(vbl.Id("vid/cbb471d895ed"), err)
				return c.WriteJSONErrorResponse(500, vibe_types.ErrorResponseData{
					ErrId: "a6a41c00-c6dc-4222-9d1d-22bd6a284578",
					Err:   err,
				})
			}

			filePath := path.Join(wd, "logs/trace.out")

			// Open the file
			file, err := os.Open(filePath)

			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/5b30e478eb5c"), err)
				return http.StatusNotFound, []byte("File not found")
			}

			defer file.Close()

			// trace is referenced in src/main/main.go
			trace.Stop()

			// Set the Content-Disposition header to trigger a download
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", "go-trace.txt"))

			_, err = io.Copy(w, file)
			if err != nil {
				vbl.Stdout.Error(vbl.Id("vid/853a4ba1a6fb"), err)
				return 500, []byte("Error copying file")
			}

			// we don't need to write more, we are done
			return -1, []byte("")

		}))
}

func createMetaHandler(c *ctx.VibeCtx) http.HandlerFunc {

	// var gid = os2.Getegid()
	var cfg = conf.GetConf()
	var startTime = time.Now()
	hostname, err := os.Hostname()

	if err != nil {
		hostname = "(unknown hostname)"
	}

	gitCommit := "(unknown git commit)"

	if os.Getenv("vibe_jlog_git_commit") != "" {
		gitCommit = os.Getenv("vibe_jlog_git_commit")
	}

	if os.Getenv("vibe_git_commit") != "" {
		gitCommit = os.Getenv("vibe_git_commit")
	}

	//    hostname: hostname,
	//	  freeMemory: os.freemem(),
	//		totalMemory: os.totalmem(),
	//		memoryUsage: process.memoryUsage(),
	//		arch: arch,
	//		uptimeInMillis: uptimeInMillis,
	//		uptimeInMinutes: Number((uptimeInMillis / 1000 / 60).toFixed(4)).valueOf(),
	//		uptimeInHours: Number((uptimeInMillis / 1000 / 60 / 60).toFixed(3)),
	//		uptimeInDays: Number((uptimeInMillis / 1000 / 60 / 60 / 24).toFixed(2))

	wd, err := os.Getwd()

	if err != nil {
		vapm.SendTrace("2c7fed27-5e3c-489b-9805-64b7c506914b", err)
		vbl.Stdout.Error(vbl.Id("vid/ba20ebf22d08"), err)
		virl_cleanup.RunCleanUpFuncs(err)
		return nil
	}

	return mw.Middleware(
		mw.DoResponse(func(cx *mw.Ctx, w http.ResponseWriter, req *http.Request) (int, []byte) {

			var m runtime.MemStats
			runtime.ReadMemStats(&m)

			// TotalAlloc represents bytes allocated and not yet freed
			totalAllocated := m.TotalAlloc / (1024 * 1024) // Convert to megabytes

			// Sys represents the total bytes of memory obtained from the OS
			totalSys := m.Sys / (1024 * 1024) // Convert to megabytes

			// Free memory is the difference between Sys and TotalAlloc
			freeMemory := totalSys - totalAllocated

			var duration = time.Now().Sub(startTime)

			return c.GenericMarshal(200, struct {
				WorkingDir           string
				GitCommit            string
				Hostname             string
				UptimeMillis         int64
				UptimeSeconds        float64
				UptimeHours          float64
				UptimeDays           float64
				FreeMemory           uint64
				TotalSystemMemory    uint64
				TotalAllocatedMemory uint64
				Conf                 interface{}
			}{
				wd,
				gitCommit,
				hostname,
				duration.Milliseconds(),
				duration.Seconds(),
				duration.Hours(),
				math.Floor(duration.Hours() / 24),
				freeMemory,
				totalSys,
				totalAllocated,
				cfg,
			})
		}))
}
