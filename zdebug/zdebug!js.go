//go:build !js

package zdebug

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
	"os"
	"os/signal"
	rpprof "runtime/pprof"
	"strconv"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zerrors"
)

var AppURLPrefix func() string

func SetupSignalHandler() {
	c := make(chan os.Signal, 10)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP, syscall.SIGSYS)
	go func() {
		sig := <-c
		if sig == syscall.SIGTRAP {
			fmt.Println("************* SIGTRAP received, continuing *************")
			return
		}
		dict := zdict.Dict{"Signal": fmt.Sprint(sig)}
		err := zerrors.MakeContextError(dict, "Restart Signal")
		// fmt.Println("SIGNAL:", sig, fmt.Sprint(sig), sig.String(), dict)
		StoreAndExitError(err, true)
	}()
}

func handleCPUProfileDownload(w http.ResponseWriter, req *http.Request) {
	var data []byte

	secs, _ := strconv.Atoi(req.URL.Query().Get("secs"))
	if secs == 0 {
		secs = 10
	}
	buf := bytes.NewBuffer(data)
	err := rpprof.StartCPUProfile(buf)
	if err != nil {
		fmt.Println("handleCPUProfileDownload start cpu profile:", err)
		return
	}
	time.Sleep(time.Second * time.Duration(secs))
	rpprof.StopCPUProfile()

	n, err := io.Copy(w, buf)
	if err != nil {
		fmt.Println("handleCPUProfileDownload copy:", n, err)
		return
	}
	// fmt.Println("handleProfileDownload", secs, n)
}

func SetProfilingHandler(router *mux.Router) {
	for _, name := range AllProfileTypes {
		path := AppURLPrefix() + ProfilingURLPrefix + name
		if name == "cpu" {
			router.HandleFunc(path, handleCPUProfileDownload)
			continue
		}
		route := router.PathPrefix(path)
		// zlog.Info("zrest.AddSubHandler:", pattern)
		route.Handler(pprof.Handler(name))
	}
}
