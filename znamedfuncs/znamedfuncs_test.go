package znamedfuncs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/torlangballe/zutil/zlog"
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
	raw, _ := json.Marshal(add)
	rp, err := callMethodName(executor, co, "Calls.Add", raw)
	if err != nil {
		t.Fatal("Call failed:", err)
	}
	if rp.Error != "" {
		t.Fatal("Call returned error:", rp.Error)
	}
	var data []byte
	data, err = json.Marshal(rp.Result)
	if err != nil {
		t.Fatal("Unmarshal result failed:", err)
	}
	zlog.Warn("Result1:", string(data), rp.Error)

	var cp CallPayload
	var rp2 ReceivePayload
	cp.Args, _ = json.Marshal(add)
	cp.Method = "Calls.Add"
	cp.CallerInfo = co
	cp.TimeToLiveSeconds = 5
	executor.Execute(&cp, &rp2, co)
	if rp2.Error != "" {
		t.Fatal("Call 2 returned error:", rp2.Error)
	}
	zlog.Warn("Result2:", string(rp2.Result))
}
