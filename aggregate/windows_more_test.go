package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheAddBatchsAndWindowsToData(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	pt := point.NewPoint("request", point.KVs{}.Add("latency", 10.0).AddTag("host", "node-1"), point.WithTime(now), point.WithPrecheck(false))
	batch := &AggregationBatch{
		RoutingKey: 1,
		Points:     &point.PBPoints{Arr: []*point.PBPoint{pt.PBPoint()}},
		AggregationOpts: map[string]*AggregationAlgo{
			"latency": {
				Method: string(SUM),
				Window: int64(time.Hour),
			},
		},
	}

	cache := NewCache(time.Second)
	n, expN := cache.AddBatchs("token-a", []*AggregationBatch{batch})
	require.Equal(t, 1, n)
	require.Zero(t, expN)
	require.Len(t, cache.WindowsBuckets, 1)

	var windows []*Window
	for _, bucket := range cache.WindowsBuckets {
		windows = append(windows, bucket.WS...)
	}
	data := WindowsToData(windows)
	require.Len(t, data, 1)
	assert.Equal(t, "token-a", data[0].Token)
	require.Len(t, data[0].PTS, 1)
	sum, ok := data[0].PTS[0].GetF("latency")
	require.True(t, ok)
	assert.Equal(t, 10.0, sum)
	assert.Equal(t, "node-1", data[0].PTS[0].GetTag("host"))

	for _, window := range windows {
		window.Reset()
		assert.Empty(t, window.Token)
		assert.Empty(t, window.cache)
	}
}

func TestCacheAddBatchDropsExpiredCalculators(t *testing.T) {
	pt := point.NewPoint("request", point.KVs{}.Add("latency", 10.0), point.WithTime(time.Now().Add(-2*time.Hour)), point.WithPrecheck(false))
	batch := &AggregationBatch{
		RoutingKey: 1,
		Points:     &point.PBPoints{Arr: []*point.PBPoint{pt.PBPoint()}},
		AggregationOpts: map[string]*AggregationAlgo{
			"latency": {
				Method: string(SUM),
				Window: int64(time.Second),
			},
		},
	}

	cache := NewCache(time.Second)
	n, expN := cache.AddBatch("token-a", batch)
	assert.Zero(t, n)
	assert.Equal(t, 1, expN)
	assert.Empty(t, cache.WindowsBuckets)
}

func TestWindowsToDataSkipsFailedCalculator(t *testing.T) {
	window := &Window{
		Token: "token-a",
		cache: map[uint64]Calculator{
			1: &algoStdev{
				MetricBase: MetricBase{key: "latency", name: "request"},
				data:       []float64{1},
			},
		},
	}

	data := WindowsToData([]*Window{window})
	require.Len(t, data, 1)
	assert.Equal(t, "token-a", data[0].Token)
	assert.Empty(t, data[0].PTS)
}
