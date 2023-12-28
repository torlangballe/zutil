package zgraphana

import (
	"runtime"
	"time"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zkeyvalrpc"
	"github.com/torlangballe/zutil/zlog"
)

type Annotation struct {
	DashboardUID string    `json:"dashboardUID"`
	PanelID      int       `json:"panelId"`
	Time         time.Time `json:"-"` // is converted to TimeEpocMS
	TimeEnd      time.Time `json:"-"` // is converted to TimeEndEpocMS
	Tags         []string  `json:"tags"`
	Text         string    `json:"text"`

	TimeEpocMS    int64 `json:"time"`
	TimeEndEpocMS int64 `json:"timeEnd"`
}

type GraphanaResult struct {
	Message string `json:"message"`
	ID      int    `json:"id"`
}

var (
	APIKey       = zkeyvalrpc.NewOption[string]("GraphanaAPIKey", "")
	URLPrefix    = zkeyvalrpc.NewOption[string]("GraphanaURLPrefix", "")
	DashboardUID = zkeyvalrpc.NewOption[string]("GraphanaDashboardUID", "")
)

func SetAnnotation(graphanaAddress string, a Annotation) {
	if runtime.GOOS == "darwin" {
		return
	}
	APIKey.Set("eyJrIjoicW5xR2VFaTBHTkZ5ZlV6clhhWTRNM04wSUlRU0dRZTEiLCJuIjoiQW5ub3RhdGlvbiIsImlkIjoxfQ==", false)
	var got GraphanaResult
	if !a.Time.IsZero() {
		a.TimeEpocMS = a.Time.UnixMilli()
	}
	if !a.TimeEnd.IsZero() {
		a.TimeEndEpocMS = a.TimeEnd.UnixMilli()
	}
	surl := graphanaAddress + "/api/annotations"
	params := zhttp.MakeParameters()
	// params.PrintBody = true
	key := APIKey.Get()
	params.Headers["Authorization"] = "Bearer " + key
	// zlog.Info("ANNO:", params.Headers)
	_, err := zhttp.Post(surl, params, a, &got)
	if zlog.OnError(err, "annot") {
		return
	}
	// zlog.Info("SatAnno:", got)
}
