package ztelemetry

import (
	"fmt"
	"strings"
)

type metricInfo struct {
	stype      string
	name       string
	help       string
	labelNames []string
}

var (
	PrometheusPort    = 9090
	metricsList       []metricInfo
	labelDescriptions = map[string]string{}
)

const (
	IPAddressLabel        = "ip_address"
	NetworkInterfaceLabel = "network-interface"
)

func addMetricType(stype, name, help string, labelNames ...string) {
	metric := metricInfo{
		stype:      stype,
		name:       name,
		help:       help,
		labelNames: labelNames,
	}
	metricsList = append(metricsList, metric)
}

func AddLabelDescription(label, description string) {
	labelDescriptions[label] = description
}

func GetDocumentationMD() string {
	str :=
		`
### Types of metrics:
* **Gage:**
	A numerical value that can go up and down such as temperature.
* **Counter:**
	A cumulative metric that represents a single numerical value that only ever goes up.

### Possible labels:
`
	for label, description := range labelDescriptions {
		str += fmt.Sprintf("* **%s:** *%s*\n", label, description)
	}
	str += `
### Provided metrics:
`
	for _, m := range metricsList {
		str += fmt.Sprintf("* **%s:** (%s)\n", m.name, m.stype)
		str += fmt.Sprintf("	%s.\n", m.help)
		if len(m.labelNames) > 0 {
			labels := strings.Join(m.labelNames, ", ")
			str += fmt.Sprintf("	*Labels: %v*\n", labels)
		}
	}
	return str
}
