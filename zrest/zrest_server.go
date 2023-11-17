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
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
)

var (
	RunningOnServer      bool
	LegalCORSOrigins     = map[string]bool{}
	CurrentInRequests    int
	ProfilingPort        int
	StaticFolderPathFunc = func(add string) string {
		return "www"
	}
)

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
	// zlog.Info("AddCorsHeaders:", o, find, req.URL.String(), "allowed:", LegalCORSOrigins, LegalCORSOrigins[find])
	if LegalCORSOrigins[find] {
		// zlog.Info("AddCorsHeaders2:", o, "allowed:", LegalCORSOrigins)
		w.Header().Set("Access-Control-Allow-Origin", o)
		// w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Access-Token")
		return
		// } else {
		// 	zlog.Info("AddCorsHeaders NOT allowed!:", o, find, LegalCORSOrigins)
	}
}

// Returns HTTP error code and error messages in JSON representation, with string made of args and, printed
func ReturnAndPrintError(w http.ResponseWriter, req *http.Request, errorCode int, a ...interface{}) error {
	str := fmt.Sprintln(a...)
	zlog.ErrorAtStack(nil, 5, a...)
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
			zlog.Error(err, s)
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
		http.Handle(pattern, h)
		return nil
	}
	route := router.PathPrefix(pattern)
	r := route.Handler(h)
	return r
}

// AddFileHandler adds a file serving handler, which removes the pattern path prefix before creating the filepath.
// It uses a FuncHandler function that is it's own http.Handler.
// It calls preprocess (if != nil) before serving the file, this can manipulate the corresponding filepath,
// or just be used for observing perposes.
func AddFileHandler(router *mux.Router, pattern, dir string, override func(w http.ResponseWriter, filepath *string, urlpath string, req *http.Request) bool) *mux.Route {
	return AddSubHandler(router, pattern, FuncHandler(func(w http.ResponseWriter, req *http.Request) {
		var path string
		if zstr.HasPrefix(req.URL.Path, AppURLPrefix+pattern, &path) {
			p := zprocess.PushProcess(30, "AddFileHandler:"+path)
			CurrentInRequests++
			filepath := filepath.Join(dir, path)
			// zlog.Info("AddFileHandler:", path, filepath)
			// str, err := zfile.ReadStringFromFile(filepath)
			// zlog.OnError(err, path, filepath)
			// str = strings.Replace(str, "\n", "•", -1)
			// zlog.Info("Serve Manifest:", err, path, str)
			if override != nil {
				if override(w, &filepath, path, req) {
					zprocess.PopProcess(p)
					CurrentInRequests--
					return
				}
			}
			http.ServeFile(w, req, filepath)
			zprocess.PopProcess(p)
			CurrentInRequests--
			return
		}
		zlog.Error(nil, "no correct dir for serving:", req.URL.Path, dir, pattern)
	}))
}

func AddHandler(router *mux.Router, pattern string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	pattern = AppURLPrefix + pattern
	defer zlog.HandlePanic(false)
	if router == nil {
		http.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
			p := zprocess.PushProcess(30, "AddHandler:"+req.URL.String())
			CurrentInRequests++
			start := time.Now()
			f(w, req)
			if zhttp.Logger != nil {
				zhttp.Logger.Add(req.URL.String(), toSecondary(req.RemoteAddr), req.Method, true, time.Since(start))
			}
			zprocess.PopProcess(p)
			CurrentInRequests--
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
		p := zprocess.PushProcess(30, "zrest.Handler:"+req.URL.String())
		CurrentInRequests++
		start := time.Now()
		f(w, req)
		if zhttp.Logger != nil {
			zhttp.Logger.Add(req.URL.String(), toSecondary(req.RemoteAddr), req.Method, true, time.Since(start))
		}
		CurrentInRequests--
		zprocess.PopProcess(p)
		// timer.Stop()
	})
}

func toSecondary(a string) string {
	var host string
	port := zstr.TailUntilWithRest(a, ":", &host)
	if port != "" {
		_, err := strconv.Atoi(port)
		if err == nil {
			return host
		}
	}
	return a
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

func SetProfilingPort(port int) {
	ProfilingPort = port
	zdebug.ProfilingPort = port
}

func SetProfilingHandler() {
	zlog.Assert(ProfilingPort != 0)
	router := mux.NewRouter()
	// http.Handle("/debug/pprof/", pprof.Index)
	spath := "/debug/pprof"
	router.PathPrefix(spath).Handler(http.DefaultServeMux)
	router.HandleFunc(spath+"/", pprof.Index)

	zlog.Info("SetProfilingHandle:", spath, ProfilingPort)
	//	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	//
	// r.HandleFunc("/debug/pprof/profile", pprof.Index)
	// router.HandleFunc("/qtt/debug/pprof/heap", pprof.Index)
	//
	//	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	//	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	go http.ListenAndServe(fmt.Sprint(":", ProfilingPort), router)
}
