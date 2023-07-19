// The zupload server component needs to be Init()'ed to handle posts from the gui's default handling.
// Call RegisterUploadHandler to register an upload handler. It handles a string id set in gui for an uploader.

//go:build server

package zupload

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zstr"
	"golang.org/x/crypto/ssh"
)

var (
	handlers      = map[string]func(name string, reader io.ReadCloser) (zdict.Dict, error){}
	authenticator zrpc.TokenAuthenticator
)

func Init(router *mux.Router, a zrpc.TokenAuthenticator) {
	zrest.AddHandler(router, "zupload", handleUpload).Methods("POST")
	authenticator = a
}

// RegisterUploadHandler registers a method to call if an upload with id is done by gui.
// It can return status in a dictionary for the gui.
func RegisterUploadHandler(id string, handler func(name string, reader io.ReadCloser) (zdict.Dict, error)) {
	handlers[id] = handler
}

func callHandler(up UploadPayload, reader io.ReadCloser) (zdict.Dict, error) {
	h := handlers[up.HandleID]
	if h == nil {
		return nil, zlog.NewError(nil, "no handle for upload with id", up.HandleID)
	}
	return h(up.Name, reader)
}

func CopySPC(url, password string, consume func(reader io.ReadCloser) error) error {
	user := zstr.HeadUntilLast(url, "@", &url)
	config, err := auth.PasswordKey(user, password, ssh.InsecureIgnoreHostKey())
	if err != nil {
		return err
	}
	address := zstr.HeadUntilLast(url, ":", &url)
	if !strings.Contains(address, ":") {
		address += ":22"
	}
	path := url
	client := scp.NewClientWithTimeout(address, &config, time.Minute*2)
	err = client.Connect()
	if err != nil {
		return zlog.Error(err, "connect", address, password)
	}
	reader, writer := io.Pipe()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		err = consume(reader) // this starts sucking from reader in goroutine, or copy below will hang
		wg.Done()
	}()
	copyErr := client.CopyFromRemotePassThru(context.Background(), writer, path, nil) // use another error as err is set in goroutine above
	writer.Close()
	wg.Wait()
	if copyErr != nil {
		return zlog.Error(copyErr, "copy", address, password)
	}
	return err
}

func copySPC(up UploadPayload) (result zdict.Dict, err error) {
	err = CopySPC(up.Text, up.Password, func(reader io.ReadCloser) error {
		var cerr error
		result, cerr = callHandler(up, reader)
		return cerr
	})
	return
}

func copyURL(up UploadPayload) (result zdict.Dict, err error) {
	params := zhttp.MakeParameters()
	params.Method = http.MethodGet
	resp, err := zhttp.GetResponse(up.Text, params)
	if err != nil {
		return nil, zlog.Error(err, "get-response")
	}
	return callHandler(up, resp.Body)
}

func handleUpload(w http.ResponseWriter, req *http.Request) {
	var up UploadPayload
	var result zdict.Dict
	var err error

	defer req.Body.Close()
	values := req.URL.Query()
	up.Name = values.Get("name")
	up.Type = values.Get("type")
	up.HandleID = values.Get("id")
	up.Text = values.Get("text")
	up.Password = req.Header.Get("X-Password")
	token := req.Header.Get("X-Token")
	if authenticator != nil && !authenticator.IsTokenValid(token) {
		zrest.ReturnAndPrintError(w, req, http.StatusUnauthorized)
		return
	}
	reader := req.Body

	switch up.Type {
	case SCP:
		result, err = copySPC(up)
		// zlog.Info("SPC done")
	case URL:
		result, err = copyURL(up)
	default:
		result, err = callHandler(up, reader)
	}
	if err != nil {
		result = zdict.Dict{"error": err.Error()}
	}
	zrest.ReturnDict(w, req, result)
}
