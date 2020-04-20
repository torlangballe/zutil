package zcommand

import (
	"fmt"
    "strings"
    "syscall"

    "github.com/AllenDang/w32"
    "github.com/hnakamur/w32syscall"
)

// https://superuser.com/questions/75614/take-a-screen-shot-from-command-line-in-windows

func CloseAppWindowWithTitle(app, title string, allExcept bool) error {
	titleName := "title"
	if app == GetAppNameOfBrowser(Safari, true) {
		titleName = "name"
	}
package main


func main() {
    err := w32syscall.EnumWindows(func(hwnd syscall.Handle, lparam uintptr) bool {
        h := w32.HWND(hwnd)
        text := w32.GetWindowText(h)
        if strings.Contains(text, "Calculator") {
            w32.MoveWindow(h, 0, 0, 200, 600, true)
        }
        return true
    }, 0)
    if err != nil {
        log.Fatalln(err)
    }
}}
