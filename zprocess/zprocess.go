package zprocess

import (
	"context"

	"github.com/torlangballe/zutil/ztime"
)

func RunFuncUntilTimeouSecs(secs float64, do func()) (completed bool) {
	ctx, _ := context.WithTimeout(context.Background(), ztime.SecondsDur(secs))
	return RunFuncUntilContextDone(ctx, do)
}

func RunFuncUntilContextDone(ctx context.Context, do func()) (completed bool) {
	doneChannel := make(chan struct{}, 2)
	go func() {
		do()
		doneChannel <- struct{}{}
	}()
	select {
	case <-doneChannel:
		return true
	case <-ctx.Done():
		return false
	}
}
