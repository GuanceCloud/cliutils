// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fragmentTestPacket(tb testing.TB, index int) *DataPacket {
	tb.Helper()
	status := "ok"
	if index%2 == 1 {
		status = "error"
	}
	pt := point.NewPoint("span", point.NewKVs(map[string]any{
		"trace_id": "trace-fragments", "span_id": fmt.Sprint(index), "status": status,
		"resource": strings.Repeat("r", 4096), "duration": int64(1000 + index), "parent_id": "0",
	}), point.CommonLoggingOptions()...)
	payload := point.AppendPointToPBPointsPayload(nil, pt)
	packet := &DataPacket{Token: "token", DataType: point.STracing, GroupKey: "trace_id", GroupIdHash: 42,
		RawGroupId: "trace-fragments", PointCount: 1, PointsPayload: payload}
	require.NoError(tb, ComputeSpanPredicates(packet))
	require.NoError(tb, SetDataPacketPayloadCompression(packet, PayloadCompressionZstd))
	require.Equal(tb, PayloadCompressionZstd, packet.PayloadCompression)
	return packet
}

func fragmentTestSampler(tb testing.TB, enabled bool, pipelines []*SamplingPipeline) *GlobalSampler {
	tb.Helper()
	s := NewGlobalSampler(1, time.Second)
	if enabled {
		s.EnableTracePayloadFragments()
	}
	require.NoError(tb, s.UpdateConfig("token", &TailSamplingConfigs{Version: 1,
		Tracing: &TraceTailSampling{DataTTL: time.Second, Pipelines: pipelines}}))
	return s
}

func fragmentOutcomePayload(tb testing.TB, outcome *TailSamplingOutcome) []byte {
	tb.Helper()
	packets := outcome.Packets
	if outcome.Packet != nil {
		packets = []*DataPacket{outcome.Packet}
	}
	var result []byte
	var count int32
	for _, packet := range packets {
		payload, err := DecompressPointsPayload(packet.PointsPayload, packet.PayloadCompression)
		require.NoError(tb, err)
		result = append(result, payload...)
		count += packet.PointCount
	}
	if outcome.Decision == DerivedMetricDecisionKept {
		require.Equal(tb, outcome.SourcePacket.PointCount, count)
	}
	return result
}

func TestTracePayloadFragmentsDecisionEquivalence(t *testing.T) {
	for _, tc := range []struct {
		name       string
		pipelines  []*SamplingPipeline
		fragmented bool
	}{
		{"keep", []*SamplingPipeline{{Name: "keep", Type: PipelineTypeSampling, Rate: 1}}, true},
		{"drop-before-keep", []*SamplingPipeline{
			{Name: "drop-error", Type: PipelineTypeCondition, Condition: `{ status = "error" }`, Action: PipelineActionDrop},
			{Name: "keep", Type: PipelineTypeSampling, Rate: 1}}, true},
		{"keep-before-drop", []*SamplingPipeline{
			{Name: "keep-error", Type: PipelineTypeCondition, Condition: `{ status = "error" }`, Action: PipelineActionKeep},
			{Name: "drop-error", Type: PipelineTypeCondition, Condition: `{ status = "error" }`, Action: PipelineActionDrop}}, true},
		{"complex-fallback", []*SamplingPipeline{{Name: "resource", Type: PipelineTypeCondition,
			Condition: `{ status = "error" AND duration > 1000 }`, Action: PipelineActionKeep}}, false},
		{"probability", []*SamplingPipeline{{Name: "sample", Type: PipelineTypeSampling, Rate: 0.001}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eager := fragmentTestSampler(t, false, tc.pipelines)
			deferred := fragmentTestSampler(t, true, tc.pipelines)
			for i := range 8 {
				packet := fragmentTestPacket(t, i)
				clone := *packet
				eager.Ingest(&clone)
				deferred.Ingest(packet)
			}
			key := tailSamplingGroupMapKey(fragmentTestPacket(t, 0))
			dg := deferred.shards[0].activeMap[key]
			require.Len(t, dg.fragments, 8, "ingestion must retain original fragments without encoding the prefix")
			assert.Equal(t, int32(8), dg.packet.PointCount)
			want := eager.TailSamplingOutcomes(eager.AdvanceTime())[key]
			got := deferred.TailSamplingFragmentOutcomes(deferred.AdvanceTime())[key]
			require.NotNil(t, got)
			assert.Equal(t, want.Decision, got.Decision)
			assert.Equal(t, want.MatchedRule, got.MatchedRule)
			assert.Equal(t, fragmentOutcomePayload(t, want), fragmentOutcomePayload(t, got))
			if tc.fragmented && got.Decision == DerivedMetricDecisionKept {
				require.Len(t, got.Packets, 8)
				assert.Nil(t, got.Packet)
			}
			if !tc.fragmented && got.Decision == DerivedMetricDecisionKept {
				require.NotNil(t, got.Packet)
				assert.Empty(t, got.Packets)
			}
		})
	}
}

func TestTracePayloadFragmentsTTLAndConfigChange(t *testing.T) {
	s := fragmentTestSampler(t, true, []*SamplingPipeline{{Name: "keep", Type: PipelineTypeSampling, Rate: 1}})
	packet := fragmentTestPacket(t, 0)
	s.Ingest(packet)
	expired := s.AdvanceTime()
	// A late fragment creates a new group after the previous one was sealed.
	s.Ingest(fragmentTestPacket(t, 1))
	require.NoError(t, s.UpdateConfig("token", &TailSamplingConfigs{Version: 2, Tracing: &TraceTailSampling{
		DataTTL: time.Second, Pipelines: []*SamplingPipeline{{Name: "drop", Type: PipelineTypeCondition,
			Condition: `{ status = "error" }`, Action: PipelineActionDrop}, {Name: "keep", Type: PipelineTypeSampling, Rate: 1}}}}))
	key := tailSamplingGroupMapKey(packet)
	old := s.TailSamplingFragmentOutcomes(expired)[key]
	assert.Equal(t, DerivedMetricDecisionKept, old.Decision)
	require.Len(t, old.Packets, 1)
	assert.Equal(t, int32(1), old.SourcePacket.PointCount)
	late := s.TailSamplingFragmentOutcomes(s.AdvanceTime())[key]
	assert.Equal(t, DerivedMetricDecisionDropped, late.Decision)
	require.NotNil(t, late.MatchedRule)
	assert.Equal(t, "drop", late.MatchedRule.Name)
	assert.Equal(t, int32(1), late.SourcePacket.PointCount)
}

func TestTracePayloadFragmentsBoundAndLegacyInterface(t *testing.T) {
	s := fragmentTestSampler(t, true, []*SamplingPipeline{{Name: "keep", Type: PipelineTypeSampling, Rate: 1}})
	var expected []byte
	for i := range maxTracePayloadFragments + 1 {
		packet := fragmentTestPacket(t, i)
		raw, err := DecompressPointsPayload(packet.PointsPayload, packet.PayloadCompression)
		require.NoError(t, err)
		expected = append(expected, raw...)
		s.Ingest(packet)
	}
	key := tailSamplingGroupMapKey(fragmentTestPacket(t, 0))
	assert.Empty(t, s.shards[0].activeMap[key].fragments)
	outcome := s.TailSamplingFragmentOutcomes(s.AdvanceTime())[key]
	assert.Equal(t, expected, fragmentOutcomePayload(t, outcome))

	s.Ingest(fragmentTestPacket(t, 0))
	s.Ingest(fragmentTestPacket(t, 1))
	kept := s.TailSamplingData(s.AdvanceTime())
	require.Len(t, kept, 1)
	assert.Equal(t, int32(2), kept[key].PointCount, "legacy consumers still receive one complete packet")
}

func TestTracePayloadFragmentsConcurrentIngest(t *testing.T) {
	s := fragmentTestSampler(t, true, []*SamplingPipeline{{Name: "keep", Type: PipelineTypeSampling, Rate: 1}})
	packets := make([]*DataPacket, 128)
	for i := range packets {
		packets[i] = fragmentTestPacket(t, i)
	}
	var wg sync.WaitGroup
	for _, packet := range packets {
		wg.Go(func() { s.Ingest(packet) })
	}
	wg.Wait()
	outcomes := s.TailSamplingFragmentOutcomes(s.AdvanceTime())
	require.Len(t, outcomes, 1)
	for _, outcome := range outcomes {
		require.Len(t, outcome.Packets, len(packets))
		payload := fragmentOutcomePayload(t, outcome)
		count := 0
		require.NoError(t, point.WalkPBPointsPayload(payload, func([]byte) bool { count++; return true }))
		assert.Equal(t, len(packets), count)
	}
}

func TestTracePayloadFragmentsRefreshTTLAndIsolateKeys(t *testing.T) {
	s := fragmentTestSampler(t, true, []*SamplingPipeline{{Name: "keep", Type: PipelineTypeSampling, Rate: 1}})
	cfg := s.configMap["token"]
	cfg.Tracing.DataTTL = 2 * time.Second
	s.Ingest(fragmentTestPacket(t, 0))
	assert.Empty(t, s.AdvanceTime())
	s.Ingest(fragmentTestPacket(t, 1))
	different := fragmentTestPacket(t, 2)
	different.GroupKey = "another-key"
	s.Ingest(different)
	assert.Empty(t, s.AdvanceTime(), "a new fragment refreshes the original TTL")
	outcomes := s.TailSamplingFragmentOutcomes(s.AdvanceTime())
	require.Len(t, outcomes, 2)
	counts := []int32{}
	for _, outcome := range outcomes {
		counts = append(counts, outcome.SourcePacket.PointCount)
	}
	assert.ElementsMatch(t, []int32{2, 1}, counts)
}

func BenchmarkTracePayloadFragments(b *testing.B) {
	for _, fragments := range []int{2, 8, 32, 128} {
		packets := make([]*DataPacket, fragments)
		// The same 128 points and exact payload bytes are used for every partition.
		raw := make([][]byte, 128)
		for i := range raw {
			packet := fragmentTestPacket(b, i)
			var err error
			raw[i], err = DecompressPointsPayload(packet.PointsPayload, packet.PayloadCompression)
			require.NoError(b, err)
		}
		for i := range packets {
			packet := fragmentTestPacket(b, i)
			packet.PointsPayload = bytes.Join(raw[i*(128/fragments):(i+1)*(128/fragments)], nil)
			packet.PayloadCompression = PayloadCompressionNone
			packet.PointCount = int32(128 / fragments)
			packet.PredicateSummaryVersion = 0
			packet.PredError = false
			require.NoError(b, ComputeSpanPredicates(packet))
			require.NoError(b, SetDataPacketPayloadCompression(packet, PayloadCompressionZstd))
			compactTracePayload(packet)
			packets[i] = packet
		}
		for _, enabled := range []bool{false, true} {
			b.Run(fmt.Sprintf("packets_%d/deferred_%t", fragments, enabled), func(b *testing.B) {
				b.ReportAllocs()
				for range b.N {
					s := fragmentTestSampler(b, enabled, []*SamplingPipeline{{Name: "keep", Type: PipelineTypeSampling, Rate: 1}})
					for _, packet := range packets {
						clone := *packet
						s.Ingest(&clone)
					}
					for _, outcome := range s.TailSamplingFragmentOutcomes(s.AdvanceTime()) {
						if outcome.SourcePacket.PointCount != 128 {
							b.Fatal("point count differs")
						}
					}
				}
			})
		}
	}
}
