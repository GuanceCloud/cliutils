package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlgoLast(t *testing.T) {
	now := time.Unix(1710000000, 0)
	calc := &algoCountLast{
		MetricBase: MetricBase{
			key:      "latency",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		},
		last:     10,
		lastTime: now.UnixNano(),
		count:    1,
	}

	calc.Add(&algoCountLast{
		last:     20,
		lastTime: now.Add(2 * time.Second).UnixNano(),
		count:    1,
	})
	calc.Add(&algoCountLast{
		last:     15,
		lastTime: now.Add(time.Second).UnixNano(),
		count:    1,
	})

	pts, err := calc.Aggr()
	require.NoError(t, err)
	require.Len(t, pts, 1)

	v, ok := pts[0].GetF("latency")
	require.True(t, ok)
	assert.Equal(t, 20.0, v)

	count, ok := pts[0].GetI("latency_count")
	require.True(t, ok)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, now.Add(2*time.Second), pts[0].Time())
	assert.Equal(t, "value1", pts[0].GetTag("tag1"))
}

func TestAlgoFirst(t *testing.T) {
	now := time.Unix(1710000000, 0)
	calc := &algoCountFirst{
		MetricBase: MetricBase{
			key:      "latency",
			name:     "test_metric",
			aggrTags: [][2]string{{"tag1", "value1"}},
		},
		first:     20,
		firstTime: now.Add(2 * time.Second).UnixNano(),
		count:     1,
	}

	calc.Add(&algoCountFirst{
		first:     10,
		firstTime: now.UnixNano(),
		count:     1,
	})
	calc.Add(&algoCountFirst{
		first:     15,
		firstTime: now.Add(time.Second).UnixNano(),
		count:     1,
	})

	pts, err := calc.Aggr()
	require.NoError(t, err)
	require.Len(t, pts, 1)

	v, ok := pts[0].GetF("latency")
	require.True(t, ok)
	assert.Equal(t, 10.0, v)

	count, ok := pts[0].GetI("latency_count")
	require.True(t, ok)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, now, pts[0].Time())
	assert.Equal(t, "value1", pts[0].GetTag("tag1"))
}

func TestNewCalculatorsLastFirst(t *testing.T) {
	now := time.Unix(1710000000, 0)
	pt := point.NewPoint("test_metric", point.NewKVs(map[string]any{
		"source_latency": 42.0,
		"host":           "node-1",
	}), point.WithTime(now), point.WithPrecheck(false))

	batch := &AggregationBatch{
		RoutingKey: 123,
		Points:     &point.PBPoints{Arr: []*point.PBPoint{pt.PBPoint()}},
		AggregationOpts: map[string]*AggregationAlgo{
			"last_latency": {
				Method:      string(LAST),
				SourceField: "source_latency",
				Window:      10,
			},
			"first_latency": {
				Method:      string(FIRST),
				SourceField: "source_latency",
				Window:      10,
			},
		},
	}

	calcs := newCalculators(batch)
	require.Len(t, calcs, 2)

	seen := map[string]bool{}
	for _, calc := range calcs {
		switch c := calc.(type) {
		case *algoCountLast:
			seen["last"] = true
			assert.Equal(t, 42.0, c.last)
			assert.Equal(t, now.UnixNano(), c.lastTime)
			assert.Equal(t, int64(1), c.count)
		case *algoCountFirst:
			seen["first"] = true
			assert.Equal(t, 42.0, c.first)
			assert.Equal(t, now.UnixNano(), c.firstTime)
			assert.Equal(t, int64(1), c.count)
		}
	}

	assert.True(t, seen["last"])
	assert.True(t, seen["first"])
}
