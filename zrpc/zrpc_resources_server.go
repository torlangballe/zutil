package zrpc

import (
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
)

// This is functionality to set that a named resource has changed.
// A RPC function can query for changed resources, and its ClientID is stored so as to
// not report is as changed to that client until updated again.
var (
	updatedResourcesSentToClient zmap.LockMap[string, []string]
)

// GetUpdatedResourcesAndSetSent is called from clients (often browsers) to ask for updated resource-ids
// The client id is stored as it having checked them out for that given update.
func (RPCCalls) GetUpdatedResourcesAndSetSent(ci ClientInfo, int Unused, reply *[]string) error {
	*reply = []string{}
	updatedResourcesSentToClient.ForEach(func(res string, c []string) bool {
		if !zstr.StringsContain(c, ci.ClientID) {
			*reply = append(*reply, res)
			c = append(c, ci.ClientID)
		}
		return true
	})
	return nil
}

// SetResourceUpdatedmarks a given resource as changed. If a client id is given, that client is NOT
// informed the resource changed (presumably because it caused the change).
func SetResourceUpdated(resID, byClientID string) {
	var c []string
	if byClientID != "" {
		c = []string{byClientID}
	}
	// fmt.Println("SetResourceUpdated:", resID, byClientID) //, "\n", zlog.GetCallingStackString())
	updatedResourcesSentToClient.Set(resID, c)
}

// ClearResourceID clears changed status for a resource
func ClearResourceID(resID string) {
	// fmt.Println("ClearResourceID:", resID)
	updatedResourcesSentToClient.Remove(resID)
	// fmt.Printf("ClearResourceID DONE: %s %+v\n", resID, updatedResourcesSentToClient)
}

// SetClientKnowsResourceUpdated sets that a given client now knows resource updated
func SetClientKnowsResourceUpdated(resID, clientID string) {
	// zlog.Info("SetClientKnowsResourceUpdated:", resID, clientID) //, "\n", zlog.GetCallingStackString())
	c, _ := updatedResourcesSentToClient.Get(resID)
	zstr.AddToSet(&c, clientID)
	updatedResourcesSentToClient.Set(resID, c)
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
