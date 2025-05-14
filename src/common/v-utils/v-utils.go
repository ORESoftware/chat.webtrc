// source file path: ./src/common/v-utils/v-utils.go
package vbu

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/oresoftware/chat.webrtc/src/common/vibelog"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
)

var BgCtx = context.Background()

func flattenDeepInternal(args []interface{}, v reflect.Value) []interface{} {

	// if v.Kind() == reflect.Ptr {
	// 	v = v.Elem()
	// }

	if v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	if v.Kind() == reflect.Array || v.Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			args = flattenDeepInternal(args, v.Index(i))
		}
	} else {
		args = append(args, v.Interface())
	}

	return args
}

func RequireParam(r *http.Request, s string, val string) error {
	keys, ok := r.URL.Query()[s]

	// Query()["key"] will return an array of items,
	// we only want the single item.

	if !ok || len(keys[0]) < 1 {
		vbl.Stdout.Warn(vbl.Id("vid/68b2da108a48"), s)
		return ErrorFromArgs(
			"1014cee6-bb0e-4d74-ac07-5013cb5b1643",
			fmt.Sprintf("the request is missing the query param: '%s'", s),
		)
	}

	if keys[0] != val {
		vbl.Stdout.Warn(vbl.Id("vid/45d88af45109"), "Url param '%s' does not equal expected value '%s'", s, val)
		return ErrorFromArgs(
			"a9b54861-d4e1-4aa4-98d0-a788e02c88b7",
			fmt.Sprintf("the request has an expected value for query param: '%s', the value was: '%s'", s, keys[0]),
		)
	}

	return nil
}

func FlattenDeep(args ...interface{}) []interface{} {
	return flattenDeepInternal(nil, reflect.ValueOf(args))
}

func MustGetUuid(s string) (string, uuid.UUID) {
	var id, err = uuid.Parse(s)

	if err != nil {
		panic(ErrorFromArgs("131b027f-1916-40a8-8699-f5c5aa5d6021", fmt.Sprintf("Could not parse uuid from string: %s", s)))
	}

	if id.String() == "00000000-0000-0000-0000-000000000000" {
		panic(ErrorFromArgs("c639c337-cf97-43d5-8ac3-991095debfda", fmt.Sprintf("The uuid was all zeros: '%s' ...", id.String())))
	}

	return s, id
}

// JoinList joins strangs
func JoinList(strangs []string) string {
	buffer := bytes.NewBufferString("")
	for _, s := range strangs {
		buffer.WriteString(s + " ")
	}
	return buffer.String()
}

// JoinArgs joins strangs
func JoinArgs(strangs ...string) string {
	buffer := bytes.NewBufferString("")
	for _, s := range strangs {
		buffer.WriteString(s + " ")
	}
	return buffer.String()
}

type VibeError struct {
	Code    int
	LogId   string
	Message string
}

func (v *VibeError) Error() string {
	if z, err := json.Marshal(v); err != nil {
		return fmt.Sprintf("Code:'%v', LogId:'%s', Message:'%s'", v.Code, v.LogId, v.Message)
	} else {
		return string(z)
	}
}

// ErrorFromArgs joins strangs
func ErrorFromArgsWithCode(id string, code int, strangs ...string) *VibeError {
	buffer := bytes.NewBufferString("")
	for _, s := range strangs {
		buffer.WriteString(s + " ")
	}
	return &VibeError{
		Code:    code,
		LogId:   id,
		Message: buffer.String(),
	}
}

type Stringer interface {
	String() string
}

type ToString interface {
	ToString() string
}

func GetInspectedVal(v interface{}) *map[string]interface{} {

	outResult := make(map[string]interface{})
	var rv = reflect.ValueOf(v)

	var errStr = ""
	var toString = ""

	var typeStr = fmt.Sprintf("%T", v)

	if rv.IsValid() && rv.CanInterface() {
		z := rv.Type().String()
		if z != typeStr {
			typeStr = fmt.Sprintf("(%T / %v)", v, z)
		}
	}

	if z, ok := v.(*error); ok {
		errStr = (*z).Error()
	}

	if z, ok := v.(error); ok {
		errStr = z.Error()
	}

	if z, ok := v.(Stringer); ok {
		toString = z.String()
	}

	if z, ok := v.(*Stringer); ok {
		toString = (*z).String()
	}

	if typeStr != "" {
		outResult["+(GoType):"] = typeStr
	}

	if errStr != "" {
		outResult["+(ErrStr):"] = errStr
	}

	if toString != "" && toString != errStr {
		outResult["+(ToStr):"] = toString
	}

	outResult["+(Val):"] = v

	return &outResult
}

// ErrorFromArgs joins strangs
func ErrorFromArgsList(id string, strangs []string) *VibeError {
	buffer := bytes.NewBufferString("")
	for _, s := range strangs {
		buffer.WriteString(s + " ")
	}
	var msg = strings.TrimSpace(buffer.String())
	vbl.Stdout.Warn(id, "trace-id:aa4f73dd-054d-4b6d-8519-6d19c85dbae4", msg)
	return &VibeError{
		Code:    -1,
		LogId:   id,
		Message: msg,
	}
}

// ErrorFromArgs joins strings and stuff
func ErrorFromArgs(id string, s string, strangs ...string) *VibeError {
	buffer := bytes.NewBufferString(s)
	for _, s := range strangs {
		buffer.WriteString(s + " ")
	}
	return &VibeError{
		Code:    -1,
		LogId:   id,
		Message: strings.TrimSpace(buffer.String()),
	}
}

// ErrorF does good stuff
func ErrorF(id string, s string, rest ...interface{}) *VibeError {
	msg := fmt.Sprintf(s, rest...)
	return &VibeError{
		Code:    -1,
		LogId:   id,
		Message: strings.TrimSpace(msg),
	}
}

func GetLocalStackTrace() string {
	// this helper func only prints the stack trace for files within vibeirl repos
	// which is very helpful for creating stack traces that you actually want to sift through
	var b strings.Builder
	stackScanner := bufio.NewScanner(bytes.NewReader(debug.Stack()))
	for stackScanner.Scan() {
		line := stackScanner.Text()
		// do your processing here to determine whether to prin the line
		if strings.Index(string(line), "vibeirl") >= 0 {
			// log.Error("stack trace line", i, ": ",v)
			fmt.Fprintf(&b, "%s\n", line)
		}
	}
	return b.String()
}

func IsWhitespace(b []byte) bool {
	// TrimSpace removes leading and trailing white space.
	return len(bytes.TrimSpace(b)) == 0
}

type ErrVal[T any] struct {
	Error error
	Val   T
}

func removeAdjacentDuplicates(slice []string) []string {

	if len(slice) == 0 {
		return []string{}
	}

	result := []string{slice[0]} // Start with the first element

	for i := 1; i < len(slice); i++ {
		if slice[i] != slice[i-1] {
			result = append(result, slice[i])
		}
	}

	return result
}

func removeAdjacentDupes(list [][]string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, sublist := range list {
		for _, item := range sublist {
			if _, exists := seen[item]; !exists {
				seen[item] = true
				result = append(result, item)
			}
		}
	}

	return result
}

// func ReadJWT_Old(tokenString string) (*jwt.MapClaims, error) {
//
//   // tokenString := "your.jwt.token.here"  // Replace with your JWT
//   // Extract the public key from private key
//   publicKey := "" // &privateKey.PublicKey
//
//   // Parse, decode, and verify the JWT
//   token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
//     // Make sure the algorithm used to sign is what you expect
//     if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
//       return nil, fmt.Errorf("vid/478aeb6e48fc: unexpected signing method: %v", token.Header["alg"])
//     }
//     return publicKey, nil
//   })
//
//   if err != nil {
//     vbl.Stdout.ErrorF("vid/0ab809b2f9d6: Error verifying JWT: %v", err)
//     return nil, err
//   }
//
//   if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
//
//     if err := token.Claims.Valid(); err != nil {
//       vbl.Stdout.Warn(vbl.Id("vid/e9474eacf9a6"), err)
//       return nil, err
//     }
//
//     return &claims, nil
//   } else {
//     return nil, fmt.Errorf("vid/fb659c71a999: could not validate token")
//   }
// }
//
// func DecryptJWT(jwtToken2 string) {
//   // Your JWT token
//   jwtToken := "your.jwt.token"
//
//   // Your private key (for RSA, it should be *rsa.PrivateKey)
//   privateKey := getYourPrivateKey()
//
//   // Parse takes the token string and a function for looking up the key
//   token, err := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
//     // Don't forget to validate the alg is what you expect:
//     if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
//       return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
//     }
//
//     return privateKey, nil
//   })
//
//   if err != nil {
//     log.Fatal(vbl.Id("vid/5d5480799815"), err)
//   }
//
//   if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
//     // Now you can use claims
//     vbl.Stdout.Info(vbl.Id("vid/067a9afd14f7"), claims)
//   } else {
//     log.Fatal("Invalid JWT Token")
//   }
//
// }

var InProd = os.Getenv("vibe_in_prod") == "yes"

func GetFilteredStacktrace() []string {
	if true || InProd {
		// we should return new string, just for immutability sake
		return []string{}
	}
	return GetNewStacktraceFrom([]string{})
}

func GetNewStacktraceFrom(x []string) []string {

	if true || InProd {
		// we should return new string, just for immutability sake
		return []string{}
	}

	// Capture the stack trace
	buf := make([]byte, 2049)
	n := runtime.Stack(buf, true)
	stackTrace := string(buf[:n])

	// Filter the stack trace
	lines := strings.Split(stackTrace, "\n")

	if len(lines) > 4 {
		// TODO: always 5 lines?
		lines = lines[5:]
		// lines = lines[:len(lines)-2]
	}

	filteredLines := []string{}
	for _, line := range lines {

		if strings.Contains(line, "/go/libexec/src/") {
			continue
		}
		if !strings.Contains(line, ".go:") {
			continue
		}
		if !strings.Contains(line, "/src/") {
			continue
		}
		if strings.Contains(line, "/go/src/") {
			continue
		}

		filteredLines = append(filteredLines, strings.TrimSpace(line))
	}

	return removeAdjacentDupes([][]string{
		x,
		filteredLines,
	})
}

func IsURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func GetObjectId(val interface{}) (*primitive.ObjectID, error) {

	if v, ok := val.(primitive.ObjectID); ok {
		if !v.IsZero() {
			return &v, nil
		}
	}

	if v, ok := val.(*primitive.ObjectID); ok {
		if !v.IsZero() {
			return v, nil
		}
	}

	if v, ok := val.(string); ok {
		if x, err := primitive.ObjectIDFromHex(v); true {
			if err != nil {
				return nil, ErrorF("08a16dfa-ac00-499f-9154-898e2c6bb069", fmt.Sprintf("%v", err))
			}
			if x.IsZero() {
				return nil, ErrorF("6b2e6ac9-66ac-4fbf-a937-afd8db45734a", "zero obj id")
			}
			return &x, nil
		}
	}

	return nil, ErrorF("f46295a8-d291-4fe6-bbc6-4e8a42cc9a95", "Could not retrieve objectId")
}

func MustGetObjectId(val interface{}) *primitive.ObjectID {

	var oid, err = GetObjectId(val)

	if err != nil {
		vbl.Stdout.Warn(vbl.Id("vid/c79ecbdeb132"), err)
	}

	if oid == nil {
		panic(JoinArgs("03476c09-d1e3-462f-9077-648a2950854b", "could not get object id"))
	}

	return oid
}

func GetTimeFromStr(val interface{}) (*time.Time, error) {

	layout := "2006-01-02T15:04:05.000Z"

	if s, ok := val.(string); ok {
		t, err := time.Parse(layout, s)
		if err != nil {
			return nil, err // Directly return nil and the error
		}
		return &t, nil // Only return a pointer to t if parsing was successful
	}

	return nil, ErrorF("d63de6ac-8d3f-468c-9803-df1017a8cdcb", "could not parse time.")
}

func HandleErrFirst[T any](v T, err error) *ErrVal[T] {
	return &ErrVal[T]{err, v}
}

func DoesNotEqualAny(str string, matches []string) bool {
	for _, m := range matches {
		if str == m {
			return false
		}
	}
	return true
}

func DoesNotStartWithAny(str string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(str, prefix) {
			return false
		}
	}
	return true
}

func PrintLocalStackTrace() {
	// this helper func only prints the stack trace for files within vibeirl repos
	// which is very helpful for creating stack traces that you actually want to sift through
	stackScanner := bufio.NewScanner(bytes.NewReader(debug.Stack()))
	for stackScanner.Scan() {
		line := stackScanner.Text()
		// do your processing here to determine whether to prin the line
		if strings.Index(string(line), "vibeirl") >= 0 {
			// log.Error("stack trace line", i, ": ",v)
			fmt.Println(line)
		}
	}
}

func Wait[T any](f func(c chan T)) T {
	c := make(chan T, 1)
	f(c)
	return <-c
}
