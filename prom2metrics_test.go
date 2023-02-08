// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package cliutils

import (
	"strings"
	"testing"
	"time"
)

const data1 = `
# HELP go_gc_duration_seconds A summary of the GC invocation durations.
# TYPE go_gc_duration_seconds summary
go_gc_duration_seconds{quantile="0"} 7.4545e-05
go_gc_duration_seconds{quantile="0.25"} 7.6999e-05
go_gc_duration_seconds{quantile="0.5"} 0.000277935
go_gc_duration_seconds{quantile="0.75"} 0.000706591
go_gc_duration_seconds{quantile="1"} 0.000706591
go_gc_duration_seconds_sum 0.00113607
go_gc_duration_seconds_count 4
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 15
# HELP cpu_usage_user Telegraf collected metric
# TYPE cpu_usage_user gauge
cpu_usage_user{cpu="cpu0"} 1.4112903225816156
cpu_usage_user{cpu="cpu1"} 0.702106318955865
cpu_usage_user{cpu="cpu2"} 2.0161290322588776
cpu_usage_user{cpu="cpu3"} 1.5045135406226022
`

const data2 = `
# HELP confluence_user_logout_count User Logout Count
# TYPE confluence_user_logout_count counter
confluence_user_logout_count{username="admin",ip="",} 2.0
# HELP confluence_user_failed_login_count User Failed Login Count
# TYPE confluence_user_failed_login_count counter
# HELP confluence_request_duration_on_path Request duration on path
# TYPE confluence_request_duration_on_path histogram
confluence_request_duration_on_path_bucket{path="/rest",le="0.005",} 0.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.01",} 4.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.025",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.05",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.075",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.1",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.25",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.5",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="0.75",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="1.0",} 5.0
confluence_request_duration_on_path_bucket{path="/rest",le="2.5",} 6.0
confluence_request_duration_on_path_bucket{path="/rest",le="5.0",} 6.0
confluence_request_duration_on_path_bucket{path="/rest",le="7.5",} 6.0
confluence_request_duration_on_path_bucket{path="/rest",le="10.0",} 6.0
confluence_request_duration_on_path_bucket{path="/rest",le="+Inf",} 6.0
confluence_request_duration_on_path_count{path="/rest",} 6.0
confluence_request_duration_on_path_sum{path="/rest",} 2.336312921
confluence_request_duration_on_path_bucket{path="/plugins",le="0.005",} 0.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.01",} 0.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.025",} 0.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.05",} 1.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.075",} 1.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.1",} 1.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.25",} 3.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.5",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="0.75",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="1.0",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="2.5",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="5.0",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="7.5",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="10.0",} 5.0
confluence_request_duration_on_path_bucket{path="/plugins",le="+Inf",} 5.0
confluence_request_duration_on_path_count{path="/plugins",} 5.0
confluence_request_duration_on_path_sum{path="/plugins",} 0.971921824
# HELP confluence_total_cluster_nodes_gauge Total Cluster Nodes Gauge
# TYPE confluence_total_cluster_nodes_gauge gauge
confluence_total_cluster_nodes_gauge 0.0
`

const data3 = `
# HELP jvm_gc_collection_seconds Time spent in a given JVM garbage collector in seconds.
# TYPE jvm_gc_collection_seconds summary
jvm_gc_collection_seconds_count{gc="G1 Young Generation",} 129.0
jvm_gc_collection_seconds_sum{gc="G1 Young Generation",} 4.615
jvm_gc_collection_seconds_count{gc="G1 Old Generation",} 0.0
jvm_gc_collection_seconds_sum{gc="G1 Old Generation",} 0.0
`

func TestProm2Metrics(t *testing.T) {
	const measurementPrefix = "confluence"
	const defaultMeasurement = "confluence"

	data := strings.NewReader(data1)
	// data := strings.NewReader(data2)
	// data := strings.NewReader(data3)
	pts, err := PromTextToMetrics(data, measurementPrefix, defaultMeasurement, time.Now())
	if err != nil {
		panic(err)
	}

	for _, pt := range pts {
		t.Log(pt)
	}
}
