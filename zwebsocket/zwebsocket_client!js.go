package zwebsocket

import (
	"io"
	"math/rand"
	"time"

	"github.com/torlangballe/zutil/zbytes"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"golang.org/x/net/websocket"
)

type socket interface {
	Send(msg []byte) error
	Receive(msg []byte) error
	Close()
}

type base struct {
	ID           string
	Timeout      time.Duration
	conn         *websocket.Conn
	shutdown     bool
	receiveChans zmap.LockMap[int64, chan message]
	handlerFunc  func(msg []byte, err error) []byte
	url          string
	sendID       bool
	openFunc     func() error
}

type Client struct {
	base
	DefaultTimeToLiveSeconds float64
}

type message struct {
	msg []byte
	err error
	num int64
}

const IDHeader = "X-ZWEBSOCKET-ID"

func NewClient(id, url string, handler func([]byte, error) []byte) (*Client, error) {
	c := &Client{}
	c.shutdown = false
	c.sendID = true
	// zlog.Warn("NewClient:", handler != nil)
	c.handlerFunc = func(msg []byte, err error) []byte {
		return handler(msg, err)
	}
	c.url = url
	c.ID = id
	c.Timeout = time.Second * 10
	c.openFunc = c.open
	err := c.open()
	if err != nil {
		return nil, err
	}
	go c.readForever()
	return c, nil
}

func (b *base) SetHandler(f func(msg []byte, err error) []byte) {
	b.handlerFunc = f
}

func (b *base) Exchange(msg []byte) ([]byte, error) {
	if b.conn == nil {
		return nil, zlog.NewError("no ws connection")
	}
	var outErr error
	num := rand.Int63()
	pre := zbytes.UInt64ToBytes(uint64(num))
	send := append(pre, msg...)
	ch := make(chan message, 1)
	b.receiveChans.Set(num, ch)
	defer b.receiveChans.Remove(num)
	err := websocket.Message.Send(b.conn, send)
	if err != nil {
		outErr = err
		return nil, outErr
	}
	for {
		select {
		case r := <-ch:
			data := r.msg
			outErr = r.err
			return data, outErr
		case <-time.After(b.Timeout):
			outErr = zlog.Error("WebSocket client exchange timeout")
			return nil, outErr
		}
	}
	return nil, outErr
}

func (b *base) Close() {
	b.shutdown = true
	if b.conn != nil {
		b.conn.Close()
	}
	b.conn = nil
}

func (b *base) readForever() {
	first := true
	for !b.shutdown {
		if b.conn == nil {
			zlog.Info("opening ws client to", b.url)
			if b.openFunc != nil {
				b.openFunc()
				time.Sleep(time.Millisecond * 100) // Give it a moment to open before trying to read
			}
			continue
		}
		var msg []byte
		err := websocket.Message.Receive(b.conn, &msg)
		if err != nil {
			if b.shutdown {
				return
			}
			if err == io.EOF && first {
				first = false
				continue
			}
			first = false
			b.handlerFunc(nil, err)
			b.Close()
			continue
		}
		num := int64(zbytes.BytesToUInt64(msg[0:8]))
		n := zint.Abs64(num)
		ch, ok := b.receiveChans.Get(n)
		// zlog.Warn("read:", zlog.Pointer(b), string(msg[8:]), "num:", num, ok)
		if ok { // it was a response, and we have a response channel waiting
			m := message{
				num: num,
				msg: msg[8:],
			}
			ch <- m // send reply which Exchange() is waiting for
			continue
		}
		if num < 0 {
			zlog.Error("Received message with return but no channel waiting:", num)
			continue
		}
		// otherwise, it's a new incoming message, handle and reply
		result := b.handlerFunc(msg[8:], nil)
		pre := zbytes.UInt64ToBytes(uint64(-n))
		err = websocket.Message.Send(b.conn, append(pre, result...))
		// zlog.Warn("post send-reply:", string(result), "num:", num)
		zlog.OnError(err, "sending reply")
		if err != nil {
			b.handlerFunc(nil, err)
			b.Close()
			continue
		}
	}
}

func (c *Client) open() error {
	// origin := "http://localhost"
	origin := "http://localhost?id=abcd"
	config, err := websocket.NewConfig(c.url, origin)
	config.Header.Set(IDHeader, c.ID)
	ws, err := websocket.DialConfig(config)
	if zlog.OnError(err, c.url) {
		c.handlerFunc(nil, err)
		time.Sleep(time.Second)
		return err
	}
	c.conn = ws
	return nil
}
