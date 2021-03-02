package zvnc

import (
	"context"
	"fmt"
	"net"
	"time"

	vnc "github.com/amitbet/vnc2video"
	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

type Client struct {
	client *vnc.ClientConn
}

func (c *Client) Close() {
	c.client.Close()
}

func Connect(address, password string, updateSecs float64, got func(i *zui.Image, err error)) (*Client, error) {
	nc, err := net.DialTimeout("tcp", address, 25*time.Second)
	if err != nil {
		return nil, zlog.Error(err, "dial")
	}
	fmt.Println("starting up the vnc client, connecting to:", address)
	// Negotiate connection with the server.
	cchServer := make(chan vnc.ServerMessage)
	cchClient := make(chan vnc.ClientMessage)
	errorCh := make(chan error)
	quitCh := make(chan struct{})

	ccfg := &vnc.ClientConfig{
		SecurityHandlers: []vnc.SecurityHandler{
			// &vnc.ClientAuthATEN{Username: []byte(os.Args[2]), Password: []byte(os.Args[3])},
			&vnc.ClientAuthVNC{Password: []byte(password)},
			&vnc.ClientAuthNone{},
		},
		DrawCursor:      false,
		PixelFormat:     vnc.PixelFormat32bit,
		ClientMessageCh: cchClient,
		ServerMessageCh: cchServer,
		Messages:        vnc.DefaultServerMessages,
		Encodings: []vnc.Encoding{
			&vnc.RawEncoding{},
			&vnc.TightEncoding{},
			&vnc.HextileEncoding{},
			&vnc.ZRLEEncoding{},
			&vnc.CopyRectEncoding{},
			&vnc.CursorPseudoEncoding{},
			&vnc.CursorPosPseudoEncoding{},
			&vnc.ZLibEncoding{},
			&vnc.RREEncoding{},
		},
		ErrorCh: errorCh,
		QuitCh:  quitCh,
	}
	var screenImage *vnc.VncCanvas
	var cc *vnc.ClientConn

	go func() { // because vnc2video.Connect puts error on error channel during setup, we need to do for/select to pop it before calling:
		// defer zlog.LogRecover()
		for {
			select {
			case <-quitCh:
				// zlog.Info("quit")
				return
			case err := <-errorCh:
				// zlog.Info(err, "error received on channel")
				if got != nil {
					got(nil, err)
				}
				return
			case msg := <-cchClient:
				fmt.Printf("Received client message type:%v msg:%v\n", msg.Type(), msg)
			case msg := <-cchServer:
				// fmt.Println("MSG:", msg)
				if msg.Type() == vnc.FramebufferUpdateMsgType {
					// secsPassed := time.Now().Sub(timeStart).Seconds()
					// frameBufferReq++
					// reqPerSec := float64(frameBufferReq) / secsPassed
					// fmt.Println("New screen!", screenImage.Bounds())
					if got != nil {
						image := zui.ImageFromGo(screenImage)
						got(image, nil)
					}
					// zlog.Info("Start new vnc fetch", updateSecs)
					ztimer.StartIn(updateSecs, func() {
						reqMsg := vnc.FramebufferUpdateRequest{Inc: 1, X: 0, Y: 0, Width: cc.Width(), Height: cc.Height()}
						//cc.ResetAllEncodings()
						reqMsg.Write(cc)
					})
				}
			}
		}
	}()

	cc, err = vnc.Connect(context.Background(), nc, ccfg)
	//	zlog.Info("Here:", err)
	if err != nil {
		return nil, zlog.Error(err, "connect")
	}
	screenImage = cc.Canvas
	for _, enc := range ccfg.Encodings {
		myRenderer, ok := enc.(vnc.Renderer)

		if ok {
			myRenderer.SetTargetImage(screenImage)
		}
	}
	// var out *os.File

	fmt.Println("vnc connected to:", address)

	cc.SetEncodings([]vnc.EncodingType{
		vnc.EncCursorPseudo,
		vnc.EncPointerPosPseudo,
		vnc.EncCopyRect,
		vnc.EncTight,
		vnc.EncZRLE,
		//vnc.EncHextile,
		//vnc.EncZlib,
		//vnc.EncRRE,
	})
	c := &Client{client: cc}
	return c, err
	//cc.Wait()
}
