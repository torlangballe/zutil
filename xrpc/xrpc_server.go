//go:build !js

package xrpc

import (
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/zwebsocket"
)

func (r *RPC) MakeServer(path string, port int) (*zwebsocket.Server, error) {
	handler := func(id string, msg []byte, err error) []byte {
		if err != nil {
			// zlog.Warn("RPC server got error from websocket connection", id, err)
			return nil
		}
		ci := znamedfuncs.CallerInfo{
			CallerID: id,
		}
		var result []byte
		err = r.Executor.ExecuteFromToJSON(msg, &result, ci)
		zlog.OnError(err, "RPC server call execute error", msg)
		return result
	}
	return zwebsocket.NewServer(path, port, handler)
}

func init() {
	exchangeWithServerFunc = func(r *RPC, pipeID string, cpJson []byte) (rpJson []byte, err error) {
		var server *zwebsocket.Server
		// zlog.Warn("RPC Server Call to pipeID:", pipeID, len(r.servers))
		if len(r.servers) == 0 {
			return nil, zlog.NewError("RPC Call with no server and no client for pipeID:", pipeID)
		}
		if pipeID == "" {
			if len(r.servers) == 1 {
				for _, s := range r.servers {
					if len(s.connection.Connections) == 1 {
						pipeID = s.connection.Connections[0].ID
						server = s.connection
						break
					}
				}
			}
		} else {
			for _, s := range r.servers {
				if s.connection == nil {
					continue
				}
				for _, c := range s.connection.Connections {
					if c.ID == pipeID {
						server = s.connection
						break
					}
				}
			}
		}
		if server == nil {
			// for _, s := range r.servers {
			// 	zlog.Warn("ServC:", s.connection != nil, zlog.Pointer(s))
			// }
			return nil, zlog.NewError("RPC Call with no id and not just one client or connection", pipeID)
		}
		return server.ExchangeWithID(pipeID, cpJson)
	}
}
