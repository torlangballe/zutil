//go:build server

package zrest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
)

var (
	RunningOnServer      bool
	LegalCORSOrigins     []string
	CurrentInRequests    int
	StaticFolderPathFunc = func(add string) string {
		return "www"
	}
	HasTelemetryFunc     = func() bool { return false }
	WrapForTelemetryFunc func(handlerName string, handlerFunc http.HandlerFunc) http.HandlerFunc
)

func AddLegalCORSAddress(add string) {
	LegalCORSOrigins = append(LegalCORSOrigins, add)
}

// Adds CORS headers to response if appropriate.
func AddCORSHeaders(w http.ResponseWriter, req *http.Request) {
	o := req.Header.Get("Origin")
	// fmt.Println("AddCorsHeaders1:", o, req.URL.String())
	if o == "" {
		return
	}
	u, err := url.Parse(o)
	find := o
	if err != nil {
		u.Host = u.Hostname()
		find = u.String()
	}
	// zlog.Info("AddCorsHeaders:", o, find, req.URL.String(), "allowed:", LegalCORSOrigins, zstr.IndexOf(find, LegalCORSOrigins))
	if zstr.StringsContain(LegalCORSOrigins, find) {
		// zlog.Info("AddCorsHeaders2:", o, "allowed:", LegalCORSOrigins)
		w.Header().Set("Access-Control-Allow-Origin", o)
		// w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "*") // "Origin, Content-Type, Accept, Access-Token, X-Date, X-Timeout-Secs "
		return
		// } else {
		// 	zlog.Info("AddCorsHeaders NOT allowed!:", o, find, LegalCORSOrigins)
	}
}

// Returns HTTP error code and error messages in JSON representation, with string made of args and, printed
func ReturnAndPrintError(w http.ResponseWriter, req *http.Request, errorCode int, a ...interface{}) error {
	str := fmt.Sprintln(a...)
	zlog.ErrorAtStack(5, a...)
	ReturnError(w, req, str, errorCode)
	return errors.New(str)
}

// Returns HTTP error code and error messages in JSON representation.
func ReturnError(w http.ResponseWriter, req *http.Request, message string, errorCode int) {
	w.WriteHeader(errorCode)
	ReturnDict(w, req, zdict.Dict{"error": message})
}

// Returns {"somekey":<some interface{}>}.
func ReturnSingle(w http.ResponseWriter, req *http.Request, key string, val interface{}) {
	ReturnDict(w, req, zdict.Dict{key: val})
}

func ReturnDict(w http.ResponseWriter, req *http.Request, dict zdict.Dict) {
	data, _ := json.Marshal(dict)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Date", time.Now().In(time.UTC).Format(time.RFC3339))
	AddCORSHeaders(w, req)
	w.Write(data)
}

func GetBoolVal(vals url.Values, name string) bool {
	str := vals.Get(name)
	return zbool.FromString(str, false)
}

func GetIntVal(vals url.Values, name string, def int) int {
	return int(GetInt64Val(vals, name, int64(def)))
}

func GetInt64Val(vals url.Values, name string, def int64) int64 {
	s := vals.Get(name)
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return def
	}
	return n
}

func GetTimeVal(vals url.Values, name string) time.Time {
	s := vals.Get(name)
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse(time.RFC3339Nano, s)
		if err != nil {
			zlog.Error(s, err)
		}
	}
	return t
}

// The FuncHandler type is a special request handler function that is a http.Handler by having a ServeHTTP method that calls itself
type FuncHandler func(http.ResponseWriter, *http.Request)

func (f FuncHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	CurrentInRequests++
	f(w, req)
	CurrentInRequests--
}

func GetFloatVal(vals url.Values, name string, def float64) float64 {
	s := vals.Get(name)
	if s == "" {
		return def
	}
	n, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return n
}

func AddSubHandler(router *mux.Router, pattern string, h http.Handler) *mux.Route {
	pattern = strings.TrimRight(AppURLPrefix+pattern, "/")
	defer zlog.HandlePanic(false)
	if router == nil {
		// zlog.Info("AddSubHandler no router:", pattern)
		if HasTelemetryFunc() {
			thandler := router.HandleFunc(pattern, WrapForTelemetryFunc(pattern, h.ServeHTTP))
			http.Handle(pattern, thandler.GetHandler())
		} else {
			http.Handle(pattern, h)
		}
		return nil
	}
	// TODO: Add Telemetry!!!
	route := router.PathPrefix(pattern)
	// zlog.Info("zrest.AddSubHandler:", pattern)
	r := route.Handler(h)
	return r
}

// AddFileHandler adds a file serving handler, which removes the pattern path prefix before creating the filepath.
// It uses a FuncHandler function that is its own http.Handler.
// It calls override (if override != nil) before serving the file, this can manipulate the corresponding filepath,
// or just be used for observing perposes.
func AddFileHandler(router *mux.Router, pattern, dir string, override func(w http.ResponseWriter, filepath *string, urlpath string, req *http.Request) bool) *mux.Route {
	handlerFunc := func(w http.ResponseWriter, req *http.Request) {
		// zlog.Info("AddFileHandler got:", req.URL)
		var path string
		if zstr.HasPrefix(req.URL.Path, AppURLPrefix+pattern, &path) {
			filepath := filepath.Join(dir, path)
			if override != nil {
				if override(w, &filepath, path, req) {
					return
				}
			}
			http.ServeFile(w, req, filepath)
			return
		}
		zlog.Error("no correct dir for serving:", req.URL.Path, dir, pattern)
	}
	if HasTelemetryFunc() {
		return AddSubHandler(router, pattern, WrapForTelemetryFunc(pattern, handlerFunc))
	}
	// zlog.Info("AddFileHandler add:", pattern)
	return AddSubHandler(router, pattern, FuncHandler(handlerFunc))
}

func AddHandler(router *mux.Router, pattern string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	pattern = AppURLPrefix + pattern
	// zlog.Info("AddHandler:", pattern)
	defer zlog.HandlePanic(false)
	// if router == nil {
	// 	if HasTelemetryFunc() {
	// 		http.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
	// 			router.HandleFunc(pattern, WrapForTelemetryFunc(pattern, f)) // router is zero!!??
	// 		})
	// 	} else {
	// 		router.HandleFunc(pattern, f)
	// 	}
	// 	return nil
	// }
	if HasTelemetryFunc() {
		return router.HandleFunc(pattern, WrapForTelemetryFunc(pattern, f))
	}
	return router.HandleFunc(pattern, f)
}

func Handle(pattern string, handler http.Handler) {
	spath := path.Join(AppURLPrefix, pattern)
	spath += "/"
	// zlog.Info("zrest.Handle:", spath, pattern)
	p := zprocess.PushProcess(30, "Handle:"+pattern)
	CurrentInRequests++
	http.Handle(spath, handler)
	zprocess.PopProcess(p)
	CurrentInRequests--
}

func SetProfilingHandler(router *mux.Router) {
	for _, name := range zdebug.AllProfileTypes {
		path := zdebug.ProfilingURLPrefix + name
		AddSubHandler(router, path, pprof.Handler(name))
	}
}
