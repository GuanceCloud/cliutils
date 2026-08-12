package aggregate

import (
	"fmt"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func predicateTracePoint(traceID, parentID string, durationUs int64, kvs ...any) *point.Point {
	fields := map[string]any{
		"trace_id":   traceID,
		"duration":   durationUs,
		"start_time": time.Now().Unix(),
	}
	if parentID != "" {
		fields["parent_id"] = parentID
	}
	for i := 0; i+1 < len(kvs); i += 2 {
		fields[fmt.Sprint(kvs[i])] = kvs[i+1]
	}

	return point.NewPoint("ddtrace", point.NewKVs(fields), point.CommonLoggingOptions()...)
}

func pickSinglePredicatePacket(t *testing.T, pts []*point.Point) *DataPacket {
	t.Helper()

	packets := PickTrace("ddtrace", pts, 1)
	require.Len(t, packets, 1)
	for _, packet := range packets {
		return packet
	}
	return nil
}

func TestSpanPredicatesExtraction(t *testing.T) {
	t.Run("error_status", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "status", "error"),
		})
		assert.True(t, packet.PredError)
	})

	t.Run("http_5xx", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "http_status_code", "500"),
		})
		assert.True(t, packet.PredHttpError)
	})

	t.Run("http_2xx_not_error", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "http_status_code", "200"),
		})
		assert.False(t, packet.PredHttpError)
	})

	t.Run("http_status_non_string_not_error", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "http_status_code", int64(500)),
		})
		assert.False(t, packet.PredHttpError)
	})

	t.Run("biz_error", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "body_code", "BIZ_1001"),
		})
		assert.True(t, packet.PredBizError)
	})

	t.Run("biz_code_200_not_error", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "body_code", "200"),
		})
		assert.False(t, packet.PredBizError)
	})

	t.Run("biz_code_missing_not_error", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000),
		})
		assert.False(t, packet.PredBizError)
	})

	t.Run("trace_keep", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 1000, "trace_keep", true),
		})
		assert.True(t, packet.PredTraceKeep)
	})

	t.Run("root_slow_duration", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "0", 1500000),
		})
		assert.Equal(t, int64(1500000), packet.RootDurationUs)
		assert.Equal(t, int64(1500000), packet.MaxSpanDurationUs)
		assert.Equal(t, int64(0), packet.MaxNonrootDurationUs)
	})

	t.Run("nonroot_slow_duration", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 600000),
		})
		assert.Equal(t, int64(600000), packet.MaxNonrootDurationUs)
		assert.Equal(t, int64(0), packet.RootDurationUs)
	})

	t.Run("missing_parent_id_ignored_for_root_and_nonroot", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "", 600000),
		})
		assert.Equal(t, int64(0), packet.RootDurationUs)
		assert.Equal(t, int64(0), packet.MaxNonrootDurationUs)
		assert.Equal(t, int64(600000), packet.MaxSpanDurationUs)
	})

	t.Run("multiple_spans_take_max", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 100000),
			predicateTracePoint("t", "s2", 800000),
			predicateTracePoint("t", "0", 1200000),
		})
		assert.Equal(t, int64(800000), packet.MaxNonrootDurationUs)
		assert.Equal(t, int64(1200000), packet.RootDurationUs)
		assert.Equal(t, int64(1200000), packet.MaxSpanDurationUs)
	})

	t.Run("no_duration_not_counted", func(t *testing.T) {
		packet := pickSinglePredicatePacket(t, []*point.Point{
			predicateTracePoint("t", "s1", 0),
		})
		assert.Equal(t, int64(0), packet.MaxSpanDurationUs)
	})
}

func TestSpanPredicatesMatchFilterSemantics(t *testing.T) {
	// 与 bingX 调研报告真实规则集对齐，验证谓词摘要与 filter 条件求值结果一致。
	rule1 := &SamplingPipeline{
		Name:      "keep-technical-or-business-error",
		Type:      PipelineTypeCondition,
		Condition: `{ status = "error" OR http_status_code MATCH ["^[45][0-9][0-9]$"] OR body_code NOTIN ["0", "200", null] }`,
		Action:    PipelineActionKeep,
	}
	require.NoError(t, rule1.Apply())

	rule2 := &SamplingPipeline{
		Name:      "keep-root-slow-over-1s",
		Type:      PipelineTypeCondition,
		Condition: `{ parent_id = "0" AND duration > 1000000 }`,
		Action:    PipelineActionKeep,
	}
	require.NoError(t, rule2.Apply())

	rule3 := &SamplingPipeline{
		Name:      "keep-nonroot-slow-over-500ms",
		Type:      PipelineTypeCondition,
		Condition: `{ parent_id != "0" AND duration > 500000 }`,
		Action:    PipelineActionKeep,
	}
	require.NoError(t, rule3.Apply())

	cases := []struct {
		name      string
		pts       []*point.Point
		wantRule1 bool
		wantRule2 bool
		wantRule3 bool
	}{
		{
			name:      "all_normal",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 10000), predicateTracePoint("t", "0", 20000)},
			wantRule1: false, wantRule2: false, wantRule3: false,
		},
		{
			name:      "error_span",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 10000, "status", "error")},
			wantRule1: true, wantRule2: false, wantRule3: false,
		},
		{
			name:      "http_500_span",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 10000, "http_status_code", "500")},
			wantRule1: true, wantRule2: false, wantRule3: false,
		},
		{
			name:      "biz_error_span",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 10000, "body_code", "BIZ_1001")},
			wantRule1: true, wantRule2: false, wantRule3: false,
		},
		{
			name:      "root_slow",
			pts:       []*point.Point{predicateTracePoint("t", "0", 1500000)},
			wantRule1: false, wantRule2: true, wantRule3: false,
		},
		{
			name:      "nonroot_slow",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 600000)},
			wantRule1: false, wantRule2: false, wantRule3: true,
		},
		{
			name:      "mixed_error_and_normal",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 10000), predicateTracePoint("t", "s2", 10000, "status", "error")},
			wantRule1: true, wantRule2: false, wantRule3: false,
		},
		{
			name:      "trace_keep",
			pts:       []*point.Point{predicateTracePoint("t", "s1", 10000, "trace_keep", true)},
			wantRule1: false, wantRule2: false, wantRule3: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packet := pickSinglePredicatePacket(t, tc.pts)

			gotRule1 := packet.PredError || packet.PredHttpError || packet.PredBizError
			assert.Equal(t, tc.wantRule1, gotRule1)

			gotRule2 := packet.RootDurationUs > 1000000
			assert.Equal(t, tc.wantRule2, gotRule2)

			gotRule3 := packet.MaxNonrootDurationUs > 500000
			assert.Equal(t, tc.wantRule3, gotRule3)

			matched1, _ := evaluatePipelines(packet, []*SamplingPipeline{rule1})
			assert.Equal(t, tc.wantRule1, matched1)

			matched2, _ := evaluatePipelines(packet, []*SamplingPipeline{rule2})
			assert.Equal(t, tc.wantRule2, matched2)

			matched3, _ := evaluatePipelines(packet, []*SamplingPipeline{rule3})
			assert.Equal(t, tc.wantRule3, matched3)
		})
	}
}

func TestMergeSpanPredicatesAcrossDataKits(t *testing.T) {
	// 模拟同一 trace 的 span 分散在两个 Datakit：
	// datakit-a 见 error + root 慢；datakit-b 见 5xx + 非根慢。
	packetA := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t", "0", 1500000, "status", "error"),
		predicateTracePoint("t", "s1", 10000),
	})
	packetB := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t", "s2", 600000, "http_status_code", "503"),
	})

	merged := &DataPacket{}
	mergeSpanPredicates(merged, packetA)
	mergeSpanPredicates(merged, packetB)

	assert.True(t, merged.PredError)
	assert.True(t, merged.PredHttpError)
	assert.False(t, merged.PredBizError)
	assert.False(t, merged.PredTraceKeep)
	assert.Equal(t, int64(1500000), merged.RootDurationUs)
	assert.Equal(t, int64(600000), merged.MaxNonrootDurationUs)
	assert.Equal(t, int64(1500000), merged.MaxSpanDurationUs)

	// 合并等价性：OR 结果 == 对合并后完整 payload 的 filter 求值。
	require.NoError(t, mergePacketPayload(packetA, packetB))
	rule1 := &SamplingPipeline{
		Name:      "keep-error",
		Type:      PipelineTypeCondition,
		Condition: `{ status = "error" OR http_status_code MATCH ["^[45][0-9][0-9]$"] }`,
	}
	require.NoError(t, rule1.Apply())
	matched, _ := evaluatePipelines(packetA, []*SamplingPipeline{rule1})
	assert.True(t, matched)
}

func TestGlobalSamplerIngestMergePredicates(t *testing.T) {
	const token = "tkn_merge_predicates"

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

	packetA := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t", "0", 1500000),
	})
	packetA.Token = token
	sampler.Ingest(packetA)

	packetB := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t", "s1", 600000, "status", "error"),
	})
	packetB.Token = token
	sampler.Ingest(packetB)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	for _, dg := range expired {
		require.NotNil(t, dg)
		assert.True(t, dg.packet.PredError)
		assert.Equal(t, int64(1500000), dg.packet.RootDurationUs)
		assert.Equal(t, int64(600000), dg.packet.MaxNonrootDurationUs)
	}
}
