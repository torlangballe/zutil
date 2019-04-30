package uhttp

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
	"zutil/ustr"
	"zutil/ztime"
)

type ErrJSON struct {
	Messages []string `json:"messages"`
}

// Marshals send to json bytes unless it IS []byte and sends to url, unmarshalling result
func PostAsJsonGetJSON(surl string, otherHeaders map[string]string, bodyDump *string, send interface{}, receive interface{}) (code int, err error) {
	bout, got := send.([]byte)
	if !got {
		bout, err = json.Marshal(send)
		if err != nil {
			return
		}
	}

	response, code, err := PostBytesSetContentLength(surl, "application/json", bout, otherHeaders)
	if err != nil {
		return
	}
	if bodyDump != nil {
		*bodyDump = GetCopyOfResponseBodyAsString(response)
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	err = json.Unmarshal(body, receive)
	if err != nil {
		return
	}
	return
}

func UnmarshalFromJSONFromURL(surl string, v interface{}, print bool, authorization, authKey string) (ecode int, response *http.Response, err error) {
	client := http.DefaultClient
	req, err := http.NewRequest("GET", surl, nil)
	if err != nil {
		return
	}
	if authKey == "" {
		authKey = "Authorization"
	}
	if authorization != "" {
		//		fmt.Println("UnmarshalFromJSONFromUrlWithBody: auth:", authKey, authorization)
		req.Header.Set(authKey, authorization)
	}
	response, err = client.Do(req)
	if err != nil {
		return
	}
	ecode = response.StatusCode
	if print {
		sbody := GetCopyOfResponseBodyAsString(response)
		fmt.Println("UnmarshalFromJSONFromUrlWithBody:\n", sbody)
	}
	defer response.Body.Close()
	defer io.Copy(ioutil.Discard, response.Body)
	if response.StatusCode > 299 {
		err = fmt.Errorf("%s %d", response.Status, response.StatusCode)
		return
	}
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}
	if print {
		fmt.Println("UnmarshalFromJSONFromURL:", surl, ":\n", string(body))
	}
	err = json.Unmarshal(body, v)

	if err != nil {
		fmt.Println("UnmarshalFromJSONFromURL unmarshal Error:\n", err, surl, ustr.Head(string(body), 2000))
		return
	}
	return
}

func GetResponseAsStringFromUrl(surl string) (str string, err error) {
	resp, err := http.Get(surl)
	if err != nil {
		fmt.Println("Error getting url:", err, surl)
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
		fmt.Println("Error reading body:", err, surl)
		return
	}
	str = strings.Trim(string(body), `"`)
	fmt.Println("GetResponseAsStringFromUrl:", str, surl)
	return
}

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
		fmt.Println(string(body))
	}

	err = json.Unmarshal(body, v)
	return
}

func PostBytesSetContentLength(surl, ctype string, body []byte, otherHeaders map[string]string) (response *http.Response, code int, err error) {
	client := http.DefaultClient
	req, err := http.NewRequest("POST", surl, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	req.Header.Set("Content-Type", ctype)
	if otherHeaders != nil {
		for k, v := range otherHeaders {
			req.Header.Set(k, v)
		}
	}
	response, err = client.Do(req)
	if err != nil {
		return
	}

	code = response.StatusCode
	if response.StatusCode != 200 {
		err = fmt.Errorf("PostBytesSetContentLength: %d %s\n%s", response.StatusCode, response.Status, surl)
		return
	}
	return
}

func PostValuesAsForm(surl string, values url.Values, otherHeaders map[string]string) (data *[]byte, reAuth bool, err error) {
	var response *http.Response
	response, _, err = PostBytesSetContentLength(surl, "application/x-www-form-urlencoded", []byte(values.Encode()), otherHeaders)
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

func GetCurrentLocalIPAddress() (address, ip4 string, err error) {
	name, err := os.Hostname()
	if err != nil {
		return
	}
	addrs, err := net.LookupHost(name)
	//	fmt.Println("CurrentLocalIP Stuff:", name, addrs, err)
	if err != nil {
		return
	}

	for _, a := range addrs {
		if strings.Contains(a, ":") {
			if address == "" {
				address = a
			}
		} else {
			if ip4 == "" {
				ip4 = a
			}

		}
	}
	return
}

func GetOutboundIP() (ip net.IP, err error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = localAddr.IP
	return
}

func GetCurrentIPAddress() (address string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.String(), nil
			case *net.IPAddr:
				return v.String(), nil
			}
			// process IP address
		}
	}
	return "", nil
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

type ErrorStruct struct {
	Type    string `json:"type"`
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e ErrorStruct) Check(err *error) bool {
	if e.Type != "" || e.Code >= 300 || e.Message != "" {
		if err != nil {
			*err = errors.New(fmt.Sprintf("%s [%d] %s", e.Type, e.Code, e.Message))
		}
		return true
	}
	return false
}

func ServeJSONP(sjson string, w http.ResponseWriter, req *http.Request) {
	callback := req.FormValue("callback")
	if callback != "" {
		fmt.Println("ServeJSONP:", fmt.Sprintf("%s(%s)", callback, sjson))
		fmt.Fprintf(w, "%s(%s)", callback, sjson)
	} else {
		fmt.Println("ServeJSONP direct")
		w.Write([]byte(sjson))
	}
}

func GetHeaders(surl string) (header http.Header, err error) {
	resp, err := http.Head(surl)
	if err != nil {
		return
	}
	header = resp.Header
	return
}

type GetInfo struct {
	ConnectSecs      time.Duration
	DnsSecs          time.Duration
	TlsHandshakeSecs time.Duration
	FirstByteSecs    time.Duration
	DoneSecs         time.Duration
	ByteCount        int64
	RemoteAddress    string
	RemotePort       int
	RemoteDomain     string
}

func TimedGet(surl string, download bool) (info GetInfo, err error) {
	s := ztime.Second(-1)
	info.ConnectSecs = s
	info.TlsHandshakeSecs = s
	info.FirstByteSecs = s
	info.DnsSecs = s
	info.DoneSecs = s

	req, err := http.NewRequest("GET", surl, nil)
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

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	start = time.Now()

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return
	}

	if download {
		buf, _ := ioutil.ReadAll(resp.Body)
		info.ByteCount = int64(len(buf))
	}
	info.DoneSecs = time.Since(start)

	var port string
	ustr.SplitN(remoteAddress, ":", &info.RemoteAddress, &port)
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
