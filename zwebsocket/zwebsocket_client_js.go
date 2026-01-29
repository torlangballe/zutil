package zwebsocket

import (
	"math/rand"
	"syscall/js"
	"time"

	"github.com/torlangballe/zui/zdom"
	"github.com/torlangballe/zutil/zbytes"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
)

type Client struct {
	id           string
	url          string
	socketJS     js.Value
	isOpen       bool
	receiveChans zmap.LockMap[int64, chan message]
	handlerFunc  func(data []byte, err error) []byte
	Timeout      time.Duration
}

type message struct {
	msg []byte
	err error
	num int64
}

func NewClient(id, url string, handler func(data []byte, err error) []byte) (*Client, error) {
	c := &Client{}
	c.ID = id
	c.url = url
	c.handlerFunc = handler
	c.Timeout = time.Second * 10
	err := c.open()
	return c, err
}

func (c *Client) open() error {
	c.socketJS = zdom.New("WebSocket", c.url)
	c.socketJS.Set("binaryType", "arraybuffer")
	c.socketJS.Set("onopen", js.FuncOf(func(this js.Value, args []js.Value) any {
		c.isOpen = true
		return nil
	}))
	c.socketJS.Set("onclose", js.FuncOf(func(this js.Value, args []js.Value) any {
		c.isOpen = false
		c.socketJS = js.Undefined()
		return nil
	}))
	c.socketJS.Set("onerror", js.FuncOf(func(this js.Value, args []js.Value) any {
		zlog.Info("ws error")
		if !c.socketJS.IsUndefined() {
			c.socketJS.Call("close")
		}
		c.socketJS = js.Undefined()
		c.isOpen = false
		return nil
	}))
	c.socketJS.Set("onmessage", js.FuncOf(func(this js.Value, args []js.Value) any {
		e := args[0]
		data := e.Get("data")
		view := zdom.New("Uint8Array", data)
		length := data.Get("byteLength").Int()
		godata := make([]byte, length)
		js.CopyBytesToGo(godata, view)
		c.handleNewMessage(godata)
		return nil
	}))
	return nil
}

func (c *Client) Close() error {
	c.isOpen = false
	if !c.socketJS.IsUndefined() {
		c.socketJS.Call("close")
	}
	c.socketJS = js.Undefined()
	return nil
}

func (c *Client) send(msg []byte) error {
	for count := 0; !c.isOpen; count++ {
		time.Sleep(time.Millisecond * 50)
		if count > 10 {
			return zlog.Error("ws not open for sending")
		}
	}
	if c.socketJS.IsUndefined() {
		return zlog.Error("no ws connection")
	}
	uint8Array := zdom.New("Uint8Array", len(msg))
	js.CopyBytesToJS(uint8Array, msg)
	c.socketJS.Call("send", uint8Array)
	return nil
}

func (c *Client) handleNewMessage(msg []byte) {
	num := int64(zbytes.BytesToUInt64(msg[0:8]))
	n := zint.Abs64(num)
	ch, ok := c.receiveChans.Get(n)
	// zlog.Warn("receivedMessage:", zlog.Pointer(c), string(msg[8:]), "num:", num, ok)
	if ok { // it was a response, and we have a response channel waiting
		m := message{
			num: n,
			msg: msg[8:],
		}
		ch <- m // send reply which Exchange() is waiting for
		return
	}
	if num < 0 {
		zlog.Error("Received message with return but no channel waiting:", num)
		return
	}
	result := c.handlerFunc(msg[8:], nil)
	pre := zbytes.UInt64ToBytes(uint64(-n))
	err := c.send(append(pre, result...))
	zlog.OnError(err, "sending reply")
	if err != nil {
		c.handlerFunc(nil, err)
		c.Close()
		return
	}
}

func (c *Client) Exchange(msg []byte) ([]byte, error) {
	var outErr error
	num := rand.Int63()
	pre := zbytes.UInt64ToBytes(uint64(num))
	send := append(pre, msg...)
	ch := make(chan message, 1)
	c.receiveChans.Set(num, ch)
	defer c.receiveChans.Remove(num)
	err := c.send(send)
	if err != nil {
		outErr = err
		return nil, outErr
	}
	for {
		select {
		case r := <-ch:
			return r.msg, r.err
		case <-time.After(c.Timeout):
			err = zlog.Error("WebSocket client exchange timeout")
			return nil, err
		}
	}
	return nil, nil // should not reach here
}
