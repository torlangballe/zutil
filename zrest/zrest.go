package zrest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zstr"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
)

var (
	RunningOnServer  bool
	AppURLPrefix     = "/"
	LegalCORSOrigins = map[string]bool{}
)

// Adds CORS headers to response if appropriate.
func AddCORSHeaders(w http.ResponseWriter, req *http.Request) {
	o := req.Header.Get("Origin")
	obase := zstr.HeadUntilLast(o, ":", nil)
	// zlog.Info("AddCorsHeaders:", o, obase, "allowed:", LegalCORSOrigins)
	if LegalCORSOrigins[o] || LegalCORSOrigins[obase] {
		// zlog.Info("AddCorsHeaders2:", o, "allowed:", LegalCORSOrigins)
		w.Header().Set("Access-Control-Allow-Origin", o)
		// w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, ZRPC-Client-Id, X-TimeZone-Offset-Hours, X-Requested-With, Content-Type, Accept, Access-Token")
		return
	}
}

// Returns HTTP error code and error messages in JSON representation, with string made of args and, printed
func ReturnAndPrintError(w http.ResponseWriter, req *http.Request, errorCode int, a ...interface{}) {
	str := fmt.Sprintln(a...)
	zlog.ErrorAtStack(nil, 5, a...)
	ReturnError(w, req, str, errorCode)
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

func GetTimeZoneFromRequest(req *http.Request) *time.Location {
	soff := req.Header.Get("X-TimeZone-Offset-Hours")
	if soff == "" {
		soff = req.URL.Query().Get("zoffset")
	}
	if soff != "" {
		offset, err := strconv.ParseInt(soff, 10, 32)
		if err != nil {
			zlog.Info("zrest.GetTimeZoneFromRequest bad offset:", soff, err)
			return nil
		}
		name := fmt.Sprintf("UTC%+d", offset)
		loc := time.FixedZone(name, int(offset)*3600)
		return loc
	}
	return nil
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
			zlog.Error(err, s)
		}
	}
	return t
}

// The FuncHandler type is a special request handler function that is a http.Handler by having a ServeHTTP method that calls itself
type FuncHandler func(http.ResponseWriter, *http.Request)

func (f FuncHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	f(w, req)
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
	// zlog.Info("zrest.AddSubHandler:", pattern)
	defer zlog.HandlePanic(false)
	if router == nil {
		http.Handle(pattern, h)
		return nil
	}
	route := router.PathPrefix(pattern)
	return route.Handler(h)
}

// AddFileHandler adds a file serving handler, which removes the pattern path prefix before creating the filepath.
// It uses a FuncHandler function that is it's own http.Handler
func AddFileHandler(router *mux.Router, pattern, dir string, peek func(filepath, urlpath string, req *http.Request)) *mux.Route {
	return AddSubHandler(router, pattern, FuncHandler(func(w http.ResponseWriter, req *http.Request) {
		var path string
		if zstr.HasPrefix(req.URL.Path, AppURLPrefix+pattern, &path) {
			filepath := filepath.Join(dir, path)
			// str, err := zfile.ReadStringFromFile(filepath)
			// zlog.OnError(err, path, filepath)
			// str = strings.Replace(str, "\n", "•", -1)
			// zlog.Info("Serve Manifest:", err, path, str)
			if peek != nil {
				peek(filepath, path, req)
			}
			http.ServeFile(w, req, filepath)
			return
		}
		zlog.Error(nil, "no correct dir for serving:", req.URL.Path, dir, pattern)
	}))
}

func AddHandler(router *mux.Router, pattern string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	pattern = AppURLPrefix + pattern
	// zlog.Info("zrest.AddHandler:", pattern)
	defer zlog.HandlePanic(false)
	if router == nil {
		http.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
			f(w, req)
		})
		return nil
	}
	return router.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		// zlog.Info("Handler:", pattern, req.URL)
		// timer := ztimer.StartIn(10, func() {
		// 	surl := req.URL.String()
		// 	zlog.Info("Request timed out after 10 seconds:", surl)
		// 	if surl == "/rpc" {
		// 		zlog.Info("RequestBody:", zhttp.GetCopyOfRequestBodyAsString(req))
		// 	}
		// 	ReturnError(w, req, "timeout out handling", http.StatusGatewayTimeout)
		// })
		f(w, req)
		// timer.Stop()
	})
}

func Handle(pattern string, handler http.Handler) {
	spath := path.Join(AppURLPrefix, pattern)
	spath += "/"
	// zlog.Info("zrest.Handle:", spath, pattern)
	http.Handle(spath, handler)
}
