package zvnc

import (
	"context"
	"image"
	"net"
	"time"

	//	vnc "github.com/amitbet/vnc2video"
	vnc "github.com/torlangballe/vnc2video"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

// https://www.techrepublic.com/article/how-to-enable-screen-sharing-on-macs-via-terminal/

type Client struct {
	client *vnc.ClientConn
}

func (c *Client) Close() {
	c.client.Close()
}

func Connect(address, password string, updateSecs float64, got func(i image.Image, err error)) (*Client, error) {
	nc, err := net.DialTimeout("tcp", address, 25*time.Second)
	if err != nil || nc == nil {
		return nil, zlog.Error("dial", err)
	}
	// Negotiate connection with the server.
	cchServer := make(chan vnc.ServerMessage)
	cchClient := make(chan vnc.ClientMessage)
	errorCh := make(chan error)
	quitCh := make(chan struct{})

	// zlog.Info("starting up the vnc client, connecting to:", address, "pass:", password)
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

	ticker := time.NewTicker(ztime.SecondsDur(updateSecs))
	var getScreen bool
	cc, err = vnc.Connect(context.Background(), nc, ccfg)
	if err != nil || cc == nil {
		return nil, zlog.Error("connect", err)
	}
	go func() { // because vnc2video.Connect puts error on error channel during setup, we need to do for/select to pop it before calling:
		// defer zlog.LogRecover()
		for {
			select {
			case <-ticker.C:
				// send message to update frame:
				// zlog.Info("en", updateSecs, screenImage.Bounds())
				getScreen = true
				reqMsg := vnc.FramebufferUpdateRequest{Inc: 1, X: 0, Y: 0, Width: cc.Width(), Height: cc.Height()}
				reqMsg.Write(cc)

			case <-quitCh:
				// zlog.Info("quit")
				return

			case err := <-errorCh:
				zlog.Error("VNC error received on channel", err)
				if got != nil {
					got(nil, err)
				}
				return

			case msg := <-cchClient:
				zlog.Info("VNC Received client message type:%v msg:%v\n", msg.Type(), msg)

			case msg := <-cchServer:
				if msg.Type() == vnc.FramebufferUpdateMsgType {
					// zlog.Info("VNC New screen!", getScreen, updateSecs, screenImage.Bounds())
					if getScreen && got != nil {
						got(screenImage, nil)
					}
					getScreen = false
				}
			}
		}
	}()

	screenImage = cc.Canvas
	for _, enc := range ccfg.Encodings {
		myRenderer, ok := enc.(vnc.Renderer)

		if ok {
			myRenderer.SetTargetImage(screenImage)
		}
	}
	zlog.Info("vnc connected to:", address)
	cc.SetEncodings([]vnc.EncodingType{
		vnc.EncCursorPseudo,
		vnc.EncPointerPosPseudo,
		vnc.EncCopyRect,
		vnc.EncTight,
		vnc.EncZRLE,
	})
	c := &Client{client: cc}
	return c, err
	//cc.Wait()
}
