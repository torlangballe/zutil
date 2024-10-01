//go:build !js

package zdebug

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zerrors"
)

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
