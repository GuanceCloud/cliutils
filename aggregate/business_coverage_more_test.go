package aggregate

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type brokenAggregationOptions struct{}

func (*brokenAggregationOptions) isAggregationAlgo_Options() {}
func (*brokenAggregationOptions) Equal(interface{}) bool     { return false }
func (*brokenAggregationOptions) MarshalTo([]byte) (int, error) {
	return 0, errors.New("broken options")
}
func (*brokenAggregationOptions) Size() int { return 1 }

func TestAggregatorConfigBusinessBranches(t *testing.T) {
	var cfg AggregatorConfigure
	assert.Error(t, cfg.UnmarshalTOML(map[string]any{"default_window": "bad"}))
	assert.Error(t, cfg.UnmarshalTOML(map[string]any{"aggregate_rules": "bad"}))
	assert.Error(t, cfg.UnmarshalTOML(map[string]any{"default_window": []string{"bad"}}))
	assert.Error(t, cfg.UnmarshalTOML(map[string]any{"default_window": func() {}}))
	require.NoError(t, cfg.UnmarshalTOML(map[string]any{}))
	require.NoError(t, cfg.UnmarshalTOML(map[string]any{
		"default_window":     float64(time.Second),
		"action":             ActionDrop,
		"delete_rules_point": true,
	}))
	assert.Equal(t, time.Second, cfg.DefaultWindow)
	assert.Equal(t, Action(ActionDrop), cfg.DefaultAction)
	assert.True(t, cfg.DeleteRulesPoint)

	assert.Error(t, (&AggregatorConfigure{DefaultAction: Action("bad")}).Setup())
	assert.Error(t, (&AggregatorConfigure{AggregateRules: []*AggregateRule{nil}}).Setup())
	assert.Error(t, (&AggregatorConfigure{AggregateRules: []*AggregateRule{{Name: "missing-selector"}}}).Setup())
	assert.Error(t, (&AggregatorConfigure{AggregateRules: []*AggregateRule{{Name: "bad-selector", Selector: &RuleSelector{Category: "bad"}}}}).Setup())
	assert.NoError(t, (&RuleSelector{Category: point.AgentLLM.String()}).Setup())

	tests := []struct {
		name       string
		algorithms map[string]*AggregationAlgoConfig
	}{
		{name: "empty-name", algorithms: map[string]*AggregationAlgoConfig{"": {Method: string(SUM)}}},
		{name: "nil-algo", algorithms: map[string]*AggregationAlgoConfig{"x": nil}},
		{name: "missing-method", algorithms: map[string]*AggregationAlgoConfig{"x": {}}},
		{name: "unknown-method", algorithms: map[string]*AggregationAlgoConfig{"x": {Method: "unknown"}}},
		{name: "expo", algorithms: map[string]*AggregationAlgoConfig{"x": {Method: string(EXPO_HISTOGRAM)}}},
		{name: "quantile-missing", algorithms: map[string]*AggregationAlgoConfig{"x": {Method: string(QUANTILES)}}},
		{name: "quantile-out-of-range", algorithms: map[string]*AggregationAlgoConfig{"x": {Method: string(QUANTILES), QuantileOpts: &QuantileOptions{Percentiles: []float64{1.1}}}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := (&AggregatorConfigure{
				DefaultWindow: time.Minute,
				AggregateRules: []*AggregateRule{
					{
						Name:       "rule",
						Selector:   &RuleSelector{Category: point.Metric.String()},
						Algorithms: tc.algorithms,
					},
				},
			}).Setup()
			assert.Error(t, err)
		})
	}

	valid := &AggregatorConfigure{
		DefaultWindow: time.Minute,
		AggregateRules: []*AggregateRule{
			{
				Name:     "rule",
				Selector: &RuleSelector{Category: point.Metric.String()},
				Groupby:  []string{"z", "a"},
				Algorithms: map[string]*AggregationAlgoConfig{
					"hist": {Method: string(HISTOGRAM), HistogramOpts: &HistogramOptions{Buckets: []float64{1}}},
					"expo": {Method: string(SUM), ExpoOpts: &ExpoHistogramOptions{MaxScale: 1}},
				},
			},
		},
	}
	require.NoError(t, valid.Setup())
	assert.Equal(t, []string{"a", "z"}, valid.AggregateRules[0].Groupby)
	assert.NotZero(t, valid.hash)
	assert.NotEmpty(t, valid.raw)
	assert.NotNil(t, valid.calcs)

	badHash := &AggregatorConfigure{
		AggregateRules: []*AggregateRule{
			{Algorithms: map[string]*AggregationAlgoConfig{"bad": {Method: string(HISTOGRAM), HistogramOpts: &HistogramOptions{Buckets: []float64{math.Inf(1)}}}}},
		},
	}
	badHash.doHash()
	assert.Zero(t, badHash.hash)
	assert.Nil(t, badHash.raw)
	assert.NotNil(t, badHash.hashView())
	assert.NotNil(t, (&AggregatorConfigure{AggregateRules: []*AggregateRule{nil}}).hashView())

	assert.Error(t, (&RuleSelector{Category: point.Metric.String(), Condition: `{ bad`}).Setup())
}

func TestPickPointsAndSelectorBranches(t *testing.T) {
	cfg := &AggregatorConfigure{
		DefaultWindow: time.Minute,
		AggregateRules: []*AggregateRule{
			{
				Name:     "rule",
				Selector: &RuleSelector{Category: point.Metric.String(), Measurements: []string{"request"}, MetricName: []string{"latency", "user", "ok"}},
				Groupby:  []string{"host", "region"},
				Algorithms: map[string]*AggregationAlgoConfig{
					"latency": {Method: string(SUM)},
				},
			},
			{
				Name:     "logging-rule",
				Selector: &RuleSelector{Category: point.Logging.String(), MetricName: []string{"message"}},
				Algorithms: map[string]*AggregationAlgoConfig{
					"message": {Method: string(COUNT)},
				},
			},
		},
	}
	require.NoError(t, cfg.Setup())

	pt := point.NewPoint("request", point.KVs{}.
		Add("latency", int64(10)).
		Add("user", "alice").
		Add("ok", true).
		Add("region", "cn").
		AddTag("host", "node-1").
		AddTag("ignored_tag", "x"),
		point.DefaultMetricOptions()...,
	)
	batches := cfg.PickPoints(point.Metric.String(), []*point.Point{pt})
	require.NotEmpty(t, batches)
	for _, batch := range batches {
		require.NotEmpty(t, batch.Batchs)
		assert.Equal(t, batch.PickKey, batch.Batchs[0].PickKey)
	}
	assert.Empty(t, cfg.PickPoints(point.RUM.String(), []*point.Point{pt}))

	selector := &RuleSelector{Category: point.Metric.String(), MetricName: []string{"latency", "user", "ok", "ratio"}}
	require.NoError(t, selector.Setup())
	pt = point.NewPoint("request", point.KVs{}.
		Add("latency", int64(10)).
		Add("user", "alice").
		Add("ok", true).
		Add("ratio", 1.5).
		AddTag("host", "node-1"),
		point.DefaultMetricOptions()...,
	)
	forked := selector.selectKVS(false, pt)
	require.Len(t, forked, 3)
	singleSelector := &RuleSelector{Category: point.Metric.String(), MetricName: []string{"latency"}}
	require.NoError(t, singleSelector.Setup())
	singlePoint := point.NewPoint("request", point.KVs{}.Add("latency", 1.0), point.DefaultMetricOptions()...)
	require.Len(t, singleSelector.selectKVS(true, singlePoint), 1)
	assert.Nil(t, singlePoint.Get("latency"))
	assert.Empty(t, selector.selectKVS(false, point.NewPoint("request", point.KVs{}.Add("missing", 1), point.DefaultMetricOptions()...)))
	tagSelector := &RuleSelector{Category: point.Metric.String(), MetricName: []string{"host"}}
	require.NoError(t, tagSelector.Setup())
	assert.Empty(t, tagSelector.selectKVS(false, point.NewPoint("request", point.KVs{}.AddTag("host", "node-1"), point.DefaultMetricOptions()...)))

	unsignedSelector := &RuleSelector{Category: point.Metric.String(), MetricName: []string{"u", "d"}}
	require.NoError(t, unsignedSelector.Setup())
	assert.NotEmpty(t, unsignedSelector.selectKVS(false, point.NewPoint("request", point.KVs{}.Add("u", uint64(1)).Add("d", []byte("x")), point.WithPrecheck(false))))
}

func TestBatchRequestBranches(t *testing.T) {
	req, err := batchRequest(&AggregationBatch{RoutingKey: 42}, "http://127.0.0.1:1/aggregate")
	require.NoError(t, err)
	assert.Equal(t, "42", req.Header.Get(GuanceRoutingKey))

	_, err = batchRequest(&AggregationBatch{RoutingKey: 42}, "://bad-url")
	assert.Error(t, err)

	_, err = batchRequest(&AggregationBatch{
		AggregationOpts: map[string]*AggregationAlgo{
			"bad": {Options: &brokenAggregationOptions{}},
		},
	}, "http://127.0.0.1:1/aggregate")
	assert.Error(t, err)
}

func TestSmallBusinessHelpersAndStrings(t *testing.T) {
	base := MetricBase{
		key:          "latency",
		name:         "request",
		aggrTags:     [][2]string{{"service", "checkout"}},
		hash:         1,
		window:       int64(time.Second),
		nextWallTime: time.Unix(1710000000, 0).Unix(),
	}
	assert.Contains(t, base.String(), "latency")
	assert.Equal(t, "base=<nil>", formatMetricBaseForCalc(nil))
	assert.Equal(t, "[1, 2.5]", formatFloat64Slice([]float64{1, 2.5}))
	assert.Equal(t, "[]", formatFloat64Slice(nil))
	assert.Equal(t, "{}", formatHistogramBuckets(nil))
	assert.Equal(t, "[]", formatDistinctValues(nil))
	assert.Contains(t, prettyBatch(&AggregationBatch{RoutingKey: 1, ConfigHash: 2, Points: &point.PBPoints{Arr: []*point.PBPoint{{Name: "p"}}}}), "routingKey")

	calcs := []Calculator{
		&algoSum{MetricBase: base, delta: 1, count: 1, maxTime: 1},
		&algoAvg{MetricBase: base, delta: 1, count: 1, maxTime: 1},
		&algoCount{MetricBase: base, count: 1, maxTime: 1},
		&algoMax{MetricBase: base, max: 1, count: 1, maxTime: 1},
		&algoMin{MetricBase: base, min: 1, count: 1, maxTime: 1},
		&algoHistogram{MetricBase: base, count: 1, val: 1, maxTime: 1, leBucket: map[string]float64{"1": 1}},
		&algoQuantiles{MetricBase: base, count: 1, maxTime: 1, quantiles: []float64{0.5}, all: []float64{1}},
		&algoStdev{MetricBase: base, maxTime: 1, data: []float64{1, 2}},
		newAlgoCountDistinct(base, 1, "alice"),
		&algoCountFirst{MetricBase: base, first: 1, firstTime: 1, count: 1},
		&algoCountLast{MetricBase: base, last: 1, lastTime: 1, count: 1},
	}
	for _, calc := range calcs {
		assert.NotEmpty(t, calc.ToString())
		assert.NotNil(t, calc.Base())
	}

	first := &algoCountFirst{first: 1, firstTime: 1, count: 1}
	first.Reset()
	assert.Zero(t, first.count)
	assert.NotNil(t, first.Base())
	last := &algoCountLast{last: 1, lastTime: 1, count: 1}
	last.Reset()
	assert.Zero(t, last.count)
	assert.NotNil(t, last.Base())

	for _, method := range []string{"", "method_unspecified", " SUM ", "avg", "count", "min", "max", "histogram", "merge_histogram", "expo_histogram", "stdev", "quantiles", "count_distinct", "last", "first", "Custom"} {
		_ = NormalizeAlgoMethod(method).String()
	}
	q := &algoQuantiles{MetricBase: MetricBase{key: "latency", name: "request", aggrTags: [][2]string{{"service", "checkout"}}}, all: []float64{1, 2}, count: 2, quantiles: []float64{0.5}}
	qpts, err := q.Aggr()
	require.NoError(t, err)
	assert.Equal(t, "checkout", qpts[0].GetTag("service"))

	pt := point.NewPoint("request", point.KVs{}.Add("host", "node-1").Add("value", int64(1)), point.WithPrecheck(false))
	assert.Equal(t, [][2]string{{"host", "node-1"}}, pointAggrTags(pt, []string{"host", "missing"}))
	start, duration := getTime(point.NewPoint("trace", point.KVs{}.Add("start_time", 1.5).Add("duration", 2.5), point.CommonLoggingOptions()...))
	assert.Equal(t, int64(1000), start)
	assert.Equal(t, int64(2000), duration)
}

func TestPtWrapAndTSDataHelpers(t *testing.T) {
	assert.Zero(t, packetPointCount(nil))
	require.NoError(t, (*DataPacket)(nil).WalkRawPBPoints(func([]byte) bool { return false }))
	require.NoError(t, (&DataPacket{}).WalkRawPBPoints(nil))

	rawPoint := point.NewPoint("request", point.KVs{}.
		Add("f", 1.5).
		Add("i", int64(2)).
		Add("u", uint64(3)).
		Add("s", "x").
		Add("d", []byte("y")).
		Add("b", true),
		point.WithPrecheck(false),
	)
	wrap := &ptWrap{Point: rawPoint}
	for _, key := range []string{"f", "i", "u", "s"} {
		_, ok := wrap.Get(key)
		assert.True(t, ok)
	}
	_, ok := wrap.Get("missing")
	assert.False(t, ok)

	pb := &point.PBPoint{
		Name: "request",
		Fields: []*point.Field{
			{Key: "f", Val: &point.Field_F{F: 1.5}},
			{Key: "i", Val: &point.Field_I{I: 2}},
			{Key: "u", Val: &point.Field_U{U: 3}},
			{Key: "s", Val: &point.Field_S{S: "x"}},
			{Key: "d", Val: &point.Field_D{D: []byte("y")}},
			{Key: "b", Val: &point.Field_B{B: true}},
			{Key: "nil"},
		},
	}
	payload := point.AppendPBPointToPBPointsPayload(nil, pb)
	var raw []byte
	err := (&DataPacket{PointsPayload: payload}).WalkRawPBPoints(func(x []byte) bool {
		raw = append([]byte(nil), x...)
		return false
	})
	require.NoError(t, err)
	wrap = &ptWrap{}
	require.NoError(t, wrap.Reset(nil))
	require.NoError(t, wrap.Reset(raw))
	for _, key := range []string{"f", "i", "u", "s", "d", "b", "nil"} {
		_, ok := wrap.Get(key)
		assert.True(t, ok)
	}
	assert.Error(t, wrap.Reset([]byte("not-pb")))
	wrap = &ptWrap{pb: point.PBPoint{Fields: []*point.Field{{Key: "unknown"}}}}
	_, ok = wrap.Get("unknown")
	assert.True(t, ok)
	_, ok = wrap.Get("absent")
	assert.False(t, ok)
	pbPoint := &point.PBPoint{Fields: []*point.Field{
		{Key: "d", Val: &point.Field_D{D: []byte("z")}},
		{Key: "b", Val: &point.Field_B{B: true}},
		{Key: "nil"},
	}}
	wrap = &ptWrap{Point: point.WrapPB(nil, pbPoint)}
	for _, key := range []string{"d", "b", "nil"} {
		_, ok = wrap.Get(key)
		assert.True(t, ok)
	}
}

func TestTailSamplingBusinessBranches(t *testing.T) {
	assert.NoError(t, (&SamplingPipeline{}).Apply())
	assert.NoError(t, (*SamplingPipeline)(nil).Apply())
	assert.Error(t, (&SamplingPipeline{Condition: `{ bad`}).Apply())
	matched, packet := (&SamplingPipeline{}).DoAction(&DataPacket{})
	assert.False(t, matched)
	assert.NotNil(t, packet)
	matched, packet = (&SamplingPipeline{Type: PipelineTypeSampling, Rate: 1}).DoAction(nil)
	assert.False(t, matched)
	assert.Nil(t, packet)
	assert.Nil(t, pipelineMatchedPacket(nil, &SamplingPipeline{}))
	assert.Nil(t, pipelineMatchedPacket(&DataPacket{}, nil))
	assert.Same(t, (*DataPacket)(nil), pipelineMatchedPacket(nil, nil))
	payload := point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "event", Fields: []*point.Field{{Key: "status", Val: &point.Field_S{S: "drop"}}}})
	td := &DataPacket{GroupIdHash: 99, PointsPayload: payload}
	keepPipeline := &SamplingPipeline{Type: PipelineTypeCondition, Condition: `{ status = "keep" }`, Action: PipelineActionKeep}
	require.NoError(t, keepPipeline.Apply())
	matched, packet = keepPipeline.DoAction(td)
	assert.False(t, matched)
	assert.NotNil(t, packet)
	matched, packet = evaluatePipelines(nil, []*SamplingPipeline{{Type: PipelineTypeSampling, Rate: 1}})
	assert.False(t, matched)
	assert.Nil(t, packet)
	matched, packet = evaluatePipelines(td, nil)
	assert.False(t, matched)
	assert.Nil(t, packet)

	matched, packet = evaluatePipelines(td, []*SamplingPipeline{
		nil,
		{Type: PipelineTypeSampling, Rate: 0},
		{Type: PipelineTypeCondition, Action: PipelineActionKeep},
	})
	assert.False(t, matched)
	assert.Nil(t, packet)
	zeroRate := &SamplingPipeline{Type: PipelineTypeSampling, Rate: 0, Condition: `{ status = "drop" }`}
	require.NoError(t, zeroRate.Apply())
	matched, packet = evaluatePipelines(td, []*SamplingPipeline{zeroRate})
	assert.False(t, matched)
	assert.Nil(t, packet)
	matched, packet = evaluatePipelines(&DataPacket{PointsPayload: []byte("bad")}, []*SamplingPipeline{{Type: PipelineTypeSampling, Rate: 1}})
	assert.False(t, matched)
	assert.Nil(t, packet)
	assert.Same(t, td, pipelineMatchedPacket(td, &SamplingPipeline{Type: "unknown"}))
	assert.Nil(t, pipelineMatchedPacket(td, &SamplingPipeline{Type: PipelineTypeSampling, Rate: 0.0001}))
	assert.Same(t, td, pipelineMatchedPacket(td, &SamplingPipeline{Type: PipelineTypeSampling, Rate: 0}))
	assert.Same(t, td, pipelineMatchedPacket(td, &SamplingPipeline{Type: PipelineTypeCondition, Action: "unknown"}))
	unknownPacket := &DataPacket{GroupIdHash: 0, PointsPayload: payload}
	assert.Same(t, unknownPacket, pipelineMatchedPacket(unknownPacket, &SamplingPipeline{Type: PipelineTypeSampling, Rate: 0.0001}))

	ptMissingTraceID := point.NewPoint("trace", point.KVs{}.Add("span_id", "1"), point.CommonLoggingOptions()...)
	assert.Empty(t, PickTrace("source", []*point.Point{ptMissingTraceID}, 1))
	ptFloatTime := point.NewPoint("trace", point.KVs{}.Add("trace_id", "t1").Add("span_id", "1").Add("start_time", 1.5).Add("duration", 2.5), point.CommonLoggingOptions()...)
	assert.NotEmpty(t, PickTrace("source", []*point.Point{ptFloatTime}, 1))
	ptError := point.NewPoint("trace", point.KVs{}.Add("trace_id", "t1").Add("span_id", "2").AddTag("status", "error").Add("start_time", int64(1)).Add("duration", int64(3)), point.CommonLoggingOptions()...)
	ptEarlier := point.NewPoint("trace", point.KVs{}.Add("trace_id", "t1").Add("span_id", "3").Add("start_time", int64(0)).Add("duration", int64(1)), point.CommonLoggingOptions()...)
	ptLater := point.NewPoint("trace", point.KVs{}.Add("trace_id", "t1").Add("span_id", "4").Add("start_time", int64(2)).Add("duration", int64(5)), point.CommonLoggingOptions()...)
	grouped := PickTrace("source", []*point.Point{ptError, ptEarlier, ptLater}, 1)
	require.Len(t, grouped, 1)
	for _, packet := range grouped {
		assert.True(t, packet.HasError)
		assert.Equal(t, int64(2000), packet.TraceStartTimeUnixNano)
		assert.Equal(t, int64(7000), packet.TraceEndTimeUnixNano)
	}

	cfg := &TailSamplingConfigs{
		Tracing: &TraceTailSampling{
			GroupKey:       "bad",
			DerivedMetrics: []*DerivedMetric{{Name: "custom"}},
			Pipelines: []*SamplingPipeline{
				nil,
				{Name: "bad-action", Type: PipelineTypeCondition, Action: "bad"},
				{Name: "bad-rate", Type: PipelineTypeSampling, Rate: 2},
				{Name: "bad-type", Type: "bad"},
				{Name: "bad-condition", Type: PipelineTypeCondition, Action: PipelineActionKeep, Condition: `{ bad`},
			},
		},
		Logging: &LoggingTailSampling{GroupDimensions: []*LoggingGroupDimension{nil, {Pipelines: []*SamplingPipeline{nil}, DerivedMetrics: []*DerivedMetric{{Name: "custom"}}}}},
		RUM:     &RUMTailSampling{GroupDimensions: []*RUMGroupDimension{nil, {Pipelines: []*SamplingPipeline{nil}, DerivedMetrics: []*DerivedMetric{{Name: "custom"}}}}},
	}
	err := cfg.Init()
	require.Error(t, err)
	for _, part := range []string{"invalid trace group key", "pipeline is nil", "invalid action", "invalid sampling rate", "invalid type", "invalid condition", "derived_metrics", "missing group_key"} {
		assert.Contains(t, err.Error(), part)
	}

	cfgs := initBuiltinMetricCfgs([]*BuiltinMetricCfg{{Name: "", Enabled: true}, nil, {Name: "x", Enabled: false}}, nil)
	require.Len(t, cfgs, 3)
	assert.Equal(t, "x", cfgs[2].Name)
	assert.False(t, appendPointPayload(nil, nil))
}

func TestTailSamplingProcessorFilteringBranches(t *testing.T) {
	sampler := NewGlobalSampler(1, time.Second)
	require.NoError(t, sampler.UpdateConfig("token-a", &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{GroupKey: "trace_id", BuiltinMetrics: []*BuiltinMetricCfg{
			nil,
			{Name: "trace_total_count", Enabled: false},
		}},
		Logging: &LoggingTailSampling{GroupDimensions: []*LoggingGroupDimension{{GroupKey: "fields.user"}}, BuiltinMetrics: []*BuiltinMetricCfg{
			{Name: "logging_total_count", Enabled: false},
		}},
		RUM: &RUMTailSampling{GroupDimensions: []*RUMGroupDimension{{GroupKey: "fields.session"}}, BuiltinMetrics: []*BuiltinMetricCfg{
			{Name: "rum_total_count", Enabled: false},
		}},
	}))
	processor := NewTailSamplingProcessor(sampler, NewDerivedMetricCollector(time.Second), DefaultTailSamplingBuiltinMetrics())
	assert.NotNil(t, processor)
	assert.False(t, processor.isBuiltinMetricEnabled("token-a", point.STracing, "trace_total_count"))
	assert.False(t, processor.isBuiltinMetricEnabled("token-a", point.SLogging, "logging_total_count"))
	assert.False(t, processor.isBuiltinMetricEnabled("token-a", point.SRUM, "rum_total_count"))
	assert.True(t, processor.isBuiltinMetricEnabled("token-a", "unknown", "x"))
	assert.True(t, processor.isBuiltinMetricEnabled("missing", point.STracing, "trace_total_count"))
	assert.True(t, processor.isBuiltinMetricEnabled("token-a", point.SLogging, "not-configured"))
	assert.True(t, processor.isBuiltinMetricEnabled("token-a", point.SRUM, "not-configured"))
	assert.True(t, processor.isBuiltinMetricEnabled("token-a", point.STracing, "not-configured"))
	assert.True(t, processor.isBuiltinMetricEnabled("missing", point.SLogging, "logging_total_count"))
	assert.True(t, processor.isBuiltinMetricEnabled("missing", point.SRUM, "rum_total_count"))
	assert.True(t, (*TailSamplingProcessor)(nil).isBuiltinMetricEnabled("token-a", point.STracing, "trace_total_count"))
	manualSampler := NewGlobalSampler(1, time.Second)
	manualSampler.configMap["token-a"] = &TailSamplingConfigs{Tracing: &TraceTailSampling{}, Logging: &LoggingTailSampling{}, RUM: &RUMTailSampling{}}
	manualProcessor := NewTailSamplingProcessor(manualSampler, nil, nil)
	assert.True(t, manualProcessor.isBuiltinMetricEnabled("token-a", point.STracing, "missing"))
	assert.True(t, manualProcessor.isBuiltinMetricEnabled("token-a", point.SLogging, "missing"))
	assert.True(t, manualProcessor.isBuiltinMetricEnabled("token-a", point.SRUM, "missing"))

	records := []DerivedMetricRecord{{MetricName: "trace_total_count"}, {MetricName: "trace_kept_count"}}
	filtered := processor.filterBuiltinRecords(&DataPacket{Token: "token-a", DataType: point.STracing}, records)
	require.Len(t, filtered, 1)
	assert.Equal(t, "trace_kept_count", filtered[0].MetricName)
	assert.Equal(t, records, processor.filterBuiltinRecords(nil, records))
	assert.Nil(t, processor.filterBuiltinRecords(&DataPacket{}, nil))

	(&TailSamplingProcessor{collector: processor.collector, metrics: processor.metrics}).IngestPacket(&DataPacket{Token: "token-a", DataType: point.STracing})
	(*TailSamplingProcessor)(nil).IngestPacket(&DataPacket{})
	processor.IngestPacket(nil)
	(&TailSamplingProcessor{sampler: sampler}).RecordDecision(&DataPacket{Token: "token-a", DataType: point.STracing}, DerivedMetricDecisionDropped)
	kept := (&TailSamplingProcessor{sampler: sampler}).TailSamplingData(map[uint64]*DataGroup{
		1: {dataType: point.STracing, packet: &DataPacket{
			Token:         "token-a",
			DataType:      point.STracing,
			GroupKey:      "trace_id",
			PointsPayload: point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "span"}),
		}},
	})
	assert.NotNil(t, kept)
	_ = processor.TailSamplingData(map[uint64]*DataGroup{1: nil, 2: {}})
}

func TestCollectorAndWindowRemainingBranches(t *testing.T) {
	assert.Equal(t, DefaultDerivedMetricFlushWindow, NewDerivedMetricCollector(0).window)
	collector := NewDerivedMetricCollector(time.Second)
	collector.Add(nil)
	assert.Contains(t, collector.String(), "derived-metric-collector")
	collector.Add([]DerivedMetricRecord{
		{
			Token:       "token-a",
			MetricName:  "x",
			Kind:        DerivedMetricKindSum,
			Stage:       DerivedMetricStageIngest,
			Measurement: "m",
			Tags:        map[string]string{"k": "v"},
			Value:       1,
		},
	})
	assert.Empty(t, collector.Flush(time.Unix(0, 0)))
	assert.NotEmpty(t, collector.Flush(time.Now().Add(time.Minute)))
	collector.Add([]DerivedMetricRecord{
		{
			Token:       "token-a",
			MetricName:  "hist",
			Kind:        DerivedMetricKindHistogram,
			Stage:       DerivedMetricStageDecision,
			Decision:    DerivedMetricDecisionKept,
			Measurement: "m",
			Tags:        map[string]string{"k": "v"},
			Value:       2,
			Buckets:     []float64{1, 5},
			Time:        time.Now(),
		},
	})
	assert.NotEmpty(t, collector.Flush(time.Now().Add(time.Minute)))

	cache := NewCache(time.Hour)
	assert.Empty(t, cache.GetExpWidows())
	cache.WindowsBuckets[time.Now().Add(time.Hour).Unix()] = &Windows{IDs: map[string]int{}, WS: []*Window{}}
	assert.Empty(t, cache.GetExpWidows())
	expiredWindow := &Window{Token: "token-a", cache: map[uint64]Calculator{}}
	cache.WindowsBuckets[time.Now().Add(-time.Second).Unix()] = &Windows{IDs: map[string]int{"token-a": 0}, WS: []*Window{expiredWindow}}
	assert.Equal(t, []*Window{expiredWindow}, cache.GetExpWidows())
}

func TestBuiltinMetricAndTimeWheelRemainingBranches(t *testing.T) {
	for _, metric := range DefaultTailSamplingBuiltinMetrics() {
		assert.NotEmpty(t, metric.Name())
	}
	hist := &histogramBuiltinDerivedMetric{
		name:    "custom_hist",
		buckets: []float64{1},
		onObserve: func(packet *DataPacket) (float64, bool) {
			return 1, true
		},
		recordTags: func(packet *DataPacket) map[string]string {
			return map[string]string{"custom": "tag"}
		},
	}
	records := hist.OnPreDecision(&DataPacket{Token: "token-a", DataType: point.STracing})
	require.Len(t, records, 1)
	assert.Equal(t, "tag", records[0].Tags["custom"])
	assert.Nil(t, DefaultTailSamplingBuiltinMetrics().OnPreDecision(&DataPacket{DataType: point.STracing, TraceStartTimeUnixNano: 10, TraceEndTimeUnixNano: 5}))
	assert.Equal(t, time.Unix(0, 20), packetTime(&DataPacket{TraceEndTimeUnixNano: 20}))

	sampler := &GlobalSampler{
		shardCount: 1,
		shards:     []*Shard{{}},
		configMap: map[string]*TailSamplingConfigs{
			"token-a": {Tracing: &TraceTailSampling{DataTTL: time.Second}},
			"token-b": {},
		},
	}
	sampler.Ingest(&DataPacket{GroupIdHash: 0, Token: "token-a", DataType: point.STracing, RawGroupId: ""})
	sampler.Ingest(&DataPacket{GroupIdHash: 0, Token: "token-a", DataType: point.STracing, RawGroupId: "trace-1"})
	assert.Equal(t, "trace-1", sampler.shards[0].activeMap[tailSamplingGroupMapKeyByFields("token-a", 0, point.STracing, "")].packet.RawGroupId)
	sampler.Ingest(&DataPacket{GroupIdHash: 1, Token: "token-b", DataType: point.SLogging})
	sampler.Ingest(&DataPacket{GroupIdHash: 1, Token: "token-b", DataType: point.SRUM})
	assert.Nil(t, sampler.GetLoggingConfig("missing"))
	outcomes := sampler.TailSamplingOutcomes(map[uint64]*DataGroup{
		1: {dataType: point.SRUM, packet: &DataPacket{
			Token:         "token-c",
			DataType:      point.SRUM,
			GroupKey:      "session",
			PointsPayload: point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "rum"}),
		}},
	})
	assert.Equal(t, DerivedMetricDecisionDropped, outcomes[1].Decision)

	require.NoError(t, sampler.UpdateConfig("token-a", &TailSamplingConfigs{Version: 2, Tracing: &TraceTailSampling{GroupKey: "trace_id"}}))
	assert.Equal(t, int64(2), sampler.configMap["token-a"].Version)
	assert.Error(t, sampler.UpdateConfig("token-a", &TailSamplingConfigs{Version: 3, Tracing: &TraceTailSampling{GroupKey: "bad"}}))
}

func TestStringHelpersEmptyInputs(t *testing.T) {
	assert.Equal(t, "[]", pipelineNames(nil))
	assert.Equal(t, "[]", derivedMetricNames(nil))
	assert.Equal(t, "[]", builtinMetricNames(nil))
	assert.Equal(t, "[]", loggingGroupStrings(nil))
	assert.Equal(t, "[]", rumGroupStrings(nil))
	assert.True(t, strings.Contains((&MetricBase{}).String(), "hash"))
}
