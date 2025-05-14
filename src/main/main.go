package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json" // Keep json for simplified stdin reading
	"fmt"
	muxctx "github.com/gorilla/context"
	"github.com/gorilla/mux"
	virl_cleanup "github.com/oresoftware/chat.webtrc/src/common/cleanup"
	virl_mongo "github.com/oresoftware/chat.webtrc/src/common/mongo"
	au "github.com/oresoftware/chat.webtrc/src/common/v-aurora"
	virl_const "github.com/oresoftware/chat.webtrc/src/common/v-constants" // May still be needed for other constants
	vbu "github.com/oresoftware/chat.webtrc/src/common/v-utils"
	virl_err "github.com/oresoftware/chat.webtrc/src/common/verrors"
	"github.com/oresoftware/chat.webtrc/src/common/vibelog"
	virl_conf "github.com/oresoftware/chat.webtrc/src/config"
	"github.com/oresoftware/chat.webtrc/src/rest/controller"
	"github.com/oresoftware/chat.webtrc/src/rest/ctx"
	mw "github.com/oresoftware/chat.webtrc/src/rest/middleware"
	virl_ws "github.com/oresoftware/chat.webtrc/src/ws"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"io"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path"
	"reflect"
	"regexp"
	"runtime/trace"
	"strings"
	"sync"
	"syscall"
	"time"

	"errors"
	vapm "github.com/oresoftware/chat.webtrc/src/common/apm"
	vhp "github.com/oresoftware/chat.webtrc/src/common/handle-panic"
	rest_conn "github.com/oresoftware/chat.webtrc/src/rest/conn" // Still needed for rest server conn counter
	"github.com/oresoftware/chat.webtrc/src/rest/rt-settings"    // Still needed for rt settings
	ws_conn "github.com/oresoftware/chat.webtrc/src/ws/conn"     // Still needed for wss conn counter
	"runtime/pprof"
)

/*

The server listens for WebSocket connToUserId on ws://localhost:8080/ws.
You can connect to this WebSocket using a WebSocket client in your preferred language.

*/

var cnfg = virl_conf.GetConf()

// Service struct to hold the WebSocket connToUserId and related data
// This struct is defined in src/ws/service.go

func registerRestRoutes(c *ctx.VibeCtx) (*mux.Router, error) {

	m := mux.NewRouter()

	m.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set("Access-Control-Allow-Origin", "*") // Replace "*" with your allowed origin(s)
			w.Header().Set("Access-Control-Allow-Headers", "*")
			next.ServeHTTP(w, r)
		})
	})

	m.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		if false {
			vbl.Stdout.Warn(vbl.Id("vid/ed567964db4c"), "hit canonical route, which is not supported.")
		}
		w.Write([]byte(`{"status":405,"result":"405: not supported / not allowed"}`))
	})

	m.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this handler doesn't seem to do its job which is my we have a PathPrefix handler after it
		if false {
			vbl.Stdout.Warn(vbl.Id("vid/62d33b520f5c"), "missing route:", r.Method, r.URL, r.Body, r.Header)
		}
		w.WriteHeader(501)
		w.Write([]byte(`{"status":501,"message":"501: Not implemented."}`))
	})

	r := m.PathPrefix("/chat").Subrouter()

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Methods", "*")
			w.Header().Set("Access-Control-Allow-Origin", "*") // Replace "*" with your allowed origin(s)
			w.Header().Set("Access-Control-Allow-Headers", "*")
			next.ServeHTTP(w, r)
		})
	})

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// TODO: change this to r.Context()
			muxctx.Set(r, "req-res-ctx", &mw.Ctx{
				Log: vbl.Stdout.Child(&map[string]interface{}{}),
			})
			defer func() {
				muxctx.Delete(r, "req-res-ctx")
				muxctx.Clear(r)
			}()
			next.ServeHTTP(w, r)
		})
	})

	r.Use(mw.AddStatusCodeWriter(c))
	r.Use(mw.LogInfoUponBadRequest(c))
	r.Use(mw.AsJSON())

	r.Use(mw.MakeRunParallel(c))
	r.Use(mw.Recovery(c))
	r.Use(mw.Tracer(c))
	r.Use(mw.BodyLogging(c))

	// if conf.GetConf().IN_PROD {
	//	throttleMiddleware, err := throttle.CreateThrottle("cp-overall")
	//	if err != nil {
	//		log.Fatal("Failed to create throttle middleware %s", err.Error())
	//	}
	//	r.Use(throttleMiddleware)
	// }

	originDomain := func(origin string) string {
		origin = strings.ToLower(origin)
		origin = strings.Replace(origin, "http://", "", -1)
		origin = strings.Replace(origin, "https://", "", -1)
		return origin
	}

	r.Use(mw.MakeOrigin(originDomain, c))

	r.Methods("GET").Path("/favicon.ico").HandlerFunc(
		mw.DoResponse(func(*mw.Ctx, http.ResponseWriter, *http.Request) (int, []byte) {
			return 200, []byte("ok")
		}),
	)

	r.PathPrefix("/").Methods("OPTIONS").Handler(
		mw.Options(originDomain, c),
	)

	r.Use(
		mw.MakeOptions(originDomain, c),
	)

	r.Use(
		mw.DebugOptions(),
	)

	// register all resource routes
	controller.RegisterRoutes(r, c)

	r.Path("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(405)
		vbl.Stdout.Warn(vbl.Id("vid/b3180a9645a6"), "hit canonical route, which is not supported.")
		w.Write([]byte(`{ "status": 405, "result": "405: not supported / not allowed" }`))
	})

	r.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this handler doesn't seem to do its job which is my we have a PathPrefix handler after
		if false {
			vbl.Stdout.Warn(vbl.Id("vid/af237065b202"), "missing route:", r.Method, r.URL, r.Body, r.Header)
		}
		w.WriteHeader(501)
		w.Write([]byte(`{"status":501,"message":"501: Not implemented."}`))
	})

	r.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this is a catch-all handler since the r.NotFoundHandler wasn't doing its job
		vbl.Stdout.Warn(vbl.Id("vid/ab12b4206298"), "missing route:", r.Method, r.URL, r.Body, r.Header)
		w.WriteHeader(501)
		w.Write([]byte(`{"status":501,"message":"501: Not implemented."}`))
	})

	return r, nil
}

var onceStdin = sync.Once{}

func listenToStdin() {
	cfg := virl_conf.GetConf()

	if !cfg.LISTEN_TO_STDIN {
		return
	}

	if cfg.IN_PROD {
		// TODO: listen to stdin?
		return
	}

	onceStdin.Do(func() {

		vbl.Stdout.Info(vbl.Id("vid/db913fc7d6cf"), "listening to stdin")

		go func() {
			reader := bufio.NewReader(os.Stdin)

			for {
				line, err := reader.ReadString('\n')

				if err == io.EOF {
					fmt.Println("EOF from stdin received")
					virl_cleanup.RunCleanUpFuncs(err)
					if os.Getenv("vibe_running_locally") == "yes" {
						os.Exit(0)
					}
					break
				}

				// Simplified: Just check the line content
				command := strings.TrimSpace(line)

				vbl.Stdout.Warn(vbl.Id("vid/07f817503d8d"), "message received from stdin:", command)

				if command == "rs" {
					vbl.Stdout.Info(vbl.Id("vid/9045439b6310"), "command 'rs' received from stdin, restarting server..")
					virl_cleanup.RunCleanUpFuncs(nil)
					os.Exit(0)
				}

				if command == "env" {
					vbl.Stdout.Info(vbl.Id("vid/daf601026499"), "command 'env' received from stdin, printing env..")
					vbl.Stdout.PrintEnv()
					vbl.Stdout.PrintEnvPlain()
					vbl.Stdout.Info(virl_conf.GetConf())
					continue
				}

				vbl.Stdout.Warn(
					"11ef8148-ede3-459b-acc1-c7213d2ceb5f",
					"unrecognized command received from stdin:",
					command,
				)
			}

		}()
	})
}

func ListenForSIGPIPE() {
	sigs := make(chan os.Signal, 1)

	// signal.Notify(sigs, syscall.SIGPIPE)

	go func() {
		sig := <-sigs
		fmt.Println("Received SIGPIPE:", sig)
		os.Exit(1)
		// Handle SIGPIPE here as needed
	}()

	// Keep the main process running for the signal to be caught
	// select {}
}

type MyHandler struct {
	// You can add any additional fields or configuration here
}

// ServeHTTP implements the http.Handler interface
func (h *MyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Your logic goes here
	fmt.Println("well fuck my life")
	fmt.Fprintf(w, "Hello, this is my custom handler!")
}

var doShutdownOnce = sync.Once{}

func controlledShutdown(wg *sync.WaitGroup) {

	doShutdownOnce.Do(func() {

		wg.Add(1)
		go func() {

			vbl.Stdout.Warn(vbl.Id("vid/d4327806e844"), "registering signal handlers.")
			interruptChannel := make(chan os.Signal, 1)
			// TODO: what is SIGQUIT?
			signal.Notify(interruptChannel, syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT)

			go func() {
				// Wait for the interrupt signal
				vbl.Stdout.Warn("37152de2-ad86-4655-9ff0-809ab1fb16ef", "waiting for interrupt (SIGINT/SIGTERM) signals..")
				<-interruptChannel
				vbl.Stdout.Warn(vbl.Id("vid/f850604ad48d"), "Signal received by process.")
				// Run cleanup code when the interrupt signal is received
				virl_cleanup.RunCleanUpFuncs(nil)
				// Exit the program
				os.Exit(0)
			}()

			wg.Done()
		}()

	})

}

func main() {

	fmt.Println("8d643771-bc9f-4610-bd29-481f9bb07a5d", "starting up...")

	// listen for signals, do controlled shutdown if signals can be handled
	var wg = sync.WaitGroup{}
	controlledShutdown(&wg)
	wg.Wait()

	defer func() {

		if r := vhp.HandlePanic("d0960f033843"); r != nil {
			vbl.Stdout.Warn("8f1f0aba-9ff2-40dc-96cb-a0d03c34f80f6", r)
		}
		// serves as a primary shutdown hook, as defer gets called when program exits
		virl_err.ErrorQueue.Iterate(func(item interface{}) {
			// iterate over all runtime errors that got saved/stored, in order to reference them easily later
			vbl.Stdout.Error("3a93b233-468b-4632-bba0-a6f2b81384b0", "stored runtime error:", item)
		})
		virl_cleanup.RunCleanUpFuncs(nil)
	}()

	virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
		vbl.Stdout.Warn(vbl.Id("vid/453fb4436057"), "running first cleanup func")
		wg.Done()
	})

	// wd can change during the process runtime, so always re-read it, I guess
	wd, err := os.Getwd()

	if err != nil {
		vbl.Stdout.Critical(vbu.ErrorFromArgs("8fc8f698-82e4-4cfa-a2b4-2c4e86dee802", fmt.Sprintf("error: %v", err)))
		virl_cleanup.RunCleanUpFuncs(err)
		os.Exit(1)
	}

	var kr = struct {
		value     string
		Key       string
		headers   interface{}
		Partition interface{}
	}{
		value:     string("123"),
		Key:       string("abc"),
		headers:   struct{ foo int }{3},
		Partition: struct{ foo struct{ bar int } }{},
	}

	// testing logging:
	vbl.Stdout.Trace("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b", "pwd:", wd)
	vbl.Stdout.Debug(vbl.Id("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b"), "pwd:", wd)
	vbl.Stdout.Info(vbl.Id("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b"), "pwd:", wd)
	vbl.Stdout.Warn(vbl.Id("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b"), "pwd:", wd)

	vbl.Stdout.Trace("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b", "kr:", kr)
	vbl.Stdout.Debug(vbl.Id("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b"), "kr:", kr)
	vbl.Stdout.Info(vbl.Id("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b"), "kr:", kr)
	vbl.Stdout.Warn(vbl.Id("zzb5ed3b-148f-48fe-bbeb-0a43f55f572b"), "kr:", kr)

	// virl_cleanup.RunCleanUpFuncs()
	// os.Exit(0)

	filePath := path.Join(wd, "go.mod")

	// Check if the file exists
	if _, err := os.Stat(filePath); err != nil {
		// File does not exist? something went wrong - bad
		vbl.Stdout.Critical(vbu.ErrorFromArgs("b1b4eece-c5fd-4aef-bf0a-8c2694683fa9", "File does not exist:", filePath))
		virl_cleanup.RunCleanUpFuncs(err)
		os.Exit(1)
	}

	logsPth := path.Join(wd, "logs")
	if err := os.MkdirAll(logsPth, os.ModePerm); err != nil {
		// Handle the error, e.g., log it or return an error
		vbl.Stdout.Critical("d22dc14e-31c5-4a50-85bc-80b190626cde", err)
		virl_cleanup.RunCleanUpFuncs(err)
		os.Exit(1)
	}

	if false { // Set to true to enable tracing
		// TODO: turn on/off tracing
		// run tracer thing
		traceFile := path.Join(logsPth, "trace.out")
		f, err := os.Create(traceFile)

		if err != nil {
			vbl.Stdout.Critical("874568dc-1ef8-4652-a105-5cf10a76a36f", err)
			virl_cleanup.RunCleanUpFuncs(err)
			os.Exit(1)
		}

		defer func() {
			if err := f.Close(); err != nil {
				vbl.Stdout.Error(vbl.Id("vid/62a99927e760"), err)
			}
		}()

		if err := trace.Start(f); err != nil {
			vbl.Stdout.Critical("203da426-5679-480c-a568-9ddc41a711ef", err)
			virl_cleanup.RunCleanUpFuncs(err)
			os.Exit(1)
		}

		defer func() {
			// trace is referenced in the meta/downloads handler
			trace.Stop()
		}()
	}

	if true { // Set to true to enable profiling
		// TODO: turn on/off profiling
		// run pprof thing (to discover slow i/o calls

		fmt.Println("here is is:", "93bcdb79-6fea-4c9c-a501-3c8708f5e11d")
		pprofPath := path.Join(logsPth, "cpu.pprof")
		f, err := os.Create(pprofPath)

		if err != nil {
			vbl.Stdout.Critical("db8e8265-bea4-43a5-afd5-6edf850b8a51", err)
			virl_cleanup.RunCleanUpFuncs(err)
			os.Exit(1)
		}

		if err := pprof.StartCPUProfile(f); err != nil {
			vbl.Stdout.Error(vbl.Id("vid/061a5feed2b6"), err)
			virl_cleanup.RunCleanUpFuncs(err)
			os.Exit(1)
		}

		defer func() {
			pprof.StopCPUProfile()
		}()

	}

	vbl.Stdout.Warn(
		vbl.Id("vid/495f94f1b034"),
		"running main()...",
	)

	vapm.SendTrace("Message body goes here")
	vapm.SendTrace("fec27dad-f060-4e86-9630-ef3b65ccd1ac", errors.New("fec27dad-f060-4e86-9630-ef3b65ccd1ac"))
	vapm.SendTrace("413e40c4-d72a-4542-93c6-fc75b5ea2f12", errors.New("test-fec27dad-f060-4e86-9630-ef3b65ccd1ac"))
	vapm.SendTrace("842e5696-aa6b-4b8c-b312-6208bc3bdea3", "fec27dad-f060-4e86-9630-ef3b65ccd1ac")

	cfg := virl_conf.GetConf()

	// Removed RabbitMQ connection setup

	// connectionString := "mongodb://mongodb-node1:27018,mongodb-node2:27019,mongodb-node3:27020/?replicaSet=myrs"
	mongoDbConnStr := cfg.MONGO_DB_URL_WO_PORT
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)

	clientOptions := options.Client().
		SetReadPreference(readpref.Primary()).
		ApplyURI(mongoDbConnStr).
		SetMinPoolSize(3).
		SetMaxPoolSize(7).
		SetServerAPIOptions(serverAPI)

	// Create a MongoDB client
	vbl.Stdout.Info(vbl.Id("vid/3f2b6afdd3b9"), "Connecting to MongoDB:", mongoDbConnStr, "...")
	mongoClient, err := mongo.Connect(context.Background(), clientOptions)

	if err != nil {
		vbl.Stdout.Critical(vbl.Id("vid/4aee198fbdd9"), "Error connecting to MongoDB:", err)
		os.Exit(1)
	}

	vbl.Stdout.Info(
		vbl.Id("vid/3e2b88282e96"),
		"Finished connecting to MongoDB:", mongoDbConnStr,
	)

	// Defer mongoClient disconnection
	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/a6ca9450b286"), err)
		}
	}()

	if err := mongoClient.Ping(context.Background(), nil); err != nil {
		vbl.Stdout.Critical(vbl.Id("vid/018fcc9f665a"), err)
		os.Exit(1)
	}

	database := mongoClient.Database(cfg.MONGO_DB_NAME)
	collection := database.Collection("vibe_chat_sequences")

	// Insert a document (for sequence generation, if needed)
	document := bson.D{
		{"SeqName", "vibe_chat_conv_seq"},
		{"SeqValue", 1},
	}

	_, err = collection.InsertOne(context.TODO(), document)
	if err != nil {
		// ignore this error in general (e.g., if it already exists)
		vbl.Stdout.Debug(vbl.Id("vid/28ae40931441"), err)
	}

	vbl.Stdout.Debug(
		vbl.Id("vid/f9e6e49ccc15"),
		"Sequence db seed doc inserted successfully (ignored if already exists)!",
	)

	virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
		defer wg.Done()
		vbl.Stdout.Info("e0cb9f9a-f990-48c9-9a3f-8ce81243b71a", "disconnecting from mongodb")
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			vbl.Stdout.Warn(vbl.Id("vid/e40e97a679bc"), err)
		}
	})

	// add default collections for mongo to memory
	if err := virl_mongo.AddDefaultCollections(*mongoClient); err != nil {
		vbl.Stdout.Critical(vbl.Id("vid/155696cfaa8a"), err)
		os.Exit(1)
	}

	// Create the Service instance (responsible for WebSocket and WebRTC handling)
	service := virl_ws.NewService(cfg)
	service.M = virl_mongo.NewVibeMongo( // Set the MongoDB client wrapper on the service
		cnfg.MONGO_DB_NAME,
		mongoClient,
		mongoClient.Database(cnfg.MONGO_DB_NAME),
	)

	// Removed RabbitMQ/Kafka/Redis setup

	// Start the WSS server (handling both WS and WebRTC signaling)
	go func() {
		rt.AddWait()

		// Start listening for WebSocket connections
		connCounter := ws_conn.ConnectionCounter{}

		s := &http.Server{
			Addr:           cfg.API_WSS_ADDRESS,
			Handler:        service, // This service handles WebSocket requests
			ReadTimeout:    20 * time.Second,
			WriteTimeout:   20 * time.Second,
			MaxHeaderBytes: 1 << 22,
			ConnState: func(conn net.Conn, state http.ConnState) {
				switch state {
				case http.StateNew: // Called when a new connection is created
					connCounter.Increase()
				case http.StateClosed, http.StateHijacked: // Called when the connection is closed or hijacked
					connCounter.Decrease()
				}
			},
		}

		l, err := net.Listen("tcp", cfg.API_WSS_ADDRESS)

		if err != nil {
			vbl.Stdout.Critical(vbl.Id("vid/d1256a7e1e6f"), err)
			virl_cleanup.RunCleanUpFuncs(err) // Clean up other resources before exit
			os.Exit(1)
		}

		virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
			defer wg.Done()
			vbl.Stdout.Warn("5e3beff6-73e5-46fc-aef4-e8379704c19", "virl cleanup - closing ws server/listener")
			if err := l.Close(); err != nil { // !! Stop listening for new connections
				vbl.Stdout.Error(vbl.Id("vid/15e553a2c85"), err)
			}
			// Use a context with timeout for graceful shutdown of active connections
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second) // Adjust timeout as needed
			defer cancelShutdown()
			if err := s.Shutdown(shutdownCtx); err != nil { // !! Close all current/existing connections gracefully
				vbl.Stdout.Error(vbl.Id("vid/584df4b6fb80"), fmt.Sprintf("Error shutting down WSS server: %v", err))
			}
		})

		vbl.Stdout.Info(
			vbl.Id("vid/39cf8d94b45e"),
			fmt.Sprintf("WSS Server listening on address: %s - %s", l.Addr().Network(), au.Col.BrightBlue(l.Addr().String())),
		)

		rt.AddDone()
		fmt.Println(`{"wss_server_state":"listening"}`) // print JSON for shell pipeline

		// Start serving requests
		if err := s.Serve(l); err != nil {
			if !errors.Is(err, http.ErrServerClosed) { // Expected error during shutdown
				vbl.Stdout.Error(vbl.Id("vid/b04cc81b6c48"), fmt.Sprintf("WSS server error: %v", err))
			}
			// Cleanup is handled by deferred RunCleanUpFuncs
			// os.Exit(1) // Removed explicit exit here, defer handles it
		}
	}()

	// Start the REST API server
	go func() {
		rt.AddWait()

		// Create a MongoDB client for the REST API context (or could reuse the main one, but original code had two)
		client, err := mongo.Connect(context.Background(), clientOptions)
		if err != nil {
			vbl.Stdout.Critical(vbl.Id("vid/5dbee7823206"), err)
			virl_cleanup.RunCleanUpFuncs(err) // Clean up other resources before exit
			os.Exit(1)
		}

		// Add cleanup for the REST MongoDB client
		virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
			defer wg.Done()
			vbl.Stdout.Info("rest_mongo_cleanup", "disconnecting from rest mongodb client")
			if err := client.Disconnect(context.Background()); err != nil {
				vbl.Stdout.Warn("vid/rest_mongo_disconnect_err", err)
			}
		})

		m := virl_mongo.NewVibeMongo(cfg.MONGO_DB_NAME, client, client.Database(cfg.MONGO_DB_NAME))
		c := ctx.NewVibeCtx(cfg, m,
			// Pass nil for removed broker pools
			nil, nil, nil, nil,
		)
		r, err := registerRestRoutes(c)

		if err != nil {
			vbl.Stdout.Critical(vbl.Id("vid/0c5b3cbd312b"), err)
			virl_cleanup.RunCleanUpFuncs(err) // Clean up other resources before exit
			os.Exit(1)
		}

		if err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			pathTemplate, err1 := route.GetPathTemplate()
			var queries, err2 = route.GetQueriesRegexp()
			var varNames, err3 = route.GetVarNames()

			if err1 == nil && err2 == nil && err3 == nil {
				var methods, _ = route.GetMethods()

				for _, m := range methods {
					vbl.Stdout.Info(
						vbl.Id("vid/fb25ef49812a"),
						"Mux route:", pathTemplate, "route name:", route.GetName(), m, queries, varNames,
					)
				}

			}
			// You can also print other information like methods, queries, etc.
			return nil
		}); err != nil {
			vbl.Stdout.Critical(vbl.Id("vid/ff8181c59e77"), err)
			virl_cleanup.RunCleanUpFuncs(err) // Clean up other resources before exit
			os.Exit(1)
		}

		connCounter := rest_conn.ConnectionCounter{}

		s := &http.Server{
			Addr:           cfg.API_SERVER_ADDRESS,
			Handler:        r, // This router handles REST API requests
			ReadTimeout:    20 * time.Second,
			WriteTimeout:   20 * time.Second,
			MaxHeaderBytes: 1 << 22,
			ConnState: func(conn net.Conn, state http.ConnState) {
				switch state {
				case http.StateNew: // Called when a new connection is created
					connCounter.Increase()
				case http.StateClosed, http.StateHijacked: // Called when the connection is closed or hijacked
					connCounter.Decrease()
				}
			},
		}

		l, err := net.Listen("tcp", cfg.API_SERVER_ADDRESS)

		if err != nil {
			vbl.Stdout.Critical("d0b9184a-5248-413e-a5a8-30fea66997f5:", err)
			virl_cleanup.RunCleanUpFuncs(err) // Clean up other resources before exit
			os.Exit(1)
		}

		if false { // This shutdown route seems like a test/debug feature
			r.HandleFunc("/shutdown", func(writer http.ResponseWriter, request *http.Request) {
				virl_cleanup.RunCleanUpFuncs(nil)
			})
		}

		virl_cleanup.AddCleanupFunc(func(wg *sync.WaitGroup) {
			defer wg.Done()
			vbl.Stdout.Warn("1f47f610-c72a-4f1f-a329-ff77ac26b95f", "virl cleanup - closing http server/listener")
			if err := l.Close(); err != nil { // !! Stop listening for new connections
				vbl.Stdout.Error(vbl.Id("vid/e5b85c17c335"), err)
			}
			// Use a context with timeout for graceful shutdown
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second) // Adjust timeout
			defer cancelShutdown()
			if err := s.Shutdown(shutdownCtx); err != nil { // !! Close all current/existing connections gracefully
				vbl.Stdout.Error(vbl.Id("vid/bf49e6297a24"), fmt.Sprintf("Error shutting down HTTP server: %v", err))
			}
		})

		vbl.Stdout.Info(
			vbl.Id("vid/d34be50925b2"),
			fmt.Sprintf("HTTP Server listening on address: %s - %s", l.Addr().Network(), au.Col.Blue(l.Addr().String())),
		)

		rt.AddDone()
		fmt.Println(`{"server_state":"listening"}`) // print JSON for shell pipeline

		vbl.Stdout.Info(vbl.Id("vid/ac8d91ac4713"), "config:", cfg)

		// Start serving requests
		if err := s.Serve(l); err != nil {
			if !errors.Is(err, http.ErrServerClosed) { // Expected error during shutdown
				vbl.Stdout.Warn(vbl.Id("vid/d26f011be27b"), fmt.Sprintf("HTTP server error: %v", err))
			}
			// Cleanup is handled by deferred RunCleanUpFuncs
			// os.Exit(1) // Removed explicit exit here
		}

	}()

	// start listening to stdin
	listenToStdin()

	// The main goroutine blocks here, waiting for a signal to exit or for goroutines to finish
	select {}
}
