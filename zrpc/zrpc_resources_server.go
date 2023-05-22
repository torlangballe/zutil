package zrpc

import (
	"fmt"
	"sync"

	"github.com/torlangballe/zutil/zhttp"
)

// This is functionality to set in the server if a named resource has changed.
// A RPC function can query for changed resources, and its ClientID is stored so as to
// not report is as changed to that client until updated again.
var (
	updatedResourcesSentToClient = map[string]map[string]bool{}
	updatedResourcesMutex        sync.Mutex
)

// GetUpdatedResourcesAndSetSent is called from clients (often browsers) to ask for updated resource-ids
// The client id is stored as it having checked them out for that given update.
func (RPCCalls) GetUpdatedResourcesAndSetSent(ci ClientInfo, in Unused, reply *[]string) error {
	fmt.Println("GetUpdatedResourcesAndSetSent", ci.ClientID)
	// zlog.Info("GetUpdatedResourcesAndSetSent", clientID, updatedResourcesSentToClient)
	*reply = []string{}
	updatedResourcesMutex.Lock()
	for res, m := range updatedResourcesSentToClient {
		if !m[ci.ClientID] {
			*reply = append(*reply, res)
			m[ci.ClientID] = true
		}
	}
	updatedResourcesMutex.Unlock()
	// zlog.Info("GetUpdatedResources Got", *reply)
	return nil
}

// SetResourceUpdatedmarks a given resource as changed. If a client id is given, that client is NOT
// informed the resource changed (presumably because it caused the change).
func SetResourceUpdated(resID, byClientID string) {
	m := map[string]bool{}
	if byClientID != "" {
		m[byClientID] = true
	}
	updatedResourcesMutex.Lock()
	// fmt.Println("SetResourceUpdated:", resID, byClientID) //, "\n", zlog.GetCallingStackString())
	updatedResourcesSentToClient[resID] = m
	updatedResourcesMutex.Unlock()
}

// ClearResourceID clears changed status for a resource
func ClearResourceID(resID string) {
	updatedResourcesMutex.Lock()
	// fmt.Println("ClearResourceID:", resID)
	updatedResourcesSentToClient[resID] = map[string]bool{}
	// fmt.Printf("ClearResourceID DONE: %s %+v\n", resID, updatedResourcesSentToClient)
	updatedResourcesMutex.Unlock()
}

// SetClientKnowsResourceUpdated sets that a given client now knows resource updated
func SetClientKnowsResourceUpdated(resID, clientID string) {
	// zlog.Info("SetClientKnowsResourceUpdated:", resID, clientID) //, "\n", zlog.GetCallingStackString())
	updatedResourcesMutex.Lock()
	if updatedResourcesSentToClient[resID] == nil {
		updatedResourcesSentToClient[resID] = map[string]bool{}
	}
	updatedResourcesSentToClient[resID][clientID] = true
	updatedResourcesMutex.Unlock()
}

// SetResourceUpdatedFromClient is called from client to say it knows of update
func (RPCCalls) SetResourceUpdatedFromClient(ci ClientInfo, resID string) error {
	// fmt.Println("SetResourceUpdatedFromClient:", *resID)
	SetResourceUpdated(resID, ci.ClientID)
	return nil
}

// GetURL is a convenience function to get the contents of a url via the server.
func (RPCCalls) GetURL(surl *string, reply *[]byte) error {
	params := zhttp.MakeParameters()
	_, err := zhttp.Get(*surl, params, reply)
	return err
}
