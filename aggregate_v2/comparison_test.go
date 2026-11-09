// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"testing"
	"time"

	v1 "github.com/GuanceCloud/cliutils/aggregate"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

// Helper to build realistic HTTP request bodies containing traces with multiple spans.
func generateBenchmarkHTTPPayloads(requestCount int, spansPerTrace int) ([][]byte, []uint64) {
	payloads := make([][]byte, requestCount)
	traceIDs := make([]uint64, 0, requestCount)

	// Create spans for several concurrent traces distributed across requests
	for reqIdx := range requestCount {
		batch := &v1.DataBatch{
			CollectorId: fmt.Sprintf("collector-node-%d", reqIdx%8),
			Packets:     make([]*v1.DataPacket, 0, 4),
		}

		for spanIdx := range 4 {
			traceID := uint64(5000 + (reqIdx*4+spanIdx)%20) // 20 concurrent traces receiving spans repeatedly
			traceIDs = append(traceIDs, traceID)

			rawSpan := fmt.Sprintf("trace_id=%d service=order-api span_id=%d duration=%dms status=ok",
				traceID, spanIdx, 10*(spanIdx+1))
			compressed, _ := CompressPayload([]byte(rawSpan))

			isErr := traceID%10 == 0
			isSlow := traceID%7 == 0
			durationUs := int64(50000)
			if isSlow {
				durationUs = 800000 // 800ms
			}

			batch.Packets = append(batch.Packets, &v1.DataPacket{
				GroupIdHash:        traceID,
				RawGroupId:         fmt.Sprintf("trace-%d", traceID),
				Token:              "tkn_bench",
				DataType:           "tracing",
				HasError:           isErr,
				PredError:          isErr,
				MaxSpanDurationUs:  durationUs,
				PointCount:         1,
				PointsPayload:      compressed,
				PayloadCompression: v1.PayloadCompressionZstd,
			})
		}

		wireBytes, err := proto.Marshal(batch)
		if err != nil {
			panic(err)
		}
		payloads[reqIdx] = wireBytes
	}

	return payloads, traceIDs
}

func BenchmarkEndToEndComparison(b *testing.B) {
	const requestCount = 50
	const spansPerTrace = 10
	payloads, _ := generateBenchmarkHTTPPayloads(requestCount, spansPerTrace)

	// Rules: keep error, keep duration >= 500ms, drop others
	v1Rules := &v1.TailSamplingConfigs{
		Tracing: &v1.TraceTailSampling{
			DataTTL: 2 * time.Second,
			Pipelines: []*v1.SamplingPipeline{
				{
					Name:      "keep-errors",
					Type:      v1.PipelineTypeCondition,
					Condition: "{ status = 'error' }",
					Action:    v1.PipelineActionKeep,
				},
				{
					Name:      "keep-slow",
					Type:      v1.PipelineTypeCondition,
					Condition: "{ duration >= 500000000 }",
					Action:    v1.PipelineActionKeep,
				},
				{
					Name:      "drop-rest",
					Type:      v1.PipelineTypeCondition,
					Condition: "",
					Action:    v1.PipelineActionDrop,
				},
			},
		},
	}
	require.NoError(b, v1Rules.Init())

	v2Rules := &TailSamplingConfigs{
		Tracing: &TraceTailSampling{
			DataTTL:         2 * time.Second,
			MaxHoldDuration: 5 * time.Second,
			Pipelines: []*SamplingPipeline{
				{
					Name:      "keep-errors",
					Type:      PipelineTypeCondition,
					Condition: "{ status = 'error' }",
					Action:    PipelineActionKeep,
				},
				{
					Name:      "keep-slow",
					Type:      PipelineTypeCondition,
					Condition: "{ duration >= 500000000 }",
					Action:    PipelineActionKeep,
				},
				{
					Name:      "drop-rest",
					Type:      PipelineTypeCondition,
					Condition: "",
					Action:    PipelineActionDrop,
				},
			},
		},
	}
	require.NoError(b, v2Rules.Init())

	b.Run("v1-end-to-end", func(b *testing.B) {
		v1Sampler := v1.NewGlobalSampler(16, 2*time.Second)
		require.NoError(b, v1Sampler.UpdateConfig("tkn_bench", v1Rules))

		b.ReportAllocs()
		b.ResetTimer()

		i := 0
		for b.Loop() {
			body := payloads[i%requestCount]

			// v1 ingress: proto.Unmarshal DataBatch
			var batch v1.DataBatch
			if err := proto.Unmarshal(body, &batch); err != nil {
				b.Fatal(err)
			}
			for _, pkt := range batch.Packets {
				v1Sampler.Ingest(pkt)
			}

			// Advance time wheel every 25 requests
			if (i+1)%25 == 0 {
				expired := v1Sampler.AdvanceTime()
				_ = v1Sampler.TailSamplingData(expired)
			}
			i++
		}
	})

	b.Run("v2-end-to-end", func(b *testing.B) {
		v2Sampler, err := NewSampler(&SamplerConfig{
			ShardCount:             16,
			DefaultTTL:             2 * time.Second,
			MaxHoldDuration:        5 * time.Second,
			MaxResidentMemoryBytes: 200 << 20,
		})
		require.NoError(b, err)
		require.NoError(b, v2Sampler.UpdateConfig("tkn_bench", v2Rules))
		defer func() { _ = v2Sampler.Close() }()

		b.ReportAllocs()
		b.ResetTimer()

		i := 0
		now := time.Now()
		for b.Loop() {
			body := payloads[i%requestCount]

			// v2 ingress: IngestBytesAt using easyproto + zero recompress
			if err := v2Sampler.IngestBytesAt(body, now); err != nil {
				b.Fatal(err)
			}

			// Advance time wheel every 25 requests
			if (i+1)%25 == 0 {
				now = now.Add(time.Second)
				_, _, _ = v2Sampler.AdvanceTime(now)
			}
			i++
		}
	})
}
