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
		{"flag_and_root", `{ status = "error" AND parent_id = "0" AND duration > 1000000 }`, true},
		// parent 单独（无 duration 阈值）严格不等价：无 duration 字段的 root span
		// 在 filter 中仍命中 parent_id="0"，但谓词摘要无法表达，保守回退。
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

// TestEvaluatePipelinesFastPathMatchesWalk 验证快速路径与解压 walk 路径的决策一致。
// 通过"1 = 1 AND (...)"恒真但不可编译的条件强制对照 pipeline 走 walk 路径。
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

	// 语义等价的 walk 对照：恒真条件使编译失败 → 强制解压 walk。
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
			require.Equal(t, PayloadCompressionZstd, packet.PayloadCompression)

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
	// 未压缩的损坏 payload：快速路径必须与原逻辑一致（不匹配）。
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

	// 空 payload：同样不匹配。
	empty := &DataPacket{PointsPayload: nil, PredError: true}
	matched, packet = evaluatePipelines(empty, []*SamplingPipeline{pipeline})
	assert.False(t, matched)
	assert.Nil(t, packet)
}
