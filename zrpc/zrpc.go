// zrpc is a package to perform Remote Procedure Calls over http.
// The server-side registers types with methods: Method(input, *result) error.
// A client calls these by calling err := client.Call("Type.Method", input, &result)
// A method variant with a ClientInfo first argument can also be called.
// The result can be skipped in the actual method, but nil must then be given when doing Call().
//
// See also zrpc_reverse_*.go for a reverse variant, where zrpc is used to ask the server if
// it has any calls it wants to perform on the client, who can also register types/methods.
package zrpc

import (
	"context"
	"encoding/json"
	"reflect"
	"time"
)

// CallPayload is what a call is packaged into and serialized to json
type CallPayload struct {
	ClientID string
	Method   string
	Args     interface{}
	Token    string `json:",omitempty"`
	Expires  time.Time
}

// callPayloadReceive is what received. It has to have same named fields as callPayload
// It is necessary as Args needs to be a json.RawMessage.
type callPayloadReceive struct {
	ClientID string
	Method   string
	Args     json.RawMessage
	Token    string    `json:",omitempty"`
	Expires  time.Time // Used for reverse calls to time out
}

// receivePayload is what the result of the call is returned in.
type receivePayload struct {
	Result                any
	Error                 string         `json:",omitempty"`
	TransportError        TransportError `json:",omitempty"`
	AuthenticationInvalid bool
}

// methodType stores the information of each method of each registered type
type methodType struct {
	Receiver      reflect.Value
	Method        reflect.Method
	hasClientInfo bool
	ArgType       reflect.Type
	ReplyType     reflect.Type
	AuthNotNeeded bool
}

// ClientInfo stores information about the client calling.
type ClientInfo struct {
	Type      string    // Type is zrpc or zrpc-rev for these calls. Might be something else if used elsewhere.
	ClientID  string    // ClientID identifies the client
	Token     string    `json:",omitempty"` // Token can be any token, or a authentication token needed to allow the call
	UserAgent string    `json:",omitempty"` // From the http request
	IPAddress string    `json:",omitempty"` // From the http request
	SendDate  time.Time `json:",omitempty"` // From the http requests Date header
	Context   context.Context
}

// TransportError is a specific error type. Any problem with the actual transport of an zrpc call is
// returned as it, so we can check if it's an error returned from the call, or a problem calling.
type TransportError string

type CallsBase struct{} // CallsBase is just a dummy type one can derive from when defining a type to add methods to for registation. You don't need to use it.
type RPCCalls CallsBase // RPCCalls is the type with zrpc's own build-in methods.
type Unused struct{}    // Any is used in function definition args/result when argument is not used

var ExecuteTimedOutError = TransportError("Execution timed out")

func (t TransportError) Error() string {
	return string(t)
}
