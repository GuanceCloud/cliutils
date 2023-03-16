// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package metrics implements datakit's Prometheus metrics

package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricServer used to export metrics via HTTP /metrics request.
type MetricServer struct {
	// Metrics request path.
	URL string `toml:"url" json:"url"`

	// HTTP server address, default to localhost:9090.
	Listen string `toml:"listen" json:"listen"`

	// Enable or disable the http server.
	Enable bool `toml:"enable" json:"enable"`

	// Enable or disable Golang related metrics in metrics URL.
	DisableGoMetrics bool `toml:"disable_go_metrics" json:"disable_go_metrics"`
}

// NewMetricServer create default metric server.
func NewMetricServer() *MetricServer {
	return &MetricServer{
		Enable: true,
		Listen: "localhost:9090",
		URL:    "/metrics",
	}
}

// Start create HTTP server to serving /metrics request.
func (ms *MetricServer) Start() error {
	if !ms.DisableGoMetrics {
		goexporter := collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.MetricsAll))
		MustRegister(goexporter)
	}

	if !ms.Enable {
		return nil
	}

	http.Handle(ms.URL, promhttp.HandlerFor(
		reg,
		promhttp.HandlerOpts{} /*TODO: add options here*/))
	return http.ListenAndServe(ms.Listen, nil)
}
