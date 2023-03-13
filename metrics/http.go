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

type MetricServer struct {
	URL    string
	Listen string

	Enable           bool
	DisableGoMetrics bool
}

func NewMetricServer() *MetricServer {
	return &MetricServer{
		Enable: true,
		Listen: "localhost:9539",
		URL:    "/metrics",
	}
}

// Start start new metric HTTP server.
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
