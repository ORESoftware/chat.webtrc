package vbcf

import (
	"encoding/json"
	"errors"
	"fmt"
	au "github.com/oresoftware/chat.webtrc/src/common/v-aurora"
	vutils "github.com/oresoftware/chat.webtrc/src/common/v-utils"
	"github.com/oresoftware/chat.webtrc/src/common/vibelog"
	"log"
	"os"
	"path"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/oresoftware/cmd-line-parser/v1/clp"
	"github.com/pion/webrtc/v3" // Import Pion for ICEServer struct
)

var ProcUUID = uuid.New()

func mustGetWorkingDir() string {
	var wd, err = os.Getwd()
	if err != nil {
		log.Fatal("ad9556ac-8e43-48d2-8a29-6a8665bafc20:", "Could not get working dir:", err)
	}
	return wd
}

var WorkingDir = mustGetWorkingDir()
var ProjectRoot = path.Join(WorkingDir, getProjectRootDir(WorkingDir, "."))

func getProjectRootDir(cwd string, currentPath string) string {
	for true {
		p := path.Join(cwd, currentPath)
		s, err := os.Stat(path.Join(p, "go.mod"))
		if err != nil {
			if p == "/" || p == currentPath {
				log.Fatal("Could not walk to project root.")
			}
			currentPath = "../" + currentPath
			continue
		}
		if s.IsDir() {
			log.Fatal("go.mod is a dir?")
		}
		if p == "/" || p == currentPath {
			log.Fatal("Could not walk to project root.")
		}
		return currentPath
	}
	panic(vutils.ErrorFromArgs("afa252ee-469b-479f-8fbe-1d3abdf65075", "project root could not be discovered."))
}

// ConfigVars holds all configuration settings
type ConfigVars struct {
	// meta
	STACK_TRACES      bool
	FULL_STACK_TRACES bool
	USE_SSL           bool
	IN_PROD           bool

	API_SERVER_PROTOCOL string
	API_SERVER_PORT     int64
	API_SERVER_HOST     string

	APM_API_URL string

	API_WSS_PROTOCOL string
	API_WSS_PORT     int64
	API_WSS_HOST     string

	// mongo
	MONGO_PORT           int64
	MONGO_HOST           string
	MONGO_DB_NAME        string
	MONGO_PROTOCOL       string
	MONGO_DB_FULL_URL    string // computed prop
	MONGO_DB_URL_WO_PORT string // computed prop

	// WebRTC
	WEB_RTC_STUN_SERVERS []string           // List of STUN server URLs
	WEB_RTC_TURN_SERVERS []string           // List of TURN server URLs (e.g., turn:user:pass@host:port)
	WebRTCIceServers     []webrtc.ICEServer // Parsed ICE servers for Pion

	// stack traces
	ALLOW_LOGIN_AS_ANYONE bool

	API_SERVER_FULL_ADDRESS string // computed prop
	API_SERVER_ADDRESS      string // computed prop
	API_WSS_FULL_ADDRESS    string // computed prop
	API_WSS_ADDRESS         string // computed prop
	LISTEN_TO_STDIN         bool

	IS_DEBUG  bool
	LOG_LEVEL string
}

var c = clp.NewCmdParser()

var vars = ConfigVars{
	IS_DEBUG:  c.GetBool(false, "vibe_is_debug", c.Flags("--debug"), "set app to debug mode (more logs etc)"),
	LOG_LEVEL: c.GetString("info", "vibe_log_level", c.Flags("--log-level"), "app log level - 6 levels -> (trace,debug,info,warn,error,critical)"),

	USE_SSL: c.GetBool(false, "vibe_use_ssl", c.Flags("--use-ssl"), "Use SSL"),

	STACK_TRACES:      c.GetBool(true, "vibe_with_stack_traces", c.Flags("--use-stack-traces"), "Use stack traces"),
	FULL_STACK_TRACES: c.GetBool(true, "vibe_with_full_stack_traces", c.Flags("--use-full-stack-traces"), "Use full stack traces"),

	API_SERVER_PROTOCOL: c.GetString("http", "vibe_api_server_protocol", c.Flags("--api-server-protocol"), "api http server protocol"),
	API_SERVER_PORT:     c.GetInt(3000, "vibe_api_server_port", c.Flags("--api-server-port"), "port for rest api server"),
	API_SERVER_HOST:     c.GetString("localhost", "vibe_api_server_host", c.Flags("--api-server-host"), "api http server host"),

	API_WSS_PROTOCOL: c.GetString("ws", "vibe_wss_protocol", c.Flags("--api-wss-protocol"), "wss protocol"),
	API_WSS_PORT:     c.GetInt(3001, "vibe_wss_port", c.Flags("--api-wss-port"), "port for wss"),
	API_WSS_HOST:     c.GetString("localhost", "vibe_wss_host", c.Flags("--api-wss-host"), "wss host"),

	// mongo
	MONGO_PORT:        c.GetInt(27017, "vibe_mongo_port", c.Flags("--mongo-port"), "Port for mongodb"),
	MONGO_PROTOCOL:    c.GetString("mongodb", "vibe_mongo_protocol", c.Flags("--mongo-protocol"), "mongo connection protocol"),
	MONGO_HOST:        c.GetString("0.0.0.0", "vibe_mongo_host", c.Flags("--mongo-host"), "mongo connection host"),
	MONGO_DB_NAME:     c.GetString("", "vibe_mongo_db_name", c.Flags("--mongo-db-name"), "name of mongo db to use"),
	MONGO_DB_FULL_URL: "", // leave empty, computed property

	// WebRTC
	WEB_RTC_STUN_SERVERS: c.GetStringSlice([]string{"stun:stun.l.google.com:19302"}, "vibe_webrtc_stun_servers", c.Flags("--webrtc-stun-servers"), "list of STUN servers for WebRTC (comma-separated)"),
	WEB_RTC_TURN_SERVERS: c.GetStringSlice([]string{}, "vibe_webrtc_turn_servers", c.Flags("--webrtc-turn-servers"), "list of TURN servers for WebRTC (comma-separated, e.g., turn:user:pass@host:port)"),
	WebRTCIceServers:     nil, // Initialized in GetConf

	IN_PROD:               c.GetBool(false, "vibe_in_prod", c.Flags("--prod"), "for production"),
	ALLOW_LOGIN_AS_ANYONE: false,

	API_SERVER_FULL_ADDRESS: "",

	LISTEN_TO_STDIN: c.GetBool(false, "vibe_listen_to_stdin", c.Flags("--stdin"), "read from stdin stream"),

	APM_API_URL: c.GetString("api.vibeirl.com/apm", "vibe_apm_url", c.Flags("--apm-url"), "url for apm server"),
}

func FlipStackTraces() bool {
	vars.STACK_TRACES = !vars.STACK_TRACES
	return vars.STACK_TRACES
}

func FlipFullStackTraces() bool {
	vars.FULL_STACK_TRACES = !vars.FULL_STACK_TRACES
	return vars.FULL_STACK_TRACES
}

func getTypeString(_type string, v interface{}) string {
	if _type == "string" {
		return "'" + au.Col.Green(v).String() + "'"
	}
	if _type == "int" || _type == "int64" {
		return au.Col.Bold(au.Col.Yellow(v)).String()
	}
	if _type == "bool" {
		return au.Col.Bold(au.Col.Blue(v)).String()
	}
	if _type == "[]string" || _type == "[]webrtc.ICEServer" {
		// For slices, print a representation or a shortened one
		vVal := reflect.ValueOf(v)
		if vVal.Kind() == reflect.Slice {
			if vVal.Len() > 5 { // Limit output for long slices
				return fmt.Sprintf("[%v, ... (%d more)]", vVal.Slice(0, 5), vVal.Len()-5)
			}
		}
		// Use a simplified representation for slices of structs like ICEServer
		if _type == "[]webrtc.ICEServer" {
			// Cannot easily print ICEServer structs directly as strings without reflection/formatting
			// Let's just indicate the count and type
			vVal := reflect.ValueOf(v)
			if vVal.Kind() == reflect.Slice {
				return fmt.Sprintf("[]webrtc.ICEServer (%d items)", vVal.Len())
			}
		}

		return au.Col.Bold(au.Col.Blue(v)).String() // Fallback for other slices
	}

	vbl.Stdout.Warn(vbl.Id("vid/6481612295eb"), _type)
	return au.Col.Bold(au.Col.BrightRed(v)).String()
}

var lck = sync.Mutex{}
var parsed = false

func GetConf() *ConfigVars {
	lck.Lock()
	defer lck.Unlock() // Use defer to ensure unlock

	if parsed {
		return &vars
	}

	parsed = true

	if c.ParseBool(os.Getenv("vibe_help")) || (len(c.FlagsMap["--help"]) > 0 && c.ParseBoolOptimistic(c.FlagsMap["--help"][0])) {
		vbl.Stdout.Info(vbl.Id("vid/a0508bef6163"), "Help requested, logging config info.")
		c.PrintHelp()
		os.Exit(1)
	}

	vbl.Stdout.Info(
		vbl.Id("vid/2afa74348870"),
		"Starting vibe chat service, for help use --help flag or vibe_help=true env var.",
	)

	// Make computed properties
	vars.API_SERVER_FULL_ADDRESS = fmt.Sprintf("%s://%s:%v",
		vars.API_SERVER_PROTOCOL,
		vars.API_SERVER_HOST,
		vars.API_SERVER_PORT,
	)
	vars.API_SERVER_ADDRESS = fmt.Sprintf("%s:%v", vars.API_SERVER_HOST, vars.API_SERVER_PORT)

	vars.API_WSS_FULL_ADDRESS = fmt.Sprintf("%s://%s:%v", vars.API_WSS_PROTOCOL, vars.API_WSS_HOST, vars.API_WSS_PORT)
	vars.API_WSS_ADDRESS = fmt.Sprintf("%s:%v", vars.API_WSS_HOST, vars.API_WSS_PORT)

	// MongoDB URL computation (simplified, assuming no auth or specific options needed in URL without user/pass)
	if os.Getenv("vibe_running_locally") == "yes" {
		vars.MONGO_DB_URL_WO_PORT = fmt.Sprintf("%s://%s/%s",
			vars.MONGO_PROTOCOL,
			vars.MONGO_HOST,
			vars.MONGO_DB_NAME,
		)
		vars.MONGO_DB_FULL_URL = fmt.Sprintf("%s://%s:%v/%s",
			vars.MONGO_PROTOCOL,
			vars.MONGO_HOST,
			vars.MONGO_PORT,
			vars.MONGO_DB_NAME,
		)
	} else {
		// Add user/pass back here if your production MongoDB requires it
		vars.MONGO_DB_URL_WO_PORT = fmt.Sprintf("%s://%s/%s?retryWrites=true&w=majority",
			vars.MONGO_PROTOCOL,
			vars.MONGO_HOST,
			vars.MONGO_DB_NAME,
		)
		vars.MONGO_DB_FULL_URL = fmt.Sprintf("%s://%s:%v/%s?retryWrites=true&w=majority",
			vars.MONGO_PROTOCOL,
			vars.MONGO_HOST,
			vars.MONGO_PORT,
			vars.MONGO_DB_NAME,
		)
	}

	// Parse TURN servers if provided (e.g., "turn:user:pass@host:port")
	var iceServers []webrtc.ICEServer
	for _, url := range vars.WEB_RTC_STUN_SERVERS {
		iceServers = append(iceServers, webrtc.ICEServer{URLs: []string{url}})
	}
	for _, url := range vars.WEB_RTC_TURN_SERVERS {
		parts := strings.SplitN(url, ":", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "turn") {
			credsAndHost := strings.Split(parts[1], "@")
			if len(credsAndHost) == 2 {
				userPass := strings.Split(credsAndHost[0], ":")
				if len(userPass) == 2 {
					iceServers = append(iceServers, webrtc.ICEServer{
						URLs:       []string{url},
						Username:   userPass[0],
						Credential: userPass[1],
					})
				} else {
					vbl.Stdout.Warn("Invalid TURN server credentials format (expected user:pass@host:port):", url)
				}
			} else {
				vbl.Stdout.Warn("Invalid TURN server format (expected user:pass@host:port):", url)
			}
		} else {
			vbl.Stdout.Warn("Invalid STUN/TURN server format (should start with stun: or turn:):", url)
		}
	}
	vars.WebRTCIceServers = iceServers

	// Log all configured variables
	s := reflect.ValueOf(&vars).Elem()
	typeOfT := s.Type()

	rgx := regexp.MustCompile("pwd|password") // Don't log passwords

	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		n := typeOfT.Field(i).Name
		if rgx.MatchString(strings.ToLower(n)) {
			continue
		}
		log.Printf("%s (%s) = %v", n, au.Col.BrightBlue(f.Type()), getTypeString(f.Type().Name(), f.Interface()))
	}

	return &vars
}

// GetWebRTCIceServers exposes the parsed ICE servers for Pion configuration.
func (c *ConfigVars) GetWebRTCIceServers() []webrtc.ICEServer {
	return c.WebRTCIceServers
}
