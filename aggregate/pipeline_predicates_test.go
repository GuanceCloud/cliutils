package aggregate

import (
	"testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompilePipelinePredicateCoverage(t *testing.T) {
	cases := []struct {
		name      string
		condition string
		wantOK    bool
	}{
		{"status_error", `{ status = "error" }`, true},
		{"status_other_value", `{ status = "keep" }`, false},
		{"http_5xx", `{ http_status_code MATCH ["^[45][0-9][0-9]$"] }`, true},
		{"http_unknown_regex", `{ http_status_code MATCH ["^[45][0-9]+$"] }`, false},
		{"biz_error", `{ body_code NOTIN ["0", "200", null] }`, true},
		{"biz_error_missing_null", `{ body_code NOTIN ["0", "200"] }`, false},
		{"trace_keep", `{ trace_keep = true }`, true},
		{"root_slow", `{ parent_id = "0" AND duration > 1000000 }`, true},
		{"nonroot_slow", `{ parent_id != "0" AND duration > 500000 }`, true},
		{"any_slow", `{ duration > 500000 }`, true},
		{"parent_only", `{ parent_id = "0" }`, false},
		{"custom_field", `{ resource = "/x" }`, false},
		{"constant_condition", `{ 1 = 1 }`, false},
		// flag AND parent/duration：跨 span 组合破坏"同一 span 满足完整条件"
		// 的语义（见 TestEvaluatePipelinesCrossSpanCounterExample），必须回退 walk。
		{"flag_and_root", `{ status = "error" AND parent_id = "0" AND duration > 1000000 }`, false},
		{"flag_and_duration", `{ status = "error" AND duration > 1000000 }`, false},
		{"flag_and_flag", `{ status = "error" AND trace_keep = true }`, false},
		// parent alone (no duration threshold) is not strictly equivalent: a root
		// span without a duration field still matches parent_id="0" in the filter,
		// which the predicate summary cannot express, so it conservatively falls back.
		{"or_combination", `{ parent_id = "0" OR status = "error" }`, false},
		{"parent_conflict", `{ parent_id = "0" AND parent_id != "0" }`, false},
		{"needs_walk_nested_or", `{ parent_id = "0" AND duration > 1000000 OR status = "error" }`, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pipeline := &SamplingPipeline{
				Name:      "test",
				Type:      PipelineTypeCondition,
				Condition: tc.condition,
				Action:    PipelineActionKeep,
			}
			require.NoError(t, pipeline.Apply())

			_, ok := compilePipelinePredicate(pipeline)
			assert.Equal(t, tc.wantOK, ok)
		})
	}
}

func TestCompilePipelinePredicateMatchAll(t *testing.T) {
	pipeline := &SamplingPipeline{Name: "match-all", Type: PipelineTypeSampling, Rate: 1}
	expr, ok := compilePipelinePredicate(pipeline)
	require.True(t, ok)
	assert.True(t, expr(&DataPacket{}))

	nilExpr, ok := compilePipelinePredicate(nil)
	assert.False(t, ok)
	assert.Nil(t, nilExpr)
}

// TestEvaluatePipelinesFastPathMatchesWalk verifies the fast path and the
// decompressed-walk path make identical decisions. The walk counterpart uses
// a "1 = 1 AND (...)" always-true but uncompilable condition to force the walk.
func TestEvaluatePipelinesFastPathMatchesWalk(t *testing.T) {
	fastPipelines := []*SamplingPipeline{
		{
			Name:      "keep-error-or-5xx",
			Type:      PipelineTypeCondition,
			Condition: `{ status = "error" OR http_status_code MATCH ["^[45][0-9][0-9]$"] }`,
			Action:    PipelineActionKeep,
		},
		{
			Name:      "keep-root-slow",
			Type:      PipelineTypeCondition,
			Condition: `{ parent_id = "0" AND duration > 1000000 }`,
			Action:    PipelineActionKeep,
		},
		{
			Name:      "keep-nonroot-slow",
			Type:      PipelineTypeCondition,
			Condition: `{ parent_id != "0" AND duration > 500000 }`,
			Action:    PipelineActionKeep,
		},
	}
	for _, p := range fastPipelines {
		require.NoError(t, p.Apply())
	}

	// Semantically equivalent walk counterpart: the always-true condition fails
	// compilation, forcing the decompressed walk.
	walkPipelines := []*SamplingPipeline{
		{
			Name:      "keep-error-or-5xx-walk",
			Type:      PipelineTypeCondition,
			Condition: `{ 1 = 1 AND ( status = "error" OR http_status_code MATCH ["^[45][0-9][0-9]$"] ) }`,
			Action:    PipelineActionKeep,
		},
		{
			Name:      "keep-root-slow-walk",
			Type:      PipelineTypeCondition,
			Condition: `{ 1 = 1 AND ( parent_id = "0" AND duration > 1000000 ) }`,
			Action:    PipelineActionKeep,
		},
		{
			Name:      "keep-nonroot-slow-walk",
			Type:      PipelineTypeCondition,
			Condition: `{ 1 = 1 AND ( parent_id != "0" AND duration > 500000 ) }`,
			Action:    PipelineActionKeep,
		},
	}
	for _, p := range walkPipelines {
		require.NoError(t, p.Apply())
		_, ok := compilePipelinePredicate(p)
		assert.False(t, ok, "walk 对照 pipeline 必须不可编译")
	}

	cases := []struct {
		name string
		pts  []*point.Point
	}{
		{"all_normal", []*point.Point{
			predicateTracePoint("t", "s1", 10000), predicateTracePoint("t", "0", 20000),
		}},
		{"error_span", []*point.Point{predicateTracePoint("t", "s1", 10000, "status", "error")}},
		{"http_500_span", []*point.Point{predicateTracePoint("t", "s1", 10000, "http_status_code", "500")}},
		{"root_slow", []*point.Point{predicateTracePoint("t", "0", 1500000)}},
		{"nonroot_slow", []*point.Point{predicateTracePoint("t", "s1", 600000)}},
		{"mixed", []*point.Point{
			predicateTracePoint("t", "s1", 10000), predicateTracePoint("t", "0", 20000, "status", "error"),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			packet := pickSinglePredicatePacket(t, tc.pts)
			require.NotNil(t, packet)
			require.Contains(t, []int32{PayloadCompressionNone, PayloadCompressionZstd}, packet.PayloadCompression)

			fastMatched, fastPacket := evaluatePipelines(packet, fastPipelines)
			walkMatched, _ := evaluatePipelines(packet, walkPipelines)

			assert.Equal(t, walkMatched, fastMatched)
			if walkMatched {
				assert.NotNil(t, fastPacket)
			} else {
				assert.Nil(t, fastPacket)
			}
		})
	}
}

func TestEvaluatePipelinesFastPathMatchAll(t *testing.T) {
	fast := &SamplingPipeline{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1}
	walk := &SamplingPipeline{Name: "keep-all-walk", Type: PipelineTypeSampling, Rate: 1, Condition: `{ 1 = 1 }`}
	require.NoError(t, walk.Apply())
	_, ok := compilePipelinePredicate(walk)
	assert.False(t, ok)

	packet := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t", "s1", 10000),
	})

	fastMatched, fastPacket := evaluatePipelines(packet, []*SamplingPipeline{fast})
	walkMatched, _ := evaluatePipelines(packet, []*SamplingPipeline{walk})
	assert.Equal(t, walkMatched, fastMatched)
	assert.Equal(t, walkMatched, fastPacket != nil)
}

func TestEvaluatePipelinesFastPathCorruptedPayload(t *testing.T) {
	// Uncompressed corrupted payload: the fast path must agree with the legacy
	// logic (no match).
	pipeline := &SamplingPipeline{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1}
	corrupted := &DataPacket{
		GroupIdHash:        99,
		PayloadCompression: PayloadCompressionNone,
		PointsPayload:      []byte("bad"),
		PredError:          true,
	}

	matched, packet := evaluatePipelines(corrupted, []*SamplingPipeline{pipeline})
	assert.False(t, matched)
	assert.Nil(t, packet)

	// Empty payload: also no match.
	empty := &DataPacket{PointsPayload: nil, PredError: true}
	matched, packet = evaluatePipelines(empty, []*SamplingPipeline{pipeline})
	assert.False(t, matched)
	assert.Nil(t, packet)
}

// TestEvaluatePipelinesCrossSpanCounterExample covers the review counterexample:
// span A {status=error, short}, span B {status=ok, long}. The condition
// `status="error" AND duration>N` must NOT match in per-point semantics, so the
// compiler must reject the flag+duration combination (fall back to walk).
func TestEvaluatePipelinesCrossSpanCounterExample(t *testing.T) {
	pipeline := &SamplingPipeline{
		Name:      "cross-span-counterexample",
		Type:      PipelineTypeCondition,
		Condition: `{ status = "error" AND duration > 1000000 }`,
		Action:    PipelineActionKeep,
	}
	require.NoError(t, pipeline.Apply())

	// 编译必须回退：flag 与 duration 的 AND 无法保持同 span 关联。
	_, ok := compilePipelinePredicate(pipeline)
	assert.False(t, ok, "flag AND duration 必须回退 walk")

	// 反例数据：error 在短 span 上，长耗时在 ok span 上。
	packet := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t-cross", "s1", 1000, "status", "error"),
		predicateTracePoint("t-cross", "s2", 1500000),
	})

	// 逐点语义：没有任何 span 同时满足 status=error 且 duration>1s → 不命中。
	matched, _ := evaluatePipelines(packet, []*SamplingPipeline{pipeline})
	assert.False(t, matched, "跨 span 反例下条件不得命中")
}

// TestEvaluatePipelinesCrossSpanFlagFlagCounterExample: two flags on different
// spans must not be AND-ed across spans.
func TestEvaluatePipelinesCrossSpanFlagFlagCounterExample(t *testing.T) {
	pipeline := &SamplingPipeline{
		Name:      "cross-span-flag-flag",
		Type:      PipelineTypeCondition,
		Condition: `{ status = "error" AND trace_keep = true }`,
		Action:    PipelineActionKeep,
	}
	require.NoError(t, pipeline.Apply())

	_, ok := compilePipelinePredicate(pipeline)
	assert.False(t, ok, "flag AND flag 必须回退 walk")

	packet := pickSinglePredicatePacket(t, []*point.Point{
		predicateTracePoint("t-ff", "s1", 1000, "status", "error"),
		predicateTracePoint("t-ff", "s2", 1000, "trace_keep", true),
	})

	matched, _ := evaluatePipelines(packet, []*SamplingPipeline{pipeline})
	assert.False(t, matched, "两个 flag 分属不同 span 时条件不得命中")
}
