package aggregate

import (
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailSamplingProcessorRecordsBuiltinMetrics(t *testing.T) {
	now := time.Unix(1710000000, 0)
	processor := NewDefaultTailSamplingProcessor(2, time.Second)
	require.NotNil(t, processor.Sampler())
	require.NotNil(t, processor.Collector())
	require.NotEmpty(t, processor.BuiltinMetrics())

	err := processor.UpdateConfig("token-a", &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL:  time.Second,
			GroupKey: "trace_id",
			BuiltinMetrics: []*BuiltinMetricCfg{
				{Name: "trace_total_count", Enabled: true},
				{Name: "trace_error_count", Enabled: false},
			},
			Pipelines: []*SamplingPipeline{
				{Name: "keep_all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
	})
	require.NoError(t, err)

	packet := &DataPacket{
		GroupIdHash:            1,
		RawGroupId:             "trace-1",
		Token:                  "token-a",
		DataType:               point.STracing,
		Source:                 "ddtrace",
		ConfigVersion:          1,
		HasError:               true,
		GroupKey:               "trace_id",
		PointCount:             2,
		TraceStartTimeUnixNano: now.UnixNano(),
		TraceEndTimeUnixNano:   now.Add(50 * time.Millisecond).UnixNano(),
		PointsPayload:          point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "span"}),
		MaxPointTimeUnixNano:   now.UnixNano(),
	}

	processor.IngestPacket(packet)
	expired := processor.AdvanceTime()
	require.Len(t, expired, 1)

	kept := processor.TailSamplingData(expired)
	require.Len(t, kept, 1)
	processor.RecordDecision(packet, DerivedMetricDecisionKept)
	pointsByToken := processor.FlushDerivedMetrics(now.Add(time.Minute))
	require.Len(t, pointsByToken, 1)
	assert.Equal(t, "token-a", pointsByToken[0].Token)

	seenFields := map[string]bool{}
	for _, pt := range pointsByToken[0].PTS {
		for _, kv := range pt.KVs() {
			if !kv.IsTag {
				seenFields[kv.Key] = true
			}
		}
	}
	assert.True(t, seenFields["trace_total_count"])
	assert.True(t, seenFields["trace_kept_count"])
	assert.True(t, seenFields["span_total_count"])
	assert.True(t, seenFields["trace_duration_bucket"])
	assert.False(t, seenFields["trace_error_count"], "disabled builtin metric should be filtered")
	assert.Nil(t, (*TailSamplingProcessor)(nil).Sampler())
	assert.Nil(t, (*TailSamplingProcessor)(nil).Collector())
	assert.Nil(t, (*TailSamplingProcessor)(nil).BuiltinMetrics())
	assert.NoError(t, (*TailSamplingProcessor)(nil).UpdateConfig("token-a", nil))
	assert.Nil(t, (*TailSamplingProcessor)(nil).AdvanceTime())
	assert.Nil(t, (*TailSamplingProcessor)(nil).TailSamplingData(nil))
	assert.Nil(t, (*TailSamplingProcessor)(nil).FlushDerivedMetrics(now))
}

func TestDefaultTailSamplingBuiltinMetricsRecordsByDataType(t *testing.T) {
	now := time.Unix(1710000000, 0)
	metrics := DefaultTailSamplingBuiltinMetrics()
	require.NotEmpty(t, metrics)

	trace := &DataPacket{
		Token:                  "token-a",
		DataType:               point.STracing,
		Source:                 "ddtrace",
		GroupKey:               "trace_id",
		HasError:               true,
		PointCount:             3,
		TraceStartTimeUnixNano: now.UnixNano(),
		TraceEndTimeUnixNano:   now.Add(10 * time.Millisecond).UnixNano(),
		MaxPointTimeUnixNano:   now.Add(20 * time.Millisecond).UnixNano(),
	}
	records := metrics.OnIngest(trace)
	assert.Contains(t, metricRecordNames(records), "trace_total_count")
	assert.Contains(t, metricRecordNames(records), "trace_error_count")
	assert.Contains(t, metricRecordNames(records), "span_total_count")

	preDecision := metrics.OnPreDecision(trace)
	require.Len(t, preDecision, 1)
	assert.Equal(t, "trace_duration", preDecision[0].MetricName)
	assert.Equal(t, DerivedMetricKindHistogram, preDecision[0].Kind)
	assert.Equal(t, now.Add(20*time.Millisecond), preDecision[0].Time)
	assert.Equal(t, "ddtrace", preDecision[0].Tags["source"])

	kept := metrics.OnDecision(trace, DerivedMetricDecisionKept)
	assert.Contains(t, metricRecordNames(kept), "trace_kept_count")
	dropped := metrics.OnDecision(trace, DerivedMetricDecisionDropped)
	assert.Contains(t, metricRecordNames(dropped), "trace_dropped_count")

	logging := &DataPacket{Token: "token-a", DataType: point.SLogging, HasError: true, GroupKey: "fields.user"}
	assert.Contains(t, metricRecordNames(metrics.OnIngest(logging)), "logging_total_count")
	assert.Contains(t, metricRecordNames(metrics.OnIngest(logging)), "logging_error_count")
	assert.Contains(t, metricRecordNames(metrics.OnDecision(logging, DerivedMetricDecisionKept)), "logging_kept_count")
	assert.Contains(t, metricRecordNames(metrics.OnDecision(logging, DerivedMetricDecisionDropped)), "logging_dropped_count")

	rum := &DataPacket{Token: "token-a", DataType: point.SRUM}
	assert.Contains(t, metricRecordNames(metrics.OnIngest(rum)), "rum_total_count")
	assert.Contains(t, metricRecordNames(metrics.OnDecision(rum, DerivedMetricDecisionKept)), "rum_kept_count")
	assert.Contains(t, metricRecordNames(metrics.OnDecision(rum, DerivedMetricDecisionDropped)), "rum_dropped_count")

	assert.Nil(t, (&countBuiltinDerivedMetric{}).OnIngest(trace))
	assert.Nil(t, (&countBuiltinDerivedMetric{}).OnDecision(trace, DerivedMetricDecisionKept))
	assert.Nil(t, (&histogramBuiltinDerivedMetric{}).OnPreDecision(trace))
	assert.Nil(t, (&histogramBuiltinDerivedMetric{}).OnIngest(trace))
	assert.Nil(t, (&histogramBuiltinDerivedMetric{}).OnDecision(trace, DerivedMetricDecisionKept))
	assert.Nil(t, DefaultTailSamplingBuiltinMetrics().OnPreDecision(&DataPacket{DataType: point.SLogging}))
	assert.NotEmpty(t, packetTime(nil))
}

func TestTailSamplingConfigStringAndRUMPick(t *testing.T) {
	cfg := &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				nil,
				{Name: "keep", Type: PipelineTypeCondition, Condition: `{ status = "ok" }`, Action: PipelineActionKeep},
			},
			DerivedMetrics: []*DerivedMetric{
				nil,
				{Name: "custom"},
			},
			BuiltinMetrics: []*BuiltinMetricCfg{
				nil,
				{Name: "trace_total_count", Enabled: true},
			},
		},
		Logging: &LoggingTailSampling{
			GroupDimensions: []*LoggingGroupDimension{
				nil,
				{GroupKey: "fields.user", Pipelines: []*SamplingPipeline{{Name: "sample", Type: PipelineTypeSampling, Rate: 0.1}}},
			},
		},
		RUM: &RUMTailSampling{
			GroupDimensions: []*RUMGroupDimension{
				nil,
				{GroupKey: "fields.session", Pipelines: []*SamplingPipeline{{Name: "keep", Type: PipelineTypeCondition, Action: PipelineActionKeep}}},
			},
		},
	}

	s := cfg.ToString()
	assert.Contains(t, s, "trace_total_count")
	assert.Contains(t, s, "<nil>")
	assert.Equal(t, "<nil>", (*TailSamplingConfigs)(nil).ToString())

	rumGroup := &RUMGroupDimension{GroupKey: "session"}
	pt := point.NewPoint("rum", point.KVs{}.Add("session", "s1").AddTag("status", "error"), point.CommonLoggingOptions()...)
	grouped, passedThrough := rumGroup.PickRUM("rum-source", []*point.Point{pt})
	require.Len(t, grouped, 1)
	assert.Empty(t, passedThrough)
	for _, packet := range grouped {
		assert.Equal(t, point.SRUM, packet.DataType)
		assert.Equal(t, "session", packet.GroupKey)
		assert.True(t, packet.HasError)
	}

	for _, input := range []any{float64(1.5), int64(2), uint64(3), "x", []byte("y"), true} {
		assert.NotEmpty(t, fieldToString(input))
	}
	assert.Empty(t, fieldToString(struct{}{}))
	assert.Contains(t, pipelineNames([]*SamplingPipeline{nil}), "<nil>")
	assert.Contains(t, derivedMetricNames([]*DerivedMetric{nil}), "<nil>")
	assert.Contains(t, builtinMetricNames([]*BuiltinMetricCfg{nil}), "<nil>")
	assert.Contains(t, loggingGroupStrings([]*LoggingGroupDimension{nil}), "<nil>")
	assert.Contains(t, rumGroupStrings([]*RUMGroupDimension{nil}), "<nil>")
	assert.True(t, strings.Contains(cfg.ToString(), "TailSamplingConfigs"))
}

func metricRecordNames(records []DerivedMetricRecord) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.MetricName)
	}
	return names
}
