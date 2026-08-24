package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
)

const (
	benchmarkTailSamplingToken = "benchmark-token"
	benchmarkTraceGroupKey     = "trace_id"
)

func BenchmarkGlobalSamplerTimeWheelIngestNewGroups(b *testing.B) {
	const groupCount = 1024
	const ttl = 10 * time.Second

	sampler := newBenchmarkGlobalSampler(b, ttl, nil)
	packets := newBenchmarkTracePackets(groupCount, benchmarkTracePayload())

	b.ReportAllocs()
	b.ReportMetric(groupCount, "groups/cycle")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sampler.Ingest(packets[i%groupCount])
		if (i+1)%groupCount == 0 {
			b.StopTimer()
			drainBenchmarkSampler(sampler, int(ttl.Seconds()))
			b.StartTimer()
		}
	}
}

func BenchmarkGlobalSamplerTimeWheelIngestMergeGroup(b *testing.B) {
	const packetCount = 1024
	const ttl = 10 * time.Second

	sampler := newBenchmarkGlobalSampler(b, ttl, nil)
	sampler.Ingest(newBenchmarkTracePacket(1, nil))
	packets := make([]*DataPacket, 0, packetCount)
	for i := 0; i < packetCount; i++ {
		packets = append(packets, newBenchmarkTracePacket(1, nil))
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sampler.Ingest(packets[i%packetCount])
	}
}

func BenchmarkGlobalSamplerTimeWheelAdvanceTimeExpiredGroups(b *testing.B) {
	const groupCount = 1024
	const ttl = time.Second

	sampler := newBenchmarkGlobalSampler(b, ttl, nil)
	packets := newBenchmarkTracePackets(groupCount, nil)

	b.ReportAllocs()
	b.ReportMetric(groupCount, "groups/op")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for _, packet := range packets {
			sampler.Ingest(packet)
		}
		b.StartTimer()

		expired := sampler.AdvanceTime()
		if len(expired) != groupCount {
			b.Fatalf("expired groups = %d, want %d", len(expired), groupCount)
		}

		b.StopTimer()
		releaseBenchmarkDataGroups(expired)
		b.StartTimer()
	}
}

func BenchmarkGlobalSamplerTimeWheelTailSamplingOutcomesKeepAll(b *testing.B) {
	const groupCount = 1024
	const ttl = time.Second

	pipelines := []*SamplingPipeline{
		{
			Name: "keep_all",
			Type: PipelineTypeSampling,
			Rate: 1,
		},
	}
	sampler := newBenchmarkGlobalSampler(b, ttl, pipelines)
	packets := newBenchmarkTracePackets(groupCount, benchmarkTracePayload())

	b.ReportAllocs()
	b.ReportMetric(groupCount, "groups/op")
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		b.StopTimer()
		dataGroups := newBenchmarkDataGroups(packets)
		b.StartTimer()

		outcomes := sampler.TailSamplingOutcomes(dataGroups)
		if len(outcomes) != groupCount {
			b.Fatalf("outcomes = %d, want %d", len(outcomes), groupCount)
		}
	}
}

func BenchmarkTailSamplingProcessorDecisionTick120K(b *testing.B) {
	const groupCount = 120_000

	scenarios := []struct {
		name           string
		pipelines      []*SamplingPipeline
		payload        []byte
		prepare        func(*DataPacket)
		defaultMetrics bool
	}{
		{
			name: "fast_predicates",
			pipelines: []*SamplingPipeline{
				{Name: "keep_error", Type: PipelineTypeCondition, Condition: `{ status = "error" }`, Action: PipelineActionKeep},
			},
			payload: benchmarkTracePayload(),
			prepare: func(packet *DataPacket) {
				packet.PredError = true
				packet.PredicateSummaryVersion = CurrentPredicateSummaryVersion
			},
		},
		{
			name: "custom_condition",
			pipelines: []*SamplingPipeline{
				{Name: "keep_custom", Type: PipelineTypeCondition, Condition: `{ custom_flag = "keep" }`, Action: PipelineActionKeep},
			},
			payload: benchmarkCustomTracePayload(),
		},
		{
			name: "custom_condition_with_default_metrics",
			pipelines: []*SamplingPipeline{
				{Name: "keep_custom", Type: PipelineTypeCondition, Condition: `{ custom_flag = "keep" }`, Action: PipelineActionKeep},
			},
			payload:        benchmarkCustomTracePayload(),
			defaultMetrics: true,
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			packets := newBenchmarkTracePackets(groupCount, scenario.payload)
			if scenario.prepare != nil {
				for _, packet := range packets {
					scenario.prepare(packet)
				}
			}

			b.Run("serial", func(b *testing.B) {
				benchmarkDecisionTicks(b, packets, scenario.pipelines, scenario.defaultMetrics, false)
			})
			b.Run("sharded", func(b *testing.B) {
				benchmarkDecisionTicks(b, packets, scenario.pipelines, scenario.defaultMetrics, true)
			})
		})
	}
}

func benchmarkDecisionTicks(
	b *testing.B,
	packets []*DataPacket,
	pipelines []*SamplingPipeline,
	defaultMetrics bool,
	sharded bool,
) {
	b.Helper()
	processor := newBenchmarkTailSamplingProcessor(b, pipelines, defaultMetrics)
	b.ReportAllocs()
	b.ReportMetric(float64(len(packets)), "groups/tick")
	b.ResetTimer()

	for range b.N {
		b.StopTimer()
		for _, packet := range packets {
			processor.IngestPacket(packet)
		}
		b.StartTimer()

		if sharded {
			tick := processor.ProcessTick()
			if len(tick.Outcomes) != len(packets) {
				b.Fatalf("outcomes = %d, want %d", len(tick.Outcomes), len(packets))
			}
			continue
		}

		expired := processor.AdvanceTime()
		outcomes := processor.TailSamplingOutcomes(expired)
		if len(outcomes) != len(packets) {
			b.Fatalf("outcomes = %d, want %d", len(outcomes), len(packets))
		}
	}
}

func newBenchmarkTailSamplingProcessor(
	tb testing.TB,
	pipelines []*SamplingPipeline,
	defaultMetrics bool,
) *TailSamplingProcessor {
	tb.Helper()

	const ttl = time.Second
	sampler := NewGlobalSampler(64, ttl)
	err := sampler.UpdateConfig(benchmarkTailSamplingToken, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL:   ttl,
			GroupKey:  benchmarkTraceGroupKey,
			Pipelines: pipelines,
		},
	})
	if err != nil {
		tb.Fatalf("update tail sampling config: %v", err)
	}

	if defaultMetrics {
		return NewTailSamplingProcessor(
			sampler,
			NewDerivedMetricCollector(DefaultDerivedMetricFlushWindow),
			DefaultTailSamplingBuiltinMetrics(),
		)
	}

	return NewTailSamplingProcessor(sampler, nil, nil)
}

func newBenchmarkGlobalSampler(tb testing.TB, ttl time.Duration, pipelines []*SamplingPipeline) *GlobalSampler {
	tb.Helper()

	sampler := NewGlobalSampler(16, ttl)
	err := sampler.UpdateConfig(benchmarkTailSamplingToken, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL:   ttl,
			GroupKey:  benchmarkTraceGroupKey,
			Pipelines: pipelines,
		},
	})
	if err != nil {
		tb.Fatalf("update tail sampling config: %v", err)
	}

	return sampler
}

func newBenchmarkTracePackets(count int, payload []byte) []*DataPacket {
	packets := make([]*DataPacket, 0, count)
	for i := 0; i < count; i++ {
		packets = append(packets, newBenchmarkTracePacket(uint64(i+1), payload))
	}
	return packets
}

func newBenchmarkTracePacket(groupIDHash uint64, payload []byte) *DataPacket {
	return &DataPacket{
		GroupIdHash:          groupIDHash,
		RawGroupId:           benchmarkTraceGroupKey,
		Token:                benchmarkTailSamplingToken,
		DataType:             point.STracing,
		Source:               "benchmark",
		ConfigVersion:        1,
		GroupKey:             benchmarkTraceGroupKey,
		PointCount:           1,
		PointsPayload:        payload,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
}

func benchmarkTracePayload() []byte {
	return point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "benchmark-span"})
}

func benchmarkCustomTracePayload() []byte {
	return point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{
		Name: "benchmark-span",
		Fields: []*point.Field{
			{Key: "custom_flag", Val: &point.Field_S{S: "keep"}},
		},
	})
}

func drainBenchmarkSampler(sampler *GlobalSampler, seconds int) {
	for i := 0; i < seconds; i++ {
		releaseBenchmarkDataGroups(sampler.AdvanceTime())
	}
}

func newBenchmarkDataGroups(packets []*DataPacket) map[uint64]*DataGroup {
	dataGroups := make(map[uint64]*DataGroup, len(packets))
	for _, packet := range packets {
		key := tailSamplingGroupMapKey(packet)
		dataGroups[key] = &DataGroup{
			dataType: point.STracing,
			packet:   packet,
		}
	}
	return dataGroups
}

func releaseBenchmarkDataGroups(dataGroups map[uint64]*DataGroup) {
	for _, dg := range dataGroups {
		if dg == nil {
			continue
		}
		dg.Reset()
		dataGroupPool.Put(dg)
	}
}
