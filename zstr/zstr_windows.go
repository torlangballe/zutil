package zstr

import (
	"runtime"
	"syscall"
)

func EnableTerminalColors(enable bool) error {
	// MIT License: https://github.com/konsorten/go-windows-terminal-sequences
	if runtime.GOOS != "windows" {
		return nil
	}
	const ENABLE_VIRTUAL_TERMINAL_PROCESSING uint32 = 0x4
	var kernel32Dll *syscall.LazyDLL = syscall.NewLazyDLL("Kernel32.dll")
	var setConsoleMode *syscall.LazyProc = kernel32Dll.NewProc("SetConsoleMode")

	var mode uint32
	err := syscall.GetConsoleMode(syscall.Stdout, &mode)
	if err != nil {
		return err
	}

	if enable {
		mode |= ENABLE_VIRTUAL_TERMINAL_PROCESSING
	} else {
		mode &^= ENABLE_VIRTUAL_TERMINAL_PROCESSING
	}

	ret, _, err := setConsoleMode.Call(uintptr(syscall.Stdout), uintptr(mode))
	if ret == 0 {
		return err
	}

	return nil
}
