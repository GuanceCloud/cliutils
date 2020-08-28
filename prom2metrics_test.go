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

func TestProm2Metrics(t *testing.T) {
	const measurementPrefix = "testing"
	const defaultMeasurement = "default_testing"

	pts, err := PromTextToMetrics(strings.NewReader(data1), measurementPrefix, defaultMeasurement, time.Now())
	if err != nil {
		panic(err)
	}

	for _, pt := range pts {
		t.Log(pt)
	}
	// output:
	// testing_go_gc,quantile=0.000 go_gc_duration_seconds=0.000074545 1598611000473403377
	// testing_go_gc,quantile=0.250 go_gc_duration_seconds=0.000076999 1598611000473403377
	// testing_go_gc,quantile=0.500 go_gc_duration_seconds=0.000277935 1598611000473403377
	// testing_go_gc,quantile=0.750 go_gc_duration_seconds=0.000706591 1598611000473403377
	// testing_go_gc,quantile=1.000 go_gc_duration_seconds=0.000706591 1598611000473403377
	// testing_go_gc go_gc_duration_seconds_count=4i,go_gc_duration_seconds_sum=0.00113607 1598611000473403377
	// default_testing go_goroutines=15 1598611000473403377
	// testing_cpu_usage,cpu=cpu0 cpu_usage_user=1.4112903225816156 1598611000473403377
	// testing_cpu_usage,cpu=cpu1 cpu_usage_user=0.702106318955865 1598611000473403377
	// testing_cpu_usage,cpu=cpu2 cpu_usage_user=2.0161290322588776 1598611000473403377
	// testing_cpu_usage,cpu=cpu3 cpu_usage_user=1.5045135406226022 1598611000473403377

	pts2, err := PromTextToMetrics(strings.NewReader(data2), measurementPrefix, defaultMeasurement, time.Now())
	if err != nil {
		panic(err)
	}

	for _, pt := range pts2 {
		t.Log(pt)
	}
	// output:
	// testing_confluence_request,path=/rest confluence_request_duration_on_path_count=6i,confluence_request_duration_on_path_sum=2.336312921 1598611146774553251
	// testing_confluence_request,le=0.005,path=/rest confluence_request_duration_on_path_bucket=0i 1598611146774553251
	// testing_confluence_request,le=0.010,path=/rest confluence_request_duration_on_path_bucket=4i 1598611146774553251
	// testing_confluence_request,le=0.025,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.050,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.075,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.100,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.250,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.500,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.750,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=1.000,path=/rest confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=2.500,path=/rest confluence_request_duration_on_path_bucket=6i 1598611146774553251
	// testing_confluence_request,le=5.000,path=/rest confluence_request_duration_on_path_bucket=6i 1598611146774553251
	// testing_confluence_request,le=7.500,path=/rest confluence_request_duration_on_path_bucket=6i 1598611146774553251
	// testing_confluence_request,le=10.000,path=/rest confluence_request_duration_on_path_bucket=6i 1598611146774553251
	// testing_confluence_request,le=+Inf,path=/rest confluence_request_duration_on_path_bucket=6i 1598611146774553251
	// testing_confluence_request,path=/plugins confluence_request_duration_on_path_count=5i,confluence_request_duration_on_path_sum=0.971921824 1598611146774553251
	// testing_confluence_request,le=0.005,path=/plugins confluence_request_duration_on_path_bucket=0i 1598611146774553251
	// testing_confluence_request,le=0.010,path=/plugins confluence_request_duration_on_path_bucket=0i 1598611146774553251
	// testing_confluence_request,le=0.025,path=/plugins confluence_request_duration_on_path_bucket=0i 1598611146774553251
	// testing_confluence_request,le=0.050,path=/plugins confluence_request_duration_on_path_bucket=1i 1598611146774553251
	// testing_confluence_request,le=0.075,path=/plugins confluence_request_duration_on_path_bucket=1i 1598611146774553251
	// testing_confluence_request,le=0.100,path=/plugins confluence_request_duration_on_path_bucket=1i 1598611146774553251
	// testing_confluence_request,le=0.250,path=/plugins confluence_request_duration_on_path_bucket=3i 1598611146774553251
	// testing_confluence_request,le=0.500,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=0.750,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=1.000,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=2.500,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=5.000,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=7.500,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=10.000,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// testing_confluence_request,le=+Inf,path=/plugins confluence_request_duration_on_path_bucket=5i 1598611146774553251
	// default_testing confluence_total_cluster_nodes_gauge=0 1598611146774553251
	// testing_confluence_user,username=admin confluence_user_logout_count=2 1598611146774553251
}
