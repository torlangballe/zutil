package urest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

var RunningOnServer bool

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
	regex := regexp.MustCompile("^[a-zA-Z]{1,35}$")
	if regex.MatchString(callback) {
		return fmt.Sprintf("%s(%s);", callback, json)
	}
	return json
}

// Adds CORS headers to response if appropriate.
func AddCORSHeaders(w http.ResponseWriter, req *http.Request) {
	origs := map[string]bool{
		"http://127.0.0.1:8090":      true,
		"http://127.0.0.1:5000":      true,
		"http://127.0.0.1:4000":      true,
		"https://51.15.138.187:8090": true,
	}
	o := req.Header.Get("Origin")
	if origs[o] {
		// fmt.Println("AddCorsHeaders:", o)
		w.Header().Set("Access-Control-Allow-Origin", o)
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Access-Token")
	}
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

	//	fmt.Println("ReturnResultWithHeaders:", string(jsonRep))
	for hName, hValue := range headers {
		w.Header().Set(hName, hValue)
	}
	// w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// w.Header().Set("Date", time.Now().In(time.UTC).Format(time.RFC3339))
	// AddCORSHeaders(w, req)

	fmt.Fprint(w, jsonRep)
}

// Returns HTTP error code and error messages in JSON representation, with string made of args and, printed
func ReturnAndPrintError(w http.ResponseWriter, req *http.Request, errorCode int, logger *log.Logger, a ...interface{}) {
	str := fmt.Sprintln(a)
	fmt.Print(str)
	if logger != nil {
		logger.Print("%s", str)
	}
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
	//	fmt.Println("ReturnError:", errorCode, messages)
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
			fmt.Println("urest.GetTimeZoneFromRequest bad offset:", soff, err)
			return nil
		}
		name := fmt.Sprintf("UTC%+d", offset)
		loc := time.FixedZone(name, int(offset)*3600)
		return loc
	}
	return nil
}

func GetIntVal(vals url.Values, name string, def int) int {
	return int(GetInt64Val(vals, name, int64(def)))
}

func GetInt64Val(vals url.Values, name string, def int64) int64 {
	s := vals.Get(name)
	if s == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return def
	}
	return n
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

func AddHandle(r *mux.Router, pattern string, f func(http.ResponseWriter, *http.Request)) *mux.Route {
	return r.HandleFunc(pattern, func(w http.ResponseWriter,
		req *http.Request) {
		f(w, req)
	})
}
