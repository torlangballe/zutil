package zkeyvalrpc

import "github.com/torlangballe/zutil/zmap"

const ResourceID = "keyvalues-rpc"

var externalChangeHandlers zmap.LockMap[string, func(key string, value any, isLoad bool)]

func AddExternalChangedHandler(key string, handler func(key string, value any, isLoad bool)) {
	externalChangeHandlers.Set(key, handler)
}
