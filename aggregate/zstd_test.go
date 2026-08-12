package aggregate

import (
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTracePoint(traceID, spanID string, idx int) *point.Point {
	kvs := point.NewKVs(map[string]any{
		"resource":   "/api/order/" + strings.Repeat("x", 64) + "?idx=" + string(rune('a'+idx%26)),
		"trace_id":   traceID,
		"span_id":    spanID,
		"start_time": time.Now().Unix(),
		"duration":   1000,
	}).SetTag("service", "demo-order")
	return point.NewPoint("ddtrace", kvs, point.CommonLoggingOptions()...)
}

func testCompressedTracePackets(t *testing.T, traceID string, spanCount int) *DataPacket {
	t.Helper()

	pts := make([]*point.Point, 0, spanCount)
	for i := 0; i < spanCount; i++ {
		pts = append(pts, testTracePoint(traceID, "span-"+string(rune('0'+i%10)), i))
	}

	packets := PickTrace("ddtrace", pts, 1)
	require.Len(t, packets, 1)
	for _, packet := range packets {
		return packet
	}
	return nil
}

func TestCompressPointsPayload(t *testing.T) {
	t.Run("empty_payload_returns_none", func(t *testing.T) {
		out, compression, err := CompressPointsPayload(nil)
		require.NoError(t, err)
		assert.Equal(t, PayloadCompressionNone, compression)
		assert.Nil(t, out)
	})

	t.Run("small_payload_falls_back_to_none", func(t *testing.T) {
		payload := []byte("tiny")
		out, compression, err := CompressPointsPayload(payload)
		require.NoError(t, err)
		assert.Equal(t, PayloadCompressionNone, compression)
		assert.Equal(t, payload, out)
	})

	t.Run("large_payload_roundtrip", func(t *testing.T) {
		payload := []byte(strings.Repeat("the quick brown fox jumps over the lazy dog. ", 2048))
		compressed, compression, err := CompressPointsPayload(payload)
		require.NoError(t, err)
		assert.Equal(t, PayloadCompressionZstd, compression)

		decompressed, err := DecompressPointsPayload(compressed, compression)
		require.NoError(t, err)
		assert.Equal(t, payload, decompressed)
	})

	t.Run("none_compression_returns_input", func(t *testing.T) {
		payload := []byte("hello")
		out, err := DecompressPointsPayload(payload, PayloadCompressionNone)
		require.NoError(t, err)
		assert.Equal(t, payload, out)
	})

	t.Run("unknown_compression_errors", func(t *testing.T) {
		_, err := DecompressPointsPayload([]byte("hello"), 99)
		require.Error(t, err)
	})

	t.Run("corrupted_payload_errors", func(t *testing.T) {
		_, err := DecompressPointsPayload([]byte("not-a-zstd-stream"), PayloadCompressionZstd)
		require.Error(t, err)
	})
}

func TestPickTraceCompressesAndWalks(t *testing.T) {
	packet := testCompressedTracePackets(t, "trace-compress-1", 2000)
	require.NotNil(t, packet)

	assert.Equal(t, PayloadCompressionZstd, packet.PayloadCompression)
	assert.Less(t, len(packet.PointsPayload), 1024*1024, "compressed payload should be small")

	var count int
	err := packet.WalkRawPBPoints(func(_ []byte) bool {
		count++
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, 2000, count)

	decoded, err := DecompressPointsPayload(packet.PointsPayload, packet.PayloadCompression)
	require.NoError(t, err)
	assert.Equal(t, int32(2000), packet.PointCount)
	assert.Greater(t, len(decoded), len(packet.PointsPayload))
}

func TestPickTraceSmallGroupContentCorrect(t *testing.T) {
	pts := []*point.Point{testTracePoint("trace-small-1", "span-1", 1)}
	packets := PickTrace("ddtrace", pts, 1)
	require.Len(t, packets, 1)

	for _, packet := range packets {
		assert.Equal(t, int32(1), packet.PointCount)

		var count int
		require.NoError(t, packet.WalkRawPBPoints(func(_ []byte) bool {
			count++
			return true
		}))
		assert.Equal(t, 1, count)
	}
}

func TestMergePacketPayload(t *testing.T) {
	walkCount := func(t *testing.T, packet *DataPacket) int {
		t.Helper()
		var count int
		require.NoError(t, packet.WalkRawPBPoints(func(_ []byte) bool {
			count++
			return true
		}))
		return count
	}

	t.Run("both_uncompressed_appends_directly", func(t *testing.T) {
		dst := &DataPacket{PointsPayload: []byte("abc"), PayloadCompression: PayloadCompressionNone}
		src := &DataPacket{PointsPayload: []byte("def"), PayloadCompression: PayloadCompressionNone}
		require.NoError(t, mergePacketPayload(dst, src))
		assert.Equal(t, []byte("abcdef"), dst.PointsPayload)
		assert.Equal(t, PayloadCompressionNone, dst.PayloadCompression)
	})

	t.Run("dst_compressed_src_uncompressed", func(t *testing.T) {
		dst := testCompressedTracePackets(t, "merge-1", 100)
		src := &DataPacket{PointsPayload: dst.PointsPayload, PayloadCompression: dst.PayloadCompression, PointCount: 100}
		srcPayload, err := DecompressPointsPayload(src.PointsPayload, src.PayloadCompression)
		require.NoError(t, err)
		src = &DataPacket{PointsPayload: srcPayload, PayloadCompression: PayloadCompressionNone, PointCount: 100}

		require.NoError(t, mergePacketPayload(dst, src))
		assert.Equal(t, PayloadCompressionZstd, dst.PayloadCompression)
		assert.Equal(t, 200, walkCount(t, dst))
	})

	t.Run("dst_uncompressed_src_compressed", func(t *testing.T) {
		src := testCompressedTracePackets(t, "merge-2", 100)
		require.Equal(t, PayloadCompressionZstd, src.PayloadCompression)

		srcPayload, err := DecompressPointsPayload(src.PointsPayload, src.PayloadCompression)
		require.NoError(t, err)
		dst := &DataPacket{PointsPayload: srcPayload[:len(srcPayload)/2], PayloadCompression: PayloadCompressionNone}

		require.NoError(t, mergePacketPayload(dst, src))
		assert.Equal(t, PayloadCompressionZstd, dst.PayloadCompression)
		assert.Greater(t, walkCount(t, dst), 0)
	})

	t.Run("both_compressed", func(t *testing.T) {
		dst := testCompressedTracePackets(t, "merge-3", 100)
		src := testCompressedTracePackets(t, "merge-3", 100)
		require.NoError(t, mergePacketPayload(dst, src))
		assert.Equal(t, PayloadCompressionZstd, dst.PayloadCompression)
		assert.Equal(t, 200, walkCount(t, dst))
	})

	t.Run("nil_handling", func(t *testing.T) {
		require.NoError(t, mergePacketPayload(nil, nil))
		require.NoError(t, mergePacketPayload(&DataPacket{}, nil))
	})
}

func TestGlobalSamplerIngestMergeCompressedPayload(t *testing.T) {
	const token = "tkn_merge_compressed"

	sampler := NewGlobalSampler(1, time.Second)
	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
	}))

	first := testCompressedTracePackets(t, "trace-integration", 100)
	first.Token = token
	require.Equal(t, PayloadCompressionZstd, first.PayloadCompression)
	sampler.Ingest(first)

	second := testCompressedTracePackets(t, "trace-integration", 100)
	second.Token = token
	sampler.Ingest(second)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	var dg *DataGroup
	for _, item := range expired {
		dg = item
	}
	require.NotNil(t, dg)

	var count int
	require.NoError(t, dg.packet.WalkRawPBPoints(func(_ []byte) bool {
		count++
		return true
	}))
	assert.Equal(t, 200, count)
	assert.Equal(t, int32(200), dg.packet.PointCount)
	assert.Equal(t, PayloadCompressionZstd, dg.packet.PayloadCompression)
}
