package zprocess

import (
	"context"
	"strconv"

	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/zlog"
)

func RunFuncUntilTimeoutSecs(secs float64, do func()) (completed bool) {
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

func SetMaxOpenFileConnections(max int) {
	str, err := RunCommand("ulimit", 5, "-n", strconv.Itoa(max))
	if err != nil {
		zlog.Error(err, str)
	}
}
