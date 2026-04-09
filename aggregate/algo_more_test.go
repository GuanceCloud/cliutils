package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumericCalculators(t *testing.T) {
	now := time.Unix(1710000000, 0)
	base := MetricBase{
		key:      "latency",
		name:     "request",
		aggrTags: [][2]string{{"service", "checkout"}},
	}

	tests := []struct {
		name      string
		calc      Calculator
		inputs    []any
		wantField string
		wantFloat float64
		wantInt   int64
		reset     func()
	}{
		{
			name: "sum",
			calc: &algoSum{MetricBase: base, delta: 1, maxTime: now.UnixNano(), count: 1},
			inputs: []any{
				&algoSum{delta: 2, maxTime: now.Add(time.Second).UnixNano()},
				&algoSum{delta: 3, maxTime: now.Add(2 * time.Second).UnixNano()},
			},
			wantField: "latency",
			wantFloat: 6,
			wantInt:   3,
			reset: func() {
				calc := &algoSum{delta: 6, maxTime: 1, count: 3}
				calc.Reset()
				assert.Equal(t, &algoSum{}, calc)
			},
		},
		{
			name: "avg",
			calc: &algoAvg{MetricBase: base, delta: 2, maxTime: now.UnixNano(), count: 1},
			inputs: []any{
				&algoAvg{delta: 4, maxTime: now.Add(time.Second).UnixNano()},
				&algoAvg{delta: 6, maxTime: now.Add(2 * time.Second).UnixNano()},
			},
			wantField: "latency",
			wantFloat: 4,
			wantInt:   3,
			reset: func() {
				calc := &algoAvg{delta: 6, maxTime: 1, count: 3}
				calc.Reset()
				assert.Equal(t, &algoAvg{}, calc)
			},
		},
		{
			name: "count",
			calc: &algoCount{MetricBase: base, maxTime: now.UnixNano(), count: 1},
			inputs: []any{
				&algoCount{maxTime: now.Add(time.Second).UnixNano()},
				&algoCount{maxTime: now.Add(2 * time.Second).UnixNano()},
			},
			wantField: "latency",
			wantFloat: 0,
			wantInt:   3,
			reset: func() {
				calc := &algoCount{maxTime: 1, count: 3}
				calc.Reset()
				assert.Equal(t, &algoCount{}, calc)
			},
		},
		{
			name: "min",
			calc: &algoMin{MetricBase: base, min: 5, maxTime: now.UnixNano(), count: 1},
			inputs: []any{
				&algoMin{min: 3, maxTime: now.Add(time.Second).UnixNano()},
				&algoMin{min: 7, maxTime: now.Add(2 * time.Second).UnixNano()},
			},
			wantField: "latency",
			wantFloat: 3,
			wantInt:   3,
			reset: func() {
				calc := &algoMin{min: 6, maxTime: 1, count: 3}
				calc.Reset()
				assert.Equal(t, &algoMin{}, calc)
			},
		},
		{
			name: "max",
			calc: &algoMax{MetricBase: base, max: 5, maxTime: now.UnixNano(), count: 1},
			inputs: []any{
				&algoMax{max: 9, maxTime: now.Add(time.Second).UnixNano()},
				&algoMax{max: 7, maxTime: now.Add(2 * time.Second).UnixNano()},
			},
			wantField: "latency",
			wantFloat: 9,
			wantInt:   3,
			reset: func() {
				calc := &algoMax{max: 6, maxTime: 1, count: 3}
				calc.Reset()
				assert.Equal(t, &algoMax{}, calc)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, input := range tc.inputs {
				tc.calc.Add(input)
			}
			tc.calc.Add("ignored")
			pts, err := tc.calc.Aggr()
			require.NoError(t, err)
			require.Len(t, pts, 1)
			assert.Equal(t, "checkout", pts[0].GetTag("service"))
			assert.Equal(t, now.Add(2*time.Second), pts[0].Time())

			if tc.name == "count" {
				got, ok := pts[0].GetI(tc.wantField)
				require.True(t, ok)
				assert.Equal(t, tc.wantInt, got)
			} else {
				got, ok := pts[0].GetF(tc.wantField)
				require.True(t, ok)
				assert.Equal(t, tc.wantFloat, got)
			}
			count, ok := pts[0].GetI(tc.wantField + "_count")
			require.True(t, ok)
			assert.Equal(t, tc.wantInt, count)
			tc.reset()
		})
	}
}

func TestHistogramAndStdevCalculators(t *testing.T) {
	now := time.Unix(1710000000, 0)
	base := MetricBase{
		key:      "duration",
		name:     "request",
		aggrTags: [][2]string{{"env", "test"}},
	}

	hist := &algoHistogram{
		MetricBase: base,
		leBucket:   map[string]float64{},
	}
	hist.Add(&algoHistogram{
		MetricBase: MetricBase{
			pt: point.NewPoint("request", point.KVs{}.Add("duration", 2.0).AddTag("le", "10"), point.DefaultMetricOptions()...).PBPoint(),
		},
		val:     2,
		maxTime: now.UnixNano(),
	})
	hist.Add(&algoHistogram{
		MetricBase: MetricBase{
			pt: point.NewPoint("request", point.KVs{}.Add("duration", 3.0).AddTag("le", "10"), point.DefaultMetricOptions()...).PBPoint(),
		},
		val:     3,
		maxTime: now.Add(time.Second).UnixNano(),
	})
	hist.Add("ignored")
	hist.doHash(1)
	require.NotZero(t, hist.Base().hash)

	pts, err := hist.Aggr()
	require.NoError(t, err)
	require.Len(t, pts, 2)
	assert.Equal(t, "test", pts[0].GetTag("env"))
	assert.Equal(t, now.Add(time.Second), pts[0].Time())
	hist.Reset()
	assert.Empty(t, hist.leBucket)
	assert.Zero(t, hist.count)

	stdev := &algoStdev{MetricBase: base}
	stdev.Add(&algoStdev{data: []float64{2}, maxTime: now.UnixNano()})
	stdev.Add(&algoStdev{data: []float64{4}, maxTime: now.Add(time.Second).UnixNano()})
	stdev.Add(&algoStdev{data: []float64{6}, maxTime: now.Add(2 * time.Second).UnixNano()})
	stdev.Add("ignored")
	stdev.doHash(1)
	require.NotZero(t, stdev.Base().hash)

	pts, err = stdev.Aggr()
	require.NoError(t, err)
	require.Len(t, pts, 1)
	got, ok := pts[0].GetF("duration")
	require.True(t, ok)
	assert.InDelta(t, 2.0, got, 0.000001)
	count, ok := pts[0].GetI("duration_count")
	require.True(t, ok)
	assert.Equal(t, int64(3), count)
	assert.Equal(t, "test", pts[0].GetTag("env"))

	_, err = (&algoStdev{data: []float64{1}}).Aggr()
	assert.Error(t, err)
	_, err = SampleStdDev([]float64{1})
	assert.Error(t, err)
	got, err = SampleStdDev([]float64{1, 2, 3})
	require.NoError(t, err)
	assert.Equal(t, 1.0, got)
	stdev.Reset()
	assert.Nil(t, stdev.data)
	assert.Zero(t, stdev.maxTime)
}

func TestNewCalculatorsCoversSupportedMethods(t *testing.T) {
	now := time.Unix(1710000000, 0)
	pt := point.NewPoint("request", point.NewKVs(map[string]any{
		"latency": int64(42),
		"user":    "alice",
		"le":      "100",
	}), point.WithTime(now), point.WithPrecheck(false))

	batch := &AggregationBatch{
		RoutingKey: 1,
		Points:     &point.PBPoints{Arr: []*point.PBPoint{pt.PBPoint()}},
		AggregationOpts: map[string]*AggregationAlgo{
			"sum_latency":       {Method: string(SUM), SourceField: "latency", Window: int64(time.Second), AddTags: map[string]string{"role": "api"}},
			"avg_latency":       {Method: string(AVG), SourceField: "latency", Window: int64(time.Second)},
			"count_latency":     {Method: string(COUNT), SourceField: "user", Window: int64(time.Second)},
			"min_latency":       {Method: string(MIN), SourceField: "latency", Window: int64(time.Second)},
			"max_latency":       {Method: string(MAX), SourceField: "latency", Window: int64(time.Second)},
			"hist_latency":      {Method: string(HISTOGRAM), SourceField: "latency", Window: int64(time.Second)},
			"quantile_latency":  {Method: string(QUANTILES), SourceField: "latency", Window: int64(time.Second), Options: &AggregationAlgo_QuantileOpts{QuantileOpts: &QuantileOptions{Percentiles: []float64{0.5}}}},
			"stdev_latency":     {Method: string(STDEV), SourceField: "latency", Window: int64(time.Second)},
			"distinct_user":     {Method: string(COUNT_DISTINCT), SourceField: "user", Window: int64(time.Second)},
			"absent":            {Method: string(SUM), Window: int64(time.Second)},
			"":                  {Method: string(COUNT), SourceField: "latency", Window: int64(time.Second)},
			"missing_latency":   {Method: string(SUM), SourceField: "missing", Window: int64(time.Second)},
			"unsupported_value": {Method: string(MAX), SourceField: "user", Window: int64(time.Second)},
			"unknown_latency":   {Method: "unknown", SourceField: "latency", Window: int64(time.Second)},
			"quantile_default":  {Method: string(QUANTILES), SourceField: "latency", Window: int64(time.Second), Options: &AggregationAlgo_HistogramOpts{}},
			"expo_latency":      {Method: string(EXPO_HISTOGRAM), SourceField: "latency", Window: int64(time.Second)},
		},
	}

	calcs := newCalculators(batch)
	require.Len(t, calcs, 10)
	seen := map[string]bool{}
	for _, calc := range calcs {
		seen[calc.Base().key] = true
		assert.Equal(t, now.Unix(), calc.Base().nextWallTime)
		assert.Equal(t, int64(time.Second), calc.Base().window)
		if calc.Base().key == "sum_latency" {
			assert.Equal(t, [][2]string{{"role", "api"}}, calc.Base().aggrTags)
		}
	}
	assert.True(t, seen["sum_latency"])
	assert.True(t, seen["distinct_user"])
}

func TestQuantileResetAndBounds(t *testing.T) {
	q := &algoQuantiles{
		MetricBase: MetricBase{key: "latency", name: "request"},
		all:        []float64{10, 20, 30},
		count:      3,
		quantiles:  []float64{0, 1},
	}
	assert.Equal(t, 0.0, (&algoQuantiles{}).GetPercentile(50))
	assert.Equal(t, 10.0, q.GetPercentile(0))
	assert.Equal(t, 30.0, q.GetPercentile(100))
	q.Reset()
	assert.Empty(t, q.all)
	assert.Zero(t, q.count)
}

func TestAlgoMethodString(t *testing.T) {
	assert.Equal(t, "sum", SUM.String())
	assert.Equal(t, "invalid", AlgoMethod("invalid").String())
}
