package zhttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"path"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zbytes"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zerrors"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type ErrJSON struct {
	Messages []string `json:"messages"`
}

type HTTPError struct {
	Err        error
	StatusCode int
}

type Parameters struct {
	Headers               map[string]string
	SkipVerifyCertificate bool
	PrintBody             bool
	ShowCURLCall          bool
	Args                  map[string]string
	TimeoutSecs           float64
	Method                string
	ContentType           string
	Body                  []byte
	Reader                io.Reader
	GetErrorFromBody      bool
	NoClientCache         bool // this creates a new client in MakeRequest(). You might need to call client.CloseIdleConnections() to avoid Keep-Alive requests qccumulating
	Context               context.Context
}

const DefaultTimeoutSeconds = 15

var (
	SetTelemetryForRedirectFunc func(surl string, secs float64)
)

func MakeParameters() Parameters {
	return Parameters{
		Headers:     map[string]string{},
		TimeoutSecs: DefaultTimeoutSeconds,
	}
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
		message = zstr.Concat(" ", message, http.StatusText(code))
	}
	e := &HTTPError{}
	e.Err = errors.New(message)
	e.StatusCode = code
	return e
}

// Post calls SendBody with method == Post
func Post(surl string, params Parameters, send, receive any) (resp *http.Response, err error) {
	params.Method = http.MethodPost
	return SendBody(surl, params, send, receive)
}

// Put calls SendBody with method == Put
func Put(surl string, params Parameters, send, receive any) (resp *http.Response, err error) {
	params.Method = http.MethodPut
	return SendBody(surl, params, send, receive)
}

// SendBody uses send as []byte, map[string]string (to url parameters, or unmarshals to use as body)
// receive can be []byte, string or a struct to unmarashal to
func SendBody(surl string, params Parameters, send, receive any) (*http.Response, error) {
	// start := time.Now()
	var err error
	if send != nil {
		zlog.Assert(params.Body == nil && params.Reader == nil)
	}
	zlog.Assert(!(send != nil && params.Body != nil || params.Reader != nil))
	if params.Body != nil {
		params.ContentType = "raw"
	} else {
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
				reader, got := send.(io.Reader)
				if got {
					params.Reader = reader
				} else {
					bout, err = json.Marshal(send)
					if err != nil {
						return nil, zlog.Error("marshal", err)
					}
					// zlog.Info("SendBody:", surl, string(bout))
				}
				if params.ContentType == "" {
					params.ContentType = "application/json"
				}
			}
		}
		params.Body = bout
		if params.ShowCURLCall {
			str := "curl -v -X " + string(params.Method) + " "
			if params.Body != nil {
				str += "-d " + zprocess.QuoteCommandLineArgument(string(params.Body)) + " "
			}
			for n, v := range params.Headers {
				str += "--header " + zprocess.QuoteCommandLineArgument(n+": "+v) + " "
			}
			str += zprocess.QuoteCommandLineArgument(surl)
			fmt.Println(zstr.EscCyan+str, zstr.EscNoColor)
		}
	}
	resp, _, err := SendBytesOrFromReader(surl, params)
	// if err != nil {
	// 	zlog.Warn("SendBytesOrFromReader:", err)
	// }
	err = processResponse(surl, resp, params, receive, err)
	if err != nil {
		err = makeContextErrorFromError(err, "Send Body", surl, params.Method)
		// zlog.Warn("SendErr2:", err, surl)
	}
	return resp, err
}

// var normalClient = &http.Client{
// 	Timeout: 15 * time.Second,
// 	// Transport: &http.Transport{
// 	// 	MaxIdleConnsPerHost: 100,
// 	// },
// }

var NoVerifyClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		//		MaxIdleConnsPerHost: 100,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	},
}

func makeClient(params Parameters) *http.Client {
	if params.TimeoutSecs == -1 {
		params.TimeoutSecs = 15
	}
	c := &http.Client{
		Timeout: ztime.SecondsDur(params.TimeoutSecs),
		Transport: &http.Transport{
			MaxIdleConnsPerHost: 5,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: params.SkipVerifyCertificate,
			},
		},
	}
	if runtime.GOOS != "js" {
		c.Transport = &http.Transport{ //
			MaxIdleConnsPerHost: 20,
			MaxConnsPerHost:     20,
			MaxIdleConns:        20,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: params.SkipVerifyCertificate,
			},
		}
	}
	return c
}

type clientInfo struct {
	timeoutSecs    float64
	skipVerifyCert bool
}

var clientCache zmap.LockMap[clientInfo, *http.Client]

func MakeRequest(surl string, params Parameters) (request *http.Request, client *http.Client, err error) {
	// wasm fetch options: https://go.dev/src/net/http/roundtrip_js.go
	if params.Args != nil {
		surl, _ = MakeURLWithArgs(surl, params.Args)
	}
	if params.NoClientCache {
		client = makeClient(params)
	} else {
		ci := clientInfo{timeoutSecs: params.TimeoutSecs, skipVerifyCert: params.SkipVerifyCertificate}
		client, _ = clientCache.Get(ci)
		if client == nil {
			client = makeClient(params)
			clientCache.Set(ci, client)
		}
	}
	// zlog.Info("MakeRequest:", client.Timeout, surl)
	reader := params.Reader
	if params.Body != nil {
		reader = bytes.NewReader(params.Body)
	}
	if params.Context == nil {
		request, err = http.NewRequest(params.Method, surl, reader)
	} else {
		request, err = http.NewRequestWithContext(params.Context, params.Method, surl, reader)
	}
	if err != nil {
		err = zlog.Error("new request", params.Context != nil, err)
		return
	}
	if params.ContentType != "" {
		params.Headers["Content-Type"] = params.ContentType
	}
	if params.Headers != nil {
		for k, v := range params.Headers {
			request.Header.Set(k, v)
		}
	}
	// zlog.Info("ContentType:", params.ContentType, request.Header)
	return request, client, err
}

var sendCount int64

func GetResponseFromReqClient(params Parameters, request *http.Request, client *http.Client) (resp *http.Response, err error) {
	// atomic.AddInt64(&sendCount, 1)
	// defer func() {
	// 	atomic.AddInt64(&sendCount, -1)
	// 	var sc int64
	// 	atomic.StoreInt64(&sc, sendCount)
	// 	if sc > 100 {
	// 		zlog.Info("too many zhttp sends:", sendCount, request.URL)
	// 	}
	// }()
	request.Close = true
	p := zprocess.PushProcess(30, "GetResponseFromReqClient:"+request.URL.String())
	resp, err = client.Do(request)
	zprocess.PopProcess(p)
	if err == nil && resp == nil {
		return nil, errors.New("client.Do gave no response: " + request.URL.String())
	}
	if params.GetErrorFromBody && (resp != nil && (err != nil || resp.StatusCode != 200)) {
		err = CheckErrorFromBody(resp)
	}
	// zlog.Info("GetResponseFromReqClient:", err)
	if err != nil {
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		return
	}
	if err == nil && resp != nil && resp.StatusCode >= 300 {
		// zlog.Info("GetResponseFromReqClient make error:")
		err = MakeHTTPError(err, resp.StatusCode, "")
		return
	}
	return
}

func GetResponse(surl string, params Parameters) (resp *http.Response, err error) {
	zlog.Assert(params.Method != "", params.Method, surl)
	req, client, err := MakeRequest(surl, params)
	// zlog.Info("GetResponse:", err, req != nil, client != nil)
	if err != nil {
		return
	}
	return GetResponseFromReqClient(params, req, client)
}

func GetResponseAndIPAddress(surl string, params Parameters) (resp *http.Response, fromIp, toIP string, err error) {
	zlog.Assert(params.Method != "", params.Method, surl)
	req, client, err := MakeRequest(surl, params)
	fip := new(string)
	tip := new(string)
	trace := &httptrace.ClientTrace{
		GotConn: func(connInfo httptrace.GotConnInfo) {
			*tip = connInfo.Conn.RemoteAddr().String()
			*fip = connInfo.Conn.LocalAddr().String()
		},
	}

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	// zlog.Info("GetResponse:", err, req != nil, client != nil)
	if err != nil {
		return
	}
	resp, err = GetResponseFromReqClient(params, req, client)
	zlog.Warn("GetResponseAndIPAddress:", err, *fip, *tip)
	return resp, *fip, *tip, err
}

func Get(surl string, params Parameters, receive any) (resp *http.Response, err error) {
	params.Method = http.MethodGet
	resp, err = GetResponse(surl, params)
	err = processResponse(surl, resp, params, receive, err)
	if err != nil {
		err = makeContextErrorFromError(err, "Get", surl, "GET")
	}
	return resp, err
}

func processResponse(surl string, resp *http.Response, params Parameters, receive any, err error) error {
	if resp == nil {
		return err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	if params.PrintBody {
		fmt.Println("dump:", resp.StatusCode, surl, ":\n"+GetCopyOfResponseBodyAsString(resp)+"\n")
	}
	if receive != nil && reflect.ValueOf(receive).Kind() != reflect.Ptr {
		zlog.Fatal("not pointer", surl)
	}
	if err != nil {
		var m zdict.Dict
		if json.Unmarshal([]byte(GetCopyOfResponseBodyAsString(resp)), &m) == nil {
			str, _ := m["error"].(string)
			if str != "" {
				err = errors.New(str)
			}
		}
		return err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		err = zlog.Error("readall", err)
		return err
	}
	if receive != nil {
		if rbytes, got := receive.(*[]byte); got {
			*rbytes = body
			return nil
		}
		if rstring, got := receive.(*string); got {
			*rstring = string(body)
			return nil
		}
		err = json.Unmarshal(body, receive)
		if err != nil {
			err = zlog.Error("unmarshal", err)
			return err
		}
	}
	return nil
}

func SendBytesOrFromReader(surl string, params Parameters) (resp *http.Response, code int, err error) {
	// zlog.Assert(len(params.Body) != 0 || params.Reader != nil, surl)
	zlog.Assert(params.ContentType != "")
	zlog.Assert(params.Method != "")
	if params.Body != nil {
		params.Headers["Content-Length"] = strconv.Itoa(len(params.Body))
	}
	// zlog.Info("SendBytesOrFromReader:", params.Method, surl)
	if params.PrintBody {
		zlog.Info("dump output:", surl, "\n", string(params.Body))
		for h, s := range params.Headers {
			zlog.Info(h+":", s)
		}
	}
	req, client, err := MakeRequest(surl, params)
	if err != nil {
		return
	}
	resp, err = GetResponseFromReqClient(params, req, client)
	if err != nil {
		return
	}
	return resp, resp.StatusCode, nil
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

func TruncateLongParametersInURL(surl string, onlyIfTotalCharsMoreThan, maxCharsInParameter int) string {
	if len(surl) <= onlyIfTotalCharsMoreThan {
		return surl
	}
	surl = zstr.ReplaceAllCapturesFunc(zstr.InSingleSquigglyBracketsRegex, surl, 0, func(cap string, index int) string {
		return "…"
	})
	u, err := url.Parse(surl)
	if zlog.OnError(err, surl) {
		return surl
	}
	var query = url.Values{}
	for k, vs := range u.Query() {
		for _, p := range vs {
			if len(p) > maxCharsInParameter {
				p = "xxx"
			}
			query.Set(k, p)
		}
	}
	u.RawQuery = query.Encode()
	return u.String()
}

func makeContextErrorFromError(err error, title, surl, method string) zerrors.ContextError {
	m := zstr.FirstToTitleCase(strings.ToLower(method))
	dict := zdict.Dict{
		"URL":         surl,
		"HTTP Method": m,
	}
	str := err.Error()
	pre := m + ` "` + surl + `": `
	if zstr.HasPrefix(str, pre, &str) {
		err = errors.New(str)
	}
	return zerrors.MakeContextError(dict, title, err)
}

func GetRedirectedURL(surl string, skipVerifyCertificate bool) (string, error) {
	var log string
	start := time.Now()
	params := MakeParameters()
	params.TimeoutSecs = 5
	params.SkipVerifyCertificate = skipVerifyCertificate
	params.NoClientCache = true // this was maybe needed since multiuple same clients got messed up doing GetRedirectedURL cuoncurrenly? Note needed client.CloseIdleConnections below
	// params.Method = http.MethodHead
	req, client, err := MakeRequest(surl, params)
	if err != nil {
		ie := makeContextErrorFromError(err, "Get Redirected URL/Make Request", surl, "Head")
		return surl, ie
	}
	lastUrlQuery := surl
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req == nil {
			log += fmt.Sprint("GetRedirectedURL:  req==nil", time.Since(start))
		} else {
			log += fmt.Sprintf("%s %v\n", req.URL.String(), time.Since(start))
			lastUrlQuery = req.URL.String()
		}
		if len(via) > 10 {
			return zlog.Error("too many redirects")
		}
		return nil
	}
	resp, err := client.Do(req)
	if resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	client.CloseIdleConnections()
	if err != nil {
		text := err.Error()
		if zstr.HasPrefix(text, `Head "`+surl+`": `, &text) {
			err = errors.New(text)
		}
		ie := makeContextErrorFromError(err, "Get Redirected URL", surl, "Head")
		err = ie
		u, _ := url.Parse(surl)
		startDNS := time.Now()
		ip, lerr := znet.DNSLookupToIP4(u.Host)
		if lerr != nil {
			zlog.Error(surl, time.Since(startDNS), "log:", log, lerr)
			return "", err
		}
		zlog.Info("GetRedirectedURL fail, lookup ok:", u.Host, ip, time.Since(startDNS), "log:", log)
	}
	if err == nil && SetTelemetryForRedirectFunc != nil {
		SetTelemetryForRedirectFunc(surl, ztime.Since(start))
	}
	return lastUrlQuery, err
}

func ReplaceURLsInText(text string, f func(surl string) string) string {
	// move this outside
	regEx := regexp.MustCompile(`(http|ftp|https):\/\/([\w\-_]+(?:(?:\.[\w\-_]+)+))([\w\-\.,@?^=%&amp;:/~\+#]*[\w\-\@?^=%&amp;/~\+#])?`)
	out := regEx.ReplaceAllStringFunc(text, f)
	return out
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

type myReader struct {
	*bytes.Buffer
}

func (m myReader) Close() error {
	return nil
}

func GetCopyOfRequestBodyAsString(req *http.Request) string {
	buf, _ := io.ReadAll(req.Body)
	reader1 := myReader{bytes.NewBuffer(buf)}
	reader2 := myReader{bytes.NewBuffer(buf)}
	req.Body = reader2
	body, _ := io.ReadAll(reader1)

	return string(body)
}

func GetCopyOfResponseBodyAsString(resp *http.Response) string {
	buf, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	reader1 := myReader{bytes.NewBuffer(buf)}
	reader2 := myReader{bytes.NewBuffer(buf)}
	resp.Body = reader2
	body, _ := io.ReadAll(reader1) // could this be buf.Bytes()?

	return string(body)
}

func CheckErrorFromBody(resp *http.Response) (err error) {
	if resp.StatusCode < 400 {
		return
	}
	body, err := io.ReadAll(resp.Body)
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
			str := zstr.Concat(" ", e.Type, code, e.Message, e.ObjectType)
			*err = errors.New(str)
		}
		return true
	}
	return false
}

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
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()
	err = CheckErrorFromBody(resp)
	if err != nil {
		return
	}
	info.ContentLengthBytes, _ = GetContentLengthFromHeader(resp.Header)

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

// ArgsFromURL gets Query arguments from a url.
// It can only store k/v of first unqiue key
func ArgsFromURL(u *url.URL) map[string]string {
	m := map[string]string{}
	for k, vs := range u.Query() {
		m[k] = vs[0]
	}
	return m
}

func MakeDataURL(data []byte, mime string) string {
	if mime == "" {
		mime = "text/html"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
}

func StringStartsWithHTTPX(str string) bool {
	return strings.HasPrefix(str, "http:") || strings.HasPrefix(str, "https:")
}

func HostIsLocal(host string) bool {
	return host == "localhost" || host == "127.0.0.1"
}

func HasURLScheme(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}
	return (u.Scheme != "")
}

func IsRuneValidForHeaderKey(r rune) bool {
	return zstr.IsRuneASCIIAlphaNumeric(r) || r == '-'
}

func GetContentLengthFromHeader(header http.Header) (int64, error) {
	return strconv.ParseInt(header.Get("Content-Length"), 10, 64)
}

// SendForFile POSTs parameter.Body or parameter.Reader to surl, with crc and Content-Length headers.
// If crc or length are 0, they are calculated, reading params.Reader into params.Body if needed.
func SendForFile(surl string, crc, length int64, params Parameters) error {
	var err error
	if crc == 0 || length == 0 {
		if params.Body == nil {
			buf := bytes.NewBuffer([]byte{})
			if crc == 0 {
				crcWriter := crc32.New(crc32.MakeTable(crc32.Castagnoli))
				multi := io.MultiWriter(crcWriter, buf)
				_, err = io.Copy(multi, params.Reader)
				crc = int64(crcWriter.Sum32())
			} else {
				_, err = io.Copy(buf, params.Reader)
			}
			if err != nil {
				return err
			}
			params.Body = buf.Bytes()
			params.Reader = nil
		}
		if crc == 0 {
			crc = zbytes.CalculateCRC32(params.Body, crc32.Castagnoli)
		}
		length = int64(len(params.Body))
	}
	params.TimeoutSecs = 300
	params.Headers["X-Zcrc"] = fmt.Sprint(crc)
	params.Headers["Content-Length"] = fmt.Sprint(length) // we set it here in case we still have a reader but know it
	_, err = Post(surl, params, nil, nil)
	return err
}

func HostFromURL(surl string) string {
	u, _ := url.Parse(surl)
	return u.Host
}
