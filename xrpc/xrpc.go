package xrpc

import (
	"encoding/json"
	"fmt"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/zwebsocket"
)

type RPC struct {
	exchangers               zwebsocket.Exchangers
	executor                 *znamedfuncs.Executor
	server                   *zwebsocket.Server
	HandleWebSocketErrorFunc func(id string, err error)
}

func NewRPC() *RPC {
	r := &RPC{}
	r.exchangers = zwebsocket.Exchangers{}
	return r
}

func (r *RPC) SetExecutor(ex *znamedfuncs.Executor) {
	r.executor = ex
}

func (r *RPC) AddExchanger(id string, exchanger zwebsocket.Exchanger) {
	r.exchangers[id] = exchanger
}

func (r *RPC) SetServer(path string, port int, pipeID string) {
	handler := func(id string, msg []byte, err error) []byte {
		if err != nil {
			if r.HandleWebSocketErrorFunc != nil {
				r.HandleWebSocketErrorFunc(id, err)
			}
			return nil
		}
		ci := znamedfuncs.CallerInfo{
			CallerID: id,
		}
		var result []byte
		err = r.executor.ExecuteFromToJSON(msg, &result, ci)
		zlog.OnError(err, "RPC server call execute error", msg)
		return result
	}
	r.server = zwebsocket.NewServer(pipeID, path, port, handler)
}

func (r *RPC) AddClient(url, pipeID string) (*zwebsocket.Client, error) {
	client, err := zwebsocket.NewClient(pipeID, url, nil)
	if err != nil {
		return nil, err
	}
	r.AddExchanger(pipeID, client)
	handler := func(msg []byte, err error) []byte {
		if err != nil {
			if r.HandleWebSocketErrorFunc != nil {
				r.HandleWebSocketErrorFunc(pipeID, err)
			}
			return nil
		}
		ci := znamedfuncs.CallerInfo{
			CallerID: pipeID,
		}
		ci.TimeToLiveSeconds = client.DefaultTimeToLiveSeconds
		var result []byte
		err = r.executor.ExecuteFromToJSON(msg, &result, ci)
		zlog.OnError(err, "RPC client call execute error", msg)
		return result
	}
	client.SetHandler(handler)
	return client, nil
}

func (r *RPC) Call(pipeID string, fullMethod string, in any, resultPtr any, cis ...znamedfuncs.CallerInfo) error {
	var cp znamedfuncs.CallPayloadSend
	cp.Method = fullMethod
	if len(cis) > 0 {
		cp.CallerInfo = cis[0]
	}
	var err error
	cp.CallerInfo.CallerID = pipeID
	cp.Args = in
	if err != nil {
		return err
	}
	cpJson, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if pipeID == "" {
		if len(r.exchangers) != 1 {
			return fmt.Errorf(`pipeID=="" but 0/multiple exchangers`)
		}
		pipeID = zmap.GetAnyKeyAsString(r.exchangers)
	}
	rpJson, err := r.exchangers.Exchange(pipeID, cpJson)
	if err == zwebsocket.ExchangerNotFound {
		err = nil
		if r.server != nil {
			if pipeID == "" {
				if len(r.server.Connections) > 1 {
					return fmt.Errorf(`pipeID=="" but multiple server connections`)
				}
				if len(r.server.Connections) == 1 {
					pipeID = r.server.Connections[0].ID
				} else {
					pipeID = r.server.ID
				}
			}
			if r.executor != nil && r.server.ID == pipeID {
				err = r.executor.ExecuteFromToJSON(cpJson, &rpJson, cp.CallerInfo)
			} else {
				rpJson, err = r.server.Exchange(pipeID, cpJson)
			}
		}
	}
	if err != nil {
		if r.HandleWebSocketErrorFunc != nil {
			r.HandleWebSocketErrorFunc(pipeID, err)
		}
		return err
	}
	var rp znamedfuncs.ReceivePayload
	err = json.Unmarshal(rpJson, &rp)
	if err != nil {
		return zlog.NewError(err, "unmarshal RP failed")
	}
	if resultPtr != nil {
		err = json.Unmarshal(rp.Result, resultPtr)
		if err != nil {
			return zlog.NewError(err, "unmarshal RP.Result payload failed")
		}
	}
	if rp.Error != "" {
		return fmt.Errorf("RPC call result error: %s", rp.Error)
	}
	if rp.TransportError != "" {
		return fmt.Errorf("RPC call transport error: %s", rp.TransportError)
	}
	return nil
}
