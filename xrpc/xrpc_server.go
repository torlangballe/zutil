//go:build !js

package xrpc

import (
	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/zwebsocket"
)

func (r *RPC) MakeServer(path string, port int, router *mux.Router) (*zwebsocket.Server, error) {
	handler := func(id string, msg []byte, err error) []byte {
		// zlog.Info("RPC server got message from websocket connection", id, len(msg), err)
		if err != nil {
			// zlog.Warn("RPC server got error from websocket connection", id, err)
			return nil
		}
		ci := znamedfuncs.ClientInfo{
			ClientID: id,
		}
		var result []byte
		err = r.Executor.ExecuteFromToJSON(msg, &result, ci, r.targetID)
		zlog.OnError(err, "RPC server call execute error", msg)
		return result
	}
	address := "ws://" + path
	address = path
	zlog.Info("Starting RPC server on", address, "with port", port)
	if router != nil {
		return zwebsocket.NewServerWithRouter(address, router, handler)
	}
	return zwebsocket.NewServer(address, port, handler)
}

func init() {
	exchangeWithServerFunc = func(r *RPC, pipeID string, cpJson []byte) (rpJson []byte, err error) {
		var server *zwebsocket.Server
		if r.servers.Count() == 0 {
			return nil, zlog.NewError("RPC Call with no server and no client for pipeID:", pipeID)
		}
		if pipeID == "" {
			if r.servers.Count() == 1 {
				r.servers.ForAll(func(id string, s *ConnectInfo[zwebsocket.Server]) {
					if len(s.connection.Connections) == 1 {
						pipeID = s.connection.Connections[0].ID
						server = s.connection
						return
					}
				})
			}
		} else {
			r.servers.ForEach(func(id string, s *ConnectInfo[zwebsocket.Server]) bool {
				if s.connection == nil {
					return true
				}
				for i, c := range s.connection.Connections {
					zlog.Info("handleServerConnectionError: looking for pipeID", i, pipeID, "in server", id, "with connections", c.ID)
					if c.ID == pipeID {
						server = s.connection
						return false
					}
				}
				return true
			})
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

func (r *RPC) handleServerConnectionError(pipeID string, err error) {
	zlog.Info("handleServerConnectionError", pipeID, err)
	s := r.servers.Index(pipeID)
	if s != nil && s.connection != nil {
		s.connection.Close()
		s.connection = nil
	}
}
