package zrest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"time"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zstr"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
)

var RunningOnServer bool
var AppURLPrefix = "/"

// Formats a JSON string (linebreaks + identation).
func formatJSON(input []byte) (out string, err error) {
	buf := bytes.NewBuffer([]byte{})
	err = json.Indent(buf, input, "", "\t")
	if err != nil {
		return
	}
	out = buf.String()
	return
}

// Adds JSONP call back to JSON string if callback is valid. Callback
// can only consist of :alpha: from len() = [1,35]

func addJSONPCallback(json string, callback string) string {
	regex := regexp.MustCompile("^[a-zA-Z]{1,35}$") // move this outside as "global"
	if regex.MatchString(callback) {
		zlog.Fatal(nil, "What is this?")
		return fmt.Sprintf("%s(%s);", callback, json)
	}
	return json
}

var LegalCORSOrigins = map[string]bool{}

// Adds CORS headers to response if appropriate.
func AddCORSHeaders(w http.ResponseWriter, req *http.Request) {
	o := req.Header.Get("Origin")
	obase := zstr.HeadUntilLast(o, ":")
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
	// if o != "" {
	// 	var sport string
	// 	u, err := url.Parse(o)
	// 	if err == nil && port != 0 {
	// 		port, _ := strconv.Atoi(sport)
	// 		if o != "" && sport != "" && (port < 5000) || port > 60000 {
	// 			zlog.Info("🟥AddCorsHeaders Fail2:", sport, port, req.RemoteAddr, req.URL, o, ":", LegalCORSOrigins) // req.Header,
	// 		}
	// 	} else {
	// 		zlog.Info("🟥AddCorsHeaders Fail: bad origin:", o)
	// 	}
	// }
}

// Returns JSON representation of given object. JSONP if callback parameter is specified.
func ReturnResult(w http.ResponseWriter, req *http.Request, obj interface{}) {
	ReturnResultWithHeaders(w, req, nil, obj)
}

// Returns JSON representation of given object. JSONP if callback parameter is specified.
// Adds given headers to the response.
func ReturnResultWithHeaders(w http.ResponseWriter, req *http.Request, headers map[string]string, obj interface{}) {
	tmp, _ := json.Marshal(obj)

	jsonRep, _ := formatJSON(tmp)
	jsonRep = addJSONPCallback(jsonRep, req.FormValue("callback"))

	//	zlog.Info("ReturnResultWithHeaders:", string(jsonRep))
	for hName, hValue := range headers {
		w.Header().Set(hName, hValue)
	}
	// w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// w.Header().Set("Date", time.Now().In(time.UTC).Format(time.RFC3339))
	// AddCORSHeaders(w, req)

	fmt.Fprint(w, jsonRep)
}

// Returns HTTP error code and error messages in JSON representation, with string made of args and, printed
func ReturnAndPrintError(w http.ResponseWriter, req *http.Request, errorCode int, a ...interface{}) {
	str := fmt.Sprintln(a...)
	zlog.ErrorAtStack(nil, 5, a...)
	ReturnError(w, req, str, errorCode)
}

// Returns HTTP error code and error messages in JSON representation.
func ReturnError(w http.ResponseWriter, req *http.Request, message string, errorCode int) {
	resMap := make(map[string][]string)
	resMap["messages"] = []string{message}
	tmp, _ := json.Marshal(resMap)
	jsonRep, _ := formatJSON(tmp)

	jsonRep = addJSONPCallback(jsonRep, req.FormValue("callback"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Date", time.Now().In(time.UTC).Format(time.RFC3339))

	AddCORSHeaders(w, req)
	//	zlog.Info("ReturnError:", errorCode, messages)
	if errorCode == 0 {
		errorCode = http.StatusInternalServerError
	}
	w.WriteHeader(errorCode)
	fmt.Fprint(w, jsonRep)
}

// Returns {"somekey":<some interface{}>}.
func ReturnSingle(w http.ResponseWriter, req *http.Request, key string, val interface{}) {
	resp := map[string]interface{}{key: val}
	tmp, _ := json.Marshal(resp)
	jsonRep := string(tmp)
	jsonRep = addJSONPCallback(jsonRep, req.FormValue("callback"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Date", time.Now().In(time.UTC).Format(time.RFC3339))
	AddCORSHeaders(w, req)
	fmt.Fprint(w, jsonRep)
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

func AddHandler(router *mux.Router, pattern string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	pattern = AppURLPrefix + pattern
	// zlog.Info("zrest.AddHandler:", pattern)
	defer zlog.HandlePanic(false)
	http.Handle(pattern, router) // do we need this????
	return router.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
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
