package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTracePacketForDerivedMetrics() *DataPacket {
	now := time.Now()

	pt := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-1",
		"resource": "/orders",
		"service":  "checkout",
	}), point.CommonLoggingOptions()...)
	pt.SetTime(now)

	return &DataPacket{
		GroupIdHash: 1,
		RawGroupId:  "trace-1",
		Token:       "token-a",
		DataType:    point.STracing,
		Points:      []*point.PBPoint{pt.PBPoint()},
	}
}

func TestAggregateDerivedMetricsSink_ConsumeDerivedMetrics(t *testing.T) {
	cache := NewCache(time.Minute)
	sink := NewAggregateDerivedMetricsSink(cache)

	packet := newTracePacketForDerivedMetrics()
	derivedBatchs := BuildDerivedMetricBatches(packet, []*DerivedMetric{TraceTotalCount}, 0)
	require.NotEmpty(t, derivedBatchs)
	assert.Equal(t, DefaultDerivedMetricWindowSeconds, derivedBatchs[0].AggregationOpts["trace_total_count"].Window)

	err := sink.ConsumeDerivedMetrics(packet.Token, derivedBatchs)
	require.NoError(t, err)
	require.NotEmpty(t, cache.WindowsBuckets)

	var calcCount int
	for _, windows := range cache.WindowsBuckets {
		for _, window := range windows.WS {
			if window.Token == packet.Token {
				calcCount += len(window.cache)
			}
		}
	}

	assert.Greater(t, calcCount, 0)
}

func TestGlobalSampler_HandleDerivedMetricsUsesSink(t *testing.T) {
	cache := NewCache(time.Minute)

	sampler := NewGlobalSampler(1, time.Minute)
	sampler.SetDerivedMetricsSink(NewAggregateDerivedMetricsSink(cache))

	packet := newTracePacketForDerivedMetrics()
	sampler.handleDerivedMetrics(packet, point.STracing, []*DerivedMetric{TraceTotalCount})

	require.NotEmpty(t, cache.WindowsBuckets)
}

func TestResolveBuiltinDerivedMetricsByDataType(t *testing.T) {
	metrics := resolveBuiltinDerivedMetrics(point.STracing, []*BuiltinDerivedMetricConfig{
		{Name: "trace_total_count", Enabled: true},
		{Name: "logging_total_count", Enabled: true},
		{Name: "trace_error_count", Enabled: false},
	})

	require.Len(t, metrics, 1)
	assert.Equal(t, "trace_total_count", metrics[0].Name)
}

func TestGlobalSampler_TailSamplingDataUsesConfiguredBuiltinMetrics(t *testing.T) {
	cache := NewCache(time.Minute)

	sampler := NewGlobalSampler(1, time.Minute)
	sampler.SetDerivedMetricsSink(NewAggregateDerivedMetricsSink(cache))
	sampler.UpdateConfig("token-a", &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Minute,
			BuiltinDerivedMetrics: []*BuiltinDerivedMetricConfig{
				{Name: "trace_total_count", Enabled: true},
			},
			DerivedMetrics: []*DerivedMetric{
				{
					Name:      "custom_metric_todo",
					Condition: "",
					Groupby:   []string{"service"},
					Algorithm: &AggregationAlgo{
						Method:      COUNT,
						SourceField: DerivedMetricFieldTraceID,
					},
				},
			},
		},
	})

	packet := newTracePacketForDerivedMetrics()
	packet.Token = "token-a"

	sampler.TailSamplingData(map[uint64]*DataGroup{
		1: {
			dataType: point.STracing,
			td:       packet,
		},
	})

	require.NotEmpty(t, cache.WindowsBuckets)
}
