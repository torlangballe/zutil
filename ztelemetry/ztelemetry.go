package ztelemetry

import (
	"github.com/torlangballe/zutil/zkeyvalrpc"
)

var PrometheusPort = zkeyvalrpc.NewOption[int]("PrometheusPort", 9090)

const (
	IPAddressLabel        = "ip_address"
	NetworkInterfaceLabel = "network-interface"
)
