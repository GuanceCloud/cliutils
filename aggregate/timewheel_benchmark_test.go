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

func newBenchmarkGlobalSampler(b testing.TB, ttl time.Duration, pipelines []*SamplingPipeline) *GlobalSampler {
	b.Helper()

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
		b.Fatalf("update tail sampling config: %v", err)
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
