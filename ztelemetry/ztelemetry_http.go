//go:build server

package ztelemetry

import (
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zrest"
)

var redirectSecsTelemetry *GaugeVec

func init() {
	zhttp.SetTelemetryForRedirectFunc = SetTelemetryForRedirect
	zrest.HasTelemetryFunc = IsRunning
	zrest.WrapForTelemetryFunc = WrapHandler
}

func EnableRedictTelemetry() {
	//	redirectSecsTelemetry = NewHistogramVec("http_redirect_seconds", []float64{0.05, 0.2, 2}, "Seconds a redirect took", URLBaseLabel)
	redirectSecsTelemetry = NewGaugeVec("http_redirect_seconds", "Seconds a redirect took", URLBaseLabel)
}

func SetTelemetryForRedirect(surl string, secs float64) {
	if IsRunning() && redirectSecsTelemetry != nil {
		base := znet.StripQueryAndFragment(surl)
		labels := map[string]string{URLBaseLabel: base}
		// redirectSecsTelemetry.WithLabelValues(base).Observe(ztime.Since(start))
		redirectSecsTelemetry.Set(secs, labels)
	}
}
