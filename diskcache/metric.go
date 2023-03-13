// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	droppedBatchVec,
	rotateVec,
	removeVec,
	putVec,
	getVec,
	putBytesVec,
	getBytesVec *prometheus.CounterVec

	sizeVec,
	openTimeVec,
	lastCloseTimeVec,
	datafilesVec *prometheus.GaugeVec

	getLatencyVec,
	putLatencyVec *prometheus.SummaryVec

	ns = "diskcache"
)

func setupMetrics() {
	var labels = []string{
		// NOTE: make them sorted.
		"batch_size",
		"capacity",
		"max_data_size",
		"no_lock",
		"no_pos",
		"no_sync",
		"path",
	}

	getLatencyVec = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: ns,
			Name:      "get_latency",
			Help:      "Get() time cost(micro-second)",
		},
		labels,
	)

	putLatencyVec = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: ns,
			Name:      "put_latency",
			Help:      "Put() time cost(micro-second)",
		},
		labels,
	)

	droppedBatchVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "dropped_total",
			Help:      "dropped files during Put() when capacity reached.",
		},
		labels,
	)

	rotateVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "rotate_total",
			Help:      "cache rotate count, mean file rotate from data to data.0000xxx",
		},
		labels,
	)

	removeVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "remove_total",
			Help:      "removed file count, if some file read EOF, remove it from un-readed list",
		},
		labels,
	)

	putVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "put_total",
			Help:      "cache Put() count",
		},
		labels,
	)

	putBytesVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "put_bytes_total",
			Help:      "cache Put() bytes count",
		},
		labels,
	)

	getVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "get_total",
			Help:      "cache Get() count",
		},
		labels,
	)

	getBytesVec = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: ns,
			Name:      "get_bytes_total",
			Help:      "cache Get() bytes count",
		},
		labels,
	)

	sizeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "size",
			Help:      "current cache size(in bytes)",
		},
		labels,
	)

	openTimeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "open_time",
			Help:      "current cache Open time in unix timestamp(second)",
		},
		labels,
	)

	lastCloseTimeVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "last_close_time",
			Help:      "current cache last Close time in unix timestamp(second)",
		},
		labels,
	)

	datafilesVec = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: ns,
			Name:      "datafiles",
			Help:      "current un-readed data files",
		},
		labels,
	)

	metrics.MustRegister(
		droppedBatchVec,
		rotateVec,
		putVec,
		getVec,
		putBytesVec,
		getBytesVec,

		openTimeVec,
		lastCloseTimeVec,
		sizeVec,
		datafilesVec,

		getLatencyVec,
		putLatencyVec)
}

// register to specified registry for testing
func register(reg *prometheus.Registry) {
	reg.MustRegister(
		droppedBatchVec,
		rotateVec,
		putVec,
		getVec,
		putBytesVec,
		getBytesVec,

		sizeVec,
		datafilesVec,

		getLatencyVec,
		putLatencyVec)
}

func resetMetrics() {
	droppedBatchVec.Reset()
	rotateVec.Reset()
	putVec.Reset()
	getVec.Reset()
	putBytesVec.Reset()
	getBytesVec.Reset()
	sizeVec.Reset()
	datafilesVec.Reset()
	getLatencyVec.Reset()
	putLatencyVec.Reset()
}

// nolint: gochecknoinits
func init() {
	setupMetrics()
}
