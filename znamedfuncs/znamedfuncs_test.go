package znamedfuncs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/torlangballe/zutil/ztesting"
)

type Calls struct{}

type AddStruct struct {
	A int
	B int
}

func (Calls) Add(in AddStruct, out *int) error {
	*out = in.A + in.B
	return nil
}

func TestCall(t *testing.T) {
	executor := NewExecutor()
	executor.Register(Calls{})
	co := CallerInfo{
		CallerID: "testcaller",
		Context:  context.Background(),
	}
	add := AddStruct{A: 3, B: 4}

	var cp CallPayloadReceive
	var rp ReceivePayload
	cp.Args, _ = json.Marshal(add)
	cp.Method = "Calls.Add"
	cp.CallerInfo = co
	cp.TimeToLiveSeconds = 5
	executor.Execute(&cp, &rp)
	if rp.Error != "" {
		t.Fatal("Call 2 returned error:", rp.Error)
	}
	ztesting.Equal(t, string(rp.Result), "7", "Add result not 7")
}
