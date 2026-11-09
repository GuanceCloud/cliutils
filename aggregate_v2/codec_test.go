// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEasyUnmarshalSinglePacket(t *testing.T) {
	orig := &DataPacket{
		GroupIdHash:             123456789,
		RawGroupId:              "trace-abc-xyz-123456",
		Token:                   "tkn_test_workspace_01",
		Source:                  "ddtrace",
		DataType:                "tracing",
		ConfigVersion:           42,
		HasError:                true,
		GroupKey:                "trace_id",
		PointCount:              18,
		TraceStartTimeUnixNano:  1700000000000000,
		TraceEndTimeUnixNano:    1700000001000000,
		PointsPayload:           []byte("compressed-zstd-points-payload-bytes"),
		MaxPointTimeUnixNano:    1700000001000000,
		PayloadCompression:      PayloadCompressionZstd,
		PredError:               true,
		PredHttpError:           true,
		PredBizError:            false,
		PredTraceKeep:           true,
		MaxSpanDurationUs:       550000,
		RootDurationUs:          400000,
		MaxNonrootDurationUs:    550000,
		PredicateSummaryVersion: 1,
	}

	wireBytes, err := MarshalDataPacket(orig)
	require.NoError(t, err)
	require.NotEmpty(t, wireBytes)

	// Unmarshal using easyproto zero-allocation parser
	var parsed DataPacket
	err = UnmarshalSinglePacket(wireBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, orig.GroupIdHash, parsed.GroupIdHash)
	assert.Equal(t, orig.RawGroupId, parsed.RawGroupId)
	assert.Equal(t, orig.Token, parsed.Token)
	assert.Equal(t, orig.Source, parsed.Source)
	assert.Equal(t, orig.DataType, parsed.DataType)
	assert.Equal(t, orig.ConfigVersion, parsed.ConfigVersion)
	assert.Equal(t, orig.HasError, parsed.HasError)
	assert.Equal(t, orig.GroupKey, parsed.GroupKey)
	assert.Equal(t, orig.PointCount, parsed.PointCount)
	assert.Equal(t, orig.TraceStartTimeUnixNano, parsed.TraceStartTimeUnixNano)
	assert.Equal(t, orig.TraceEndTimeUnixNano, parsed.TraceEndTimeUnixNano)
	assert.True(t, bytes.Equal(orig.PointsPayload, parsed.PointsPayload))
	assert.Equal(t, orig.PayloadCompression, parsed.PayloadCompression)
	assert.Equal(t, orig.PredError, parsed.PredError)
	assert.Equal(t, orig.PredHttpError, parsed.PredHttpError)
	assert.Equal(t, orig.PredBizError, parsed.PredBizError)
	assert.Equal(t, orig.PredTraceKeep, parsed.PredTraceKeep)
	assert.Equal(t, orig.MaxSpanDurationUs, parsed.MaxSpanDurationUs)
	assert.Equal(t, orig.RootDurationUs, parsed.RootDurationUs)
	assert.Equal(t, orig.MaxNonrootDurationUs, parsed.MaxNonrootDurationUs)
	assert.Equal(t, orig.PredicateSummaryVersion, parsed.PredicateSummaryVersion)
}

func TestEasyUnmarshalBatchPackets(t *testing.T) {
	batch := &DataBatch{
		CollectorId: "collector-sh-01",
		Packets: []*DataPacket{
			{
				GroupIdHash:       101,
				RawGroupId:        "trace-101",
				Token:             "tkn_01",
				DataType:          "tracing",
				HasError:          false,
				PointCount:        5,
				MaxSpanDurationUs: 120000,
			},
			{
				GroupIdHash:       102,
				RawGroupId:        "trace-102",
				Token:             "tkn_01",
				DataType:          "tracing",
				HasError:          true,
				PointCount:        8,
				MaxSpanDurationUs: 950000,
			},
		},
	}

	wireBytes, err := MarshalDataBatch(batch)
	require.NoError(t, err)

	bp := NewBatchPackets()
	defer bp.Release()

	err = bp.UnmarshalBatch(wireBytes)
	require.NoError(t, err)

	assert.Equal(t, "collector-sh-01", bp.CollectorID)
	require.Len(t, bp.Packets, 2)
	assert.Equal(t, uint64(101), bp.Packets[0].GroupIdHash)
	assert.Equal(t, "trace-101", bp.Packets[0].RawGroupId)
	assert.False(t, bp.Packets[0].HasError)

	assert.Equal(t, uint64(102), bp.Packets[1].GroupIdHash)
	assert.Equal(t, "trace-102", bp.Packets[1].RawGroupId)
	assert.True(t, bp.Packets[1].HasError)
}

func BenchmarkEasyProtoBatch(b *testing.B) {
	batch := &DataBatch{
		CollectorId: "collector-bench-01",
		Packets:     make([]*DataPacket, 0, 50),
	}
	for i := range 50 {
		batch.Packets = append(batch.Packets, &DataPacket{
			GroupIdHash:       uint64(1000 + i),
			RawGroupId:        "trace-id-benchmark-sample",
			Token:             "tkn_benchmark_workspace",
			Source:            "ddtrace",
			DataType:          "tracing",
			HasError:          i%5 == 0,
			PointCount:        int32(10 + i),
			PointsPayload:     []byte("compressed-payload-chunk-data"),
			MaxSpanDurationUs: int64(100000 * (i + 1)),
		})
	}

	b.Run("easyproto-encode", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			_, err := MarshalDataBatch(batch)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	wireBytes, err := MarshalDataBatch(batch)
	require.NoError(b, err)

	b.Run("easyproto-decode-pooled", func(b *testing.B) {
		bp := NewBatchPackets()
		defer bp.Release()

		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			bp.Reset()
			if err := bp.UnmarshalBatch(wireBytes); err != nil {
				b.Fatal(err)
			}
		}
	})
}
