package point

import (
	p8s "github.com/prometheus/client_golang/prometheus"
)

var (
	pointSize = p8s.NewSummary(
		p8s.SummaryOpts{
			Namespace: "pointpool",
			Name:      "point_size",
			Help:      "Byte size of point",
			Objectives: map[float64]float64{
				0.5:  0.05,
				0.9:  0.01,
				0.99: 0.001,
			},
		},
	)

	pointBufCap = p8s.NewSummary(
		p8s.SummaryOpts{
			Namespace: "pointpool",
			Name:      "point_bytes_buf_cap",
			Help:      "Capacity of point's bytes buffer",
			Objectives: map[float64]float64{
				0.5:  0.05,
				0.9:  0.01,
				0.99: 0.001,
			},
		},
	)
)

// Metrics return all exported prometheus metrics of point package.
func Metrics() []p8s.Collector {
	return []p8s.Collector{
		pointBufCap,
		pointSize,
	}
}
