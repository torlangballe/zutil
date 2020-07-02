package uhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type BrowserType string

const (
	Safari  BrowserType = "safari"
	Chrome              = "chrome"
	Edge                = "edge"
	Default             = "default"
)

type ErrJSON struct {
	Messages []string `json:"messages"`
}

type HTTPError struct {
	Err        error
	StatusCode int
}

func (e *HTTPError) Error() string {
	return e.Err.Error()
}

func (e *HTTPError) Unwrap() error {
	return e.Err
}

func MakeHTTPError(err error, code int, message string) error {
	if message != "" {
		zstr.Replace(&message, "<p>", "\n")
		zstr.Replace(&message, "</p>", "\n")
		for zstr.Replace(&message, "\n\n", "\n") {
		}
		if err != nil {
			message += ": " + err.Error()
		}
	} else {
		if err != nil {
			message = err.Error()
		}
	}
	if err == nil && code > 0 {
		zstr.Concat(&message, " ", http.StatusText(code))
	}
	e := &HTTPError{}
	e.Err = errors.New(message)
	e.StatusCode = code
	return e
}

type Parameters struct {
	Headers               map[string]string
	SkipVerifyCertificate bool
	PrintBody             bool
	Args                  map[string]string
	TimeoutSecs           float64
	UseHTTPS              bool
	Method                string
	ContentType           string
	Body                  []byte
}

func MakeParameters() Parameters {
	return Parameters{
		Headers:     map[string]string{},
		TimeoutSecs: 15,
	}
}

// Post uses send as []byte, map[string]string (to url parameters, or unmarshals to use as body)
// receive can be []byte, string or a struct to unmarashal to
func Post(surl string, params Parameters, send, receive interface{}) (headers http.Header, err error) {
	params.Method = http.MethodPost
	bout, got := send.([]byte)
	if got {
		if params.ContentType == "" {
			params.ContentType = "raw"
		}
	} else {
		m, got := send.(map[string]string)
		if got {
			bout = []byte(zstr.GetArgsAsURLParameters(m))
			if params.ContentType == "" {
				params.ContentType = "application/x-www-form-urlencoded"
			}
		} else {
			bout, err = json.Marshal(send)
			// zlog.Info("POST:", err, string(bout))
			if err != nil {
				err = zlog.Error(err, "marshal")
				return
			}
			if params.ContentType == "" {
				params.ContentType = "application/json"
			}
		}
	}
	params.Body = bout
	response, code, err := PostBytesSetContentLength(surl, params)
	if err != nil || code >= 300 {
		if response != nil {
			response.Body.Close()
		}
		err = zlog.Error(err, "post bytes")
		err = MakeHTTPError(err, code, "post")
		return
	}
	return processResponse(surl, response, params.PrintBody, receive)
}

func MakeRequest(surl string, params Parameters) (request *http.Request, client *http.Client, err error) {
	if params.Args != nil {
		surl, _ = MakeURLWithArgs(surl, params.Args)
	}
	client = http.DefaultClient
	if params.SkipVerifyCertificate {
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
	}
	if params.TimeoutSecs != 0 {
		client.Timeout = time.Duration(float64(time.Second) * params.TimeoutSecs)
	}

	var reader io.Reader
	if params.Body != nil {
		reader = bytes.NewReader(params.Body)
	}
	req, err := http.NewRequest(params.Method, surl, reader)
	if err != nil {
		err = zlog.Error(err, "new request")
		return
	}
	if params.Headers != nil {
		for k, v := range params.Headers {
			req.Header.Set(k, v)
		}
	}

	return req, client, err
}

func GetResponseFromReqClient(request *http.Request, client *http.Client) (response *http.Response, err error) {
	request.Close = true
	response, err = client.Do(request)
	// zlog.Info("GetResponseFromReqClient:", err, response != nil, request.URL)
	if err == nil && response == nil {
		return nil, errors.New("client.Do gave no response: " + request.URL.String())
	}
	if err != nil && response != nil {
		response.Body.Close()
		return
	}
	if err == nil && response != nil && response.StatusCode >= 300 {
		// zlog.Info("GetResponseFromReqClient make error:")
		err = MakeHTTPError(err, response.StatusCode, "")
		return
	}
	return
}

func GetResponse(surl string, params Parameters) (response *http.Response, err error) {
	zlog.Assert(params.Method != "", params, surl)
	req, client, err := MakeRequest(surl, params)
	// zlog.Info("GetResponse:", err, req != nil, client != nil)
	if err != nil {
		return
	}
	return GetResponseFromReqClient(req, client)
}

func Get(surl string, params Parameters, receive interface{}) (headers http.Header, err error) {
	params.Method = http.MethodGet
	resp, err := GetResponse(surl, params)
	if err != nil || resp == nil {
		return
	}
	return processResponse(surl, resp, params.PrintBody, receive)
}

func processResponse(surl string, response *http.Response, printBody bool, receive interface{}) (headers http.Header, err error) {
	headers = response.Header
	if printBody {
		zlog.Info("dump:", response.StatusCode, surl, ":\n"+GetCopyOfResponseBodyAsString(response)+"\n")
	}
	if receive != nil && reflect.ValueOf(receive).Kind() != reflect.Ptr {
		zlog.Fatal(nil, "not pointer", surl)
	}
	if response.Body != nil {
		defer response.Body.Close()
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		err = zlog.Error(err, "readall")
		return
	}
	if receive != nil {
		if rbytes, got := receive.(*[]byte); got {
			*rbytes = body
			return
		}
		if rstring, got := receive.(*string); got {
			*rstring = string(body)
			return
		}
		err = json.Unmarshal(body, receive)
		if err != nil {
			err = zlog.Error(err, "unmarshal")
			return
		}
	}
	return
}

func UnmarshalFromJSONFromURL(surl string, v interface{}, print bool, authorization, authKey string) (response *http.Response, err error) {
	client := http.DefaultClient
	req, err := http.NewRequest("GET", surl, nil)
	if err != nil {
		return
	}
	if authKey == "" {
		authKey = "Authorization"
	}
	if authorization != "" {
		//		zlog.Info("UnmarshalFromJSONFromUrlWithBody: auth:", authKey, authorization)
		req.Header.Set(authKey, authorization)
	}
	response, err = client.Do(req)
	if err != nil {
		if response != nil {
			err = MakeHTTPError(err, response.StatusCode, "get")
		}
		return
	}
	if print {
		sbody := GetCopyOfResponseBodyAsString(response)
		zlog.Info("UnmarshalFromJSONFromUrlWithBody:\n", sbody)
	}
	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)
	ecode := response.StatusCode
	if ecode >= 300 {
		err = MakeHTTPError(nil, ecode, "get")
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	if print {
		zlog.Info("UnmarshalFromJSONFromURL:", surl, ":\n", string(body))
	}
	err = json.Unmarshal(body, v)

	if err != nil {
		zlog.Info("UnmarshalFromJSONFromURL unmarshal Error:\n", err, surl, zstr.Head(string(body), 2000))
		return
	}
	return
}

/*
func GetResponseAsStringFromUrl(surl string) (str string, err error) {
	resp, err := http.Get(surl)
	if err != nil {
		zlog.Info("Error getting url:", err, surl)
		return
	}
	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	if resp.StatusCode != 200 {
		err = fmt.Errorf("%s %d", resp.Status, resp.StatusCode)
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		zlog.Info("Error reading body:", err, surl)
		return
	}
	str = strings.Trim(string(body), `"`)
	zlog.Info("GetResponseAsStringFromUrl:", str, surl)
	return
}
*/

/*
func UnmarshalFromJSONFromPostForm(surl string, vals url.Values, v interface{}, print bool) (err error) {
	response, err := http.PostForm(surl, vals)
	if err != nil {
		return
	}
	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	if response.StatusCode >= 300 {
		var ej ErrJSON
		jerr := json.Unmarshal(body, &ej)
		if jerr == nil && len(ej.Messages) > 0 {
			err = errors.New(ej.Messages[0])
			return
		}
	}
	if print {
		zlog.Info(string(body))
	}

	err = json.Unmarshal(body, v)
	return
}
*/

func PostBytesSetContentLength(surl string, params Parameters) (response *http.Response, code int, err error) {
	zlog.Assert(len(params.Body) != 0, surl)
	zlog.Assert(params.ContentType != "")
	params.Headers["Content-Length"] = strconv.Itoa(len(params.Body))
	params.Headers["Content-Type"] = params.ContentType
	params.Method = http.MethodPost
	req, client, err := MakeRequest(surl, params)
	if err != nil {
		return
	}
	resp, err := GetResponseFromReqClient(req, client)
	if err != nil {
		return
	}
	return resp, resp.StatusCode, nil
}

func PostValuesAsForm(surl string, params Parameters, values url.Values) (data *[]byte, reAuth bool, err error) {
	var response *http.Response
	params.Body = []byte(values.Encode())
	params.ContentType = "application/x-www-form-urlencoded"
	response, _, err = PostBytesSetContentLength(surl, params)
	if err != nil {
		return
	}
	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	data = &body
	return
}

func MakeURLWithArgs(surl string, args map[string]string) (string, error) {
	u, err := url.Parse(surl)
	if err != nil {
		return "", err
	}
	q := u.Query()
	for k, v := range args {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func GetRedirectedURL(surl string) (string, error) {
	resp, err := http.Get(surl)
	if err != nil {
		return surl, errors.New(fmt.Sprint("getRedirectedURL: ", surl, " ", err))
	}
	defer resp.Body.Close()
	defer io.Copy(ioutil.Discard, resp.Body)
	return resp.Request.URL.String(), nil
}

func ReplaceUrlsInText(text string, f func(surl string) string) string {
	regEx := regexp.MustCompile(`(http|ftp|https):\/\/([\w\-_]+(?:(?:\.[\w\-_]+)+))([\w\-\.,@?^=%&amp;:/~\+#]*[\w\-\@?^=%&amp;/~\+#])?`)
	out := regEx.ReplaceAllStringFunc(text, f)
	return out
}

func GetDomainFromUrl(surl string) string {
	u, err := url.Parse(surl)
	if err != nil {
		return surl
	}
	return u.Host
}

func GetIPAddressAndPortFromRequest(req *http.Request) (ip, port string, err error) {
	sip, sport, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return
	}

	userIP := net.ParseIP(sip)
	if userIP == nil {
		return
	}
	ip = sip
	port = sport
	return
}

func GetAndReturnUrlEncodedBodyValues(surl string) (values url.Values, err error) {
	response, err := http.Get(surl)
	if err != nil {
		return
	}
	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	return url.ParseQuery(string(body))
}

type myReader struct {
	*bytes.Buffer
}

func (m myReader) Close() error {
	return nil
}

func GetCopyOfRequestBodyAsString(req *http.Request) string {
	buf, _ := ioutil.ReadAll(req.Body)
	reader1 := myReader{bytes.NewBuffer(buf)}
	reader2 := myReader{bytes.NewBuffer(buf)}
	req.Body = reader2
	body, _ := ioutil.ReadAll(reader1)

	return string(body)
}

func GetCopyOfResponseBodyAsString(resp *http.Response) string {
	buf, _ := ioutil.ReadAll(resp.Body)
	reader1 := myReader{bytes.NewBuffer(buf)}
	reader2 := myReader{bytes.NewBuffer(buf)}
	resp.Body = reader2
	body, _ := ioutil.ReadAll(reader1)

	return string(body)
}

func CheckErrorFromBody(resp *http.Response) (err error) {
	if resp.StatusCode < 400 {
		return
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}
	var e ErrorStruct
	jerr := json.Unmarshal(body, &e)
	if jerr != nil {
		if e.Check(&err) {
			return
		}
	}
	var e2 struct {
		Result ErrorStruct `json:"result"`
	}
	jerr = json.Unmarshal(body, &e2)
	if e2.Result.Check(&err) {
		return
	}
	var e3 struct {
		Error ErrorStruct `json:"error"`
	}
	jerr = json.Unmarshal(body, &e3)
	if e3.Error.Check(&err) {
		return
	}
	var e4 struct {
		Result struct {
			Error ErrorStruct `json:"error"`
		} `json:"result"`
	}
	jerr = json.Unmarshal(body, &e4)
	if e4.Result.Error.Check(&err) {
		return
	}

	err = errors.New(fmt.Sprintf("Code: %d", resp.StatusCode))
	return
}

type ErrorStruct struct {
	Type       string          `json:"type"`
	ObjectType string          `json:"objectType"`
	Code       json.RawMessage `json:"code"`
	Message    string          `json:"message"`
}

func (e ErrorStruct) Check(err *error) bool {
	if e.Type != "" || e.Message != "" {
		if err != nil {
			code := strings.Trim(string(e.Code), `"`)
			str := zstr.ConcatenateNonEmpty(" ", e.Type, code, e.Message, e.ObjectType)
			*err = errors.New(str)
		}
		return true
	}
	return false
}

/*
func ServeJSONP(sjson string, w http.ResponseWriter, req *http.Request) {
	callback := req.FormValue("callback")
	if callback != "" {
		zlog.Info("ServeJSONP:", fmt.Sprintf("%s(%s)", callback, sjson))
		fmt.Fprintf(w, "%s(%s)", callback, sjson)
	} else {
		zlog.Info("ServeJSONP direct")
		w.Write([]byte(sjson))
	}
}
*/

func GetHeaders(surl string) (header http.Header, err error) {
	resp, err := http.Head(surl)
	if err != nil {
		return
	}
	if resp.Body != nil {
		resp.Body.Close()
	}
	header = resp.Header
	return
}

type GetInfo struct {
	ConnectSecs        time.Duration
	DnsSecs            time.Duration
	TlsHandshakeSecs   time.Duration
	FirstByteSecs      time.Duration
	DoneSecs           time.Duration
	ByteCount          int64
	RemoteAddress      string
	RemotePort         int
	RemoteDomain       string
	ContentLengthBytes int64
}

func TimedGet(surl string, downloadBytes int64) (info GetInfo, err error) {
	s := ztime.SecondsDur(-1)
	info.ConnectSecs = s
	info.TlsHandshakeSecs = s
	info.FirstByteSecs = s
	info.DnsSecs = s
	info.DoneSecs = s

	req, err := http.NewRequest("GET", surl, nil)
	if err != nil {
		return
	}
	var start, connect, dns, tlsHandshake time.Time
	var remoteAddress string

	trace := &httptrace.ClientTrace{
		DNSStart: func(dsi httptrace.DNSStartInfo) { dns = time.Now() },
		DNSDone: func(ddi httptrace.DNSDoneInfo) {
			info.DnsSecs = time.Since(dns)
		},

		TLSHandshakeStart: func() { tlsHandshake = time.Now() },
		TLSHandshakeDone: func(cs tls.ConnectionState, err error) {
			info.TlsHandshakeSecs = time.Since(tlsHandshake)
		},

		ConnectStart: func(network, addr string) { connect = time.Now() },
		ConnectDone: func(network, addr string, err error) {
			info.ConnectSecs = time.Since(connect)
		},

		GotFirstResponseByte: func() {
			info.FirstByteSecs = time.Since(start)
		},
		GotConn: func(connInfo httptrace.GotConnInfo) {
			remoteAddress = connInfo.Conn.RemoteAddr().String()
		},
	}

	ctx, cancel := context.WithTimeout(req.Context(), 30*time.Second) // context.Background()
	defer cancel()

	req = req.WithContext(httptrace.WithClientTrace(ctx, trace)) // req.Context()

	start = time.Now()

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	err = CheckErrorFromBody(resp)
	if err != nil {
		return
	}
	info.ContentLengthBytes, _ = strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)

	if downloadBytes != 0 {
		bytes := make([]byte, 1024)
		for {
			n, rerr := resp.Body.Read(bytes)
			info.ByteCount += int64(n)
			if rerr != nil && rerr != io.EOF {
				err = rerr
				return
			}
			if rerr == io.EOF || info.ByteCount >= downloadBytes {
				break
			}
		}
	}
	info.DoneSecs = time.Since(start)

	var port string
	zstr.SplitN(remoteAddress, ":", &info.RemoteAddress, &port)
	names, _ := net.LookupAddr(info.RemoteAddress)
	if len(names) > 0 {
		info.RemoteDomain = names[0]
	} else {
		info.RemoteDomain = info.RemoteAddress
	}
	p, _ := strconv.ParseInt(port, 10, 32)
	info.RemotePort = int(p)
	return
}

func AddPathToURL(surl, add string) string {
	u, err := url.Parse(surl)
	if err != nil {
		return path.Join(surl, add)
	}
	u.Path = path.Join(u.Path, add)
	return u.String()
}

func ValsFromURL(surl string) url.Values {
	u, err := url.Parse(surl)
	if err == nil {
		return u.Query()
	}
	return url.Values{}
}

func MakeDataURL(data []byte, mime string) string {
	if mime == "" {
		mime = "text/plain;charset=utf-8"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}
