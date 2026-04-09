package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCalculatorToString(t *testing.T) {
	t.Run("histogram-stable-order", func(t *testing.T) {
		calc := &algoHistogram{
			MetricBase: MetricBase{
				key:          "latency_bucket",
				name:         "demo",
				hash:         42,
				window:       int64(10 * time.Second),
				nextWallTime: 1700000010,
				heapIdx:      3,
				aggrTags:     [][2]string{{"service", "api"}},
			},
			count:    2,
			val:      5,
			maxTime:  time.Unix(1700000005, 0).UnixNano(),
			leBucket: map[string]float64{"10": 2, "1": 1},
		}

		assert.Equal(
			t,
			"algoHistogram{count=2 val=5 max_time=1700000005000000000 buckets={1=1, 10=2} base={name=demo key=latency_bucket hash=42 window=10s next_wall_time=2023-11-14T22:13:30Z heap_idx=3 tags=[service=api]}}",
			calc.ToString(),
		)
	})

	t.Run("count-distinct-stable-order", func(t *testing.T) {
		calc := newAlgoCountDistinct(
			MetricBase{
				key:      "user",
				name:     "demo",
				hash:     7,
				window:   int64(5 * time.Second),
				aggrTags: [][2]string{{"env", "prod"}},
			},
			time.Unix(1700000001, 0).UnixNano(),
			"alice",
		)
		calc.distinctValues[42] = struct{}{}
		calc.distinctValues[true] = struct{}{}

		assert.Equal(
			t,
			"algoCountDistinct{count=3 max_time=1700000001000000000 distinct_values=[bool:true, int:42, string:alice] base={name=demo key=user hash=7 window=5s next_wall_time=<zero> heap_idx=0 tags=[env=prod]}}",
			calc.ToString(),
		)
	})
}
