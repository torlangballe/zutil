//go:build server

package ztelemetry

import (
	"net/url"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zrest"
)

var redirectSecsTelemetry *GaugeVec

func init() {
	zhttp.SetTelemetryForRedirectFunc = SetTelemetryForRedirect
	zrest.HasTelemetryFunc = IsRunning
	zrest.WrapForTelemetryFunc = WrapHandler
}

func EnableRedirectTelemetry() {
	//	redirectSecsTelemetry = NewHistogramVec("http_redirect_seconds", []float64{0.05, 0.2, 2}, "Seconds a redirect took", URLBaseLabel)
	redirectSecsTelemetry = NewGaugeVec("http_redirect_seconds", "Seconds a redirect took", URLBaseLabel)
}

func SetTelemetryForRedirect(surl string, secs float64) {
	if IsRunning() && redirectSecsTelemetry != nil {
		u, _ := url.Parse(surl)
		host := u.Hostname()
		if host != "" {
			labels := map[string]string{URLBaseLabel: host}
			// redirectSecsTelemetry.WithLabelValues(base).Observe(ztime.Since(start))
			redirectSecsTelemetry.Set(secs, labels)
		}
	}
}
