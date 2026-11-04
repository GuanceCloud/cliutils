// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.
package point

import (
	"github.com/GuanceCloud/cliutils/metrics"
	p8s "github.com/prometheus/client_golang/prometheus"
)

var (
	pointCheckWarnVec = p8s.NewCounterVec(
		p8s.CounterOpts{
			Namespace: "point",
			Name:      "check_point_warning_total",
			Help:      "Warnings among build new point",
		},
		[]string{"type"},
	)
)

func ResetMetrics() {
	pointCheckWarnVec.Reset()
}

func init() {
	metrics.MustRegister(pointCheckWarnVec)
}
