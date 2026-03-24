package aggregate

import (
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_alignNextWallTime(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		now := time.Unix(123, 0)
		wallTime := AlignNextWallTime(now, time.Second*10)
		assert.Equal(t, int64(130), wallTime)

		wallTime = AlignNextWallTime(now, time.Second)
		assert.Equal(t, int64(123), wallTime)
	})
}

func TestQuantile(t *T.T) {
	q := &algoQuantiles{
		MetricBase: MetricBase{
			key:  "latency",
			name: "tail_sampling",
		},
		count:     2,
		all:       []float64{10, 30},
		quantiles: []float64{0.5, 0.95},
		maxTime:   time.Unix(1700000002, 0).UnixNano(),
	}

	q.Add(&algoQuantiles{
		count:   2,
		all:     []float64{20, 40},
		maxTime: time.Unix(1700000003, 0).UnixNano(),
	})

	assert.Equal(t, int64(4), q.count)
	assert.Equal(t, []float64{10, 30, 20, 40}, q.all)
	assert.Equal(t, 25.0, q.GetPercentile(50))
	assert.Equal(t, 38.5, q.GetPercentile(95))

	pts, err := q.Aggr()
	require.NoError(t, err)
	require.Len(t, pts, 1)

	p50, ok := pts[0].GetF("latency_P50")
	require.True(t, ok)
	assert.Equal(t, 25.0, p50)

	p95, ok := pts[0].GetF("latency_P95")
	require.True(t, ok)
	assert.Equal(t, 38.5, p95)

	count := pts[0].Get("latency_count")
	require.NotNil(t, count)
	assert.Equal(t, int64(4), count)

	assert.Equal(t, q.maxTime, pts[0].Time().UnixNano())
}

func TestNewCalculatorsQuantiles(t *T.T) {
	pt := point.NewPoint(
		"demo",
		point.KVs{}.Add("latency", 42.0),
		point.WithTime(time.Unix(1700000001, 0)),
	)

	batch := &AggregationBatch{
		AggregationOpts: map[string]*AggregationAlgo{
			"latency": {
				Method:      string(QUANTILES),
				SourceField: "latency",
				Options: &AggregationAlgo_QuantileOpts{
					QuantileOpts: &QuantileOptions{
						Percentiles: []float64{0.5, 0.9},
					},
				},
			},
		},
		Points: &point.PBPoints{Arr: []*point.PBPoint{pt.PBPoint()}},
	}

	calcs := newCalculators(batch)
	require.Len(t, calcs, 1)

	q, ok := calcs[0].(*algoQuantiles)
	require.True(t, ok)
	assert.Equal(t, int64(1), q.count)
	assert.Equal(t, []float64{42}, q.all)
	assert.Equal(t, []float64{0.5, 0.9}, q.quantiles)
}
