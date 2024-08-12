//go:build !js

package zdebug

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func SetupSignalHandler() {
	c := make(chan os.Signal, 10)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGQUIT, syscall.SIGILL, syscall.SIGTRAP, syscall.SIGSYS)
	go func() {
		sig := <-c
		// if sig != syscall.SIGABRT && sig != syscall.SIGTERM {
		err := fmt.Errorf("%v", sig)
		StoreAndExitError(err, true)
		// }
	}()
}
