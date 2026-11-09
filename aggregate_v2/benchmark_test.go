// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"testing"
	"time"
)

// Legacy merge logic simulation for comparison
func legacyMerge(dst, src []byte) ([]byte, error) {
	if len(src) == 0 {
		return dst, nil
	}
	if len(dst) == 0 {
		return append([]byte(nil), src...), nil
	}
	dstPlain, err := DecompressPayload(dst)
	if err != nil {
		return nil, err
	}
	srcPlain, err := DecompressPayload(src)
	if err != nil {
		return nil, err
	}
	merged := make([]byte, 0, len(dstPlain)+len(srcPlain))
	merged = append(merged, dstPlain...)
	merged = append(merged, srcPlain...)

	return CompressPayload(merged)
}

func BenchmarkMergeConcatenationVsLegacy(b *testing.B) {
	// 20 chunks simulating 20 incoming spans of a single trace
	const chunkCount = 20
	rawChunks := make([][]byte, chunkCount)
	compressedChunks := make([][]byte, chunkCount)

	for i := range chunkCount {
		rawChunks[i] = []byte(fmt.Sprintf("span-%03d: trace_id=abcdef123456 service=order-api-service endpoint=/api/v1/checkout duration=%dms status=ok", i, 10+i))
		c, err := CompressPayload(rawChunks[i])
		if err != nil {
			b.Fatal(err)
		}
		compressedChunks[i] = c
	}

	b.Run("legacy-decompress-merge-recompress", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			var accumulated []byte
			for j := range chunkCount {
				var err error
				accumulated, err = legacyMerge(accumulated, compressedChunks[j])
				if err != nil {
					b.Fatal(err)
				}
			}
		}
	})

	b.Run("v2-zstd-frame-concatenation", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			var accumulated []byte
			for j := range chunkCount {
				accumulated = ConcatZstdPayloads(accumulated, compressedChunks[j])
			}
		}
	})
}

func BenchmarkSamplerIngestAndAdvance(b *testing.B) {
	cfg := &SamplerConfig{
		ShardCount:             16,
		DefaultTTL:             2 * time.Second,
		MaxHoldDuration:        5 * time.Second,
		MaxResidentMemoryBytes: 500 << 20,
	}
	sampler, err := NewSampler(cfg)
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = sampler.Close() }()

	payload, err := CompressPayload([]byte("span-test-payload-sample-bytes-for-benchmark"))
	if err != nil {
		b.Fatal(err)
	}

	now := time.Now()
	b.ReportAllocs()
	b.ResetTimer()

	i := 0
	for b.Loop() {
		packet := &DataPacket{
			GroupIdHash:        uint64(1000 + (i % 2000)),
			RawGroupId:         "bench-trace",
			Token:              "tkn_bench",
			DataType:           "tracing",
			PointsPayload:      payload,
			PayloadCompression: PayloadCompressionZstd,
		}
		if err := sampler.Ingest(packet); err != nil {
			b.Fatal(err)
		}
		if (i+1)%500 == 0 {
			now = now.Add(time.Second)
			_, _, _ = sampler.AdvanceTime(now)
		}
		i++
	}
}
