package zrpc

import "net/http"

// updatedResourcesSentToClient stores Which clients have I sent info about resource being updated to [res][client]bool
var updatedResourcesSentToClient = map[string]map[string]bool{}

func SetResourceUpdated(resID, byClientID string) {
	m := map[string]bool{}
	if byClientID != "" {
		m[byClientID] = true
	}
	updatedResourcesSentToClient[resID] = m
}

func ClearResourceUpdated(resID, clientID string) {
	if updatedResourcesSentToClient[resID] == nil {
		updatedResourcesSentToClient[resID] = map[string]bool{}
	}
	updatedResourcesSentToClient[resID][clientID] = true
}

type RPCCalls CallsBase

func (c *RPCCalls) GetUpdatedResources(req *http.Request, args *Any, reply *[]string) error {
	clientID, err := AuthenticateRequest(req)
	if err != nil {
		return err
	}
	// fmt.Println("GetUpdatedResources", clientID)
	*reply = []string{}
	for res, m := range updatedResourcesSentToClient {
		if m[clientID] == false {
			*reply = append(*reply, res)
			m[clientID] = true
		}
	}
	return nil
}

var Calls = new(RPCCalls)
