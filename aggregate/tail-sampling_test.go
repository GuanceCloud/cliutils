package aggregate

import (
	"github.com/GuanceCloud/cliutils/point"
	"testing"
	"time"
)

func TestPickTrace(t *testing.T) {
	now := time.Now()
	pt1 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "/resource",
		"trace_id":                    "1000000000",
		"span_id":                     "123456789",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt1.SetTime(now)

	packages := PickTrace("ddtrace", []*point.Point{pt1})

	t.Logf("package len=%d", len(packages))
}
