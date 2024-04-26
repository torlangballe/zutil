package zwebsocket

import (
	"syscall/js"

	"github.com/torlangballe/zui/zdom"
	"github.com/torlangballe/zutil/zlog"
)

type WebSocket struct {
	socketJS js.Value
}

func New(address string, got func(data []byte)) *WebSocket {
	w := &WebSocket{}

	w.socketJS = zdom.New("WebSocket", address)
	w.socketJS.Set("binaryType", "arraybuffer")

	w.socketJS.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		zlog.Info("ws close")
		return nil
	}))
	w.socketJS.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		zlog.Info("ws err", args[0].String())
		return nil
	}))
	w.socketJS.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		e := args[0]
		data := e.Get("data")
		view := zdom.New("Uint8Array", data)
		length := data.Get("byteLength").Int()
		godata := make([]byte, length)
		js.CopyBytesToGo(godata, view)
		got(godata)
		// zlog.Info("ws message:", len(out), out)
		return nil
	}))
	return w
}

// let socket = new WebSocket("ws://192.168.1.127:81/")

// var view = new Uint8Array(e.data);
