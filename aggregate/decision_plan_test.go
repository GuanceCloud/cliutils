// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailSamplingConfigPrecompilesDecisionPlans(t *testing.T) {
	t.Parallel()

	cfg := &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			Pipelines: []*SamplingPipeline{{
				Name: "keep-error", Type: PipelineTypeCondition,
				Condition: `{ status = "error" }`, Action: PipelineActionKeep,
			}},
		},
		Logging: &LoggingTailSampling{GroupDimensions: []*LoggingGroupDimension{{
			GroupKey:  "session_id",
			Pipelines: []*SamplingPipeline{{Name: "sample", Type: PipelineTypeSampling, Rate: 1}},
		}}},
	}

	require.NoError(t, cfg.Init())
	require.NotNil(t, cfg.Tracing.decisionPlan)
	assert.True(t, cfg.Tracing.decisionPlan.fast)
	require.NotNil(t, cfg.Logging.GroupDimensions[0].decisionPlan)
	assert.True(t, cfg.Logging.GroupDimensions[0].decisionPlan.fast)
}

func TestTailSamplingOutcomeCarriesMatchedDropRuleWithoutHydratingPayload(t *testing.T) {
	t.Parallel()

	const token = "tkn_matched_rule"
	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1)
	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "keep-http", Type: PipelineTypeCondition, Condition: `{ http_status_code MATCH ["^[45][0-9][0-9]$"] }`, Action: PipelineActionKeep},
				{Name: "drop-error", Type: PipelineTypeCondition, Condition: `{ status = "error" }`, Action: PipelineActionDrop},
			},
		},
	}))

	packet := &DataPacket{
		GroupIdHash: 1, RawGroupId: "trace-1", Token: token,
		DataType: point.STracing, GroupKey: "trace_id", PointCount: 1,
		PointsPayload: []byte(strings.Repeat("x", 64)), PredError: true,
		PredicateSummaryVersion: CurrentPredicateSummaryVersion,
	}
	sampler.Ingest(packet)
	outcomes := sampler.TailSamplingOutcomes(sampler.AdvanceTime())
	require.Len(t, outcomes, 1)
	for _, outcome := range outcomes {
		require.Equal(t, DerivedMetricDecisionDropped, outcome.Decision)
		require.NotNil(t, outcome.MatchedRule)
		assert.Equal(t, 2, outcome.MatchedRule.Index)
		assert.Equal(t, "drop-error", outcome.MatchedRule.Name)
		assert.Equal(t, PipelineType(PipelineTypeCondition), outcome.MatchedRule.Type)
		assert.Equal(t, PipelineAction(PipelineActionDrop), outcome.MatchedRule.Action)
	}
	assert.Zero(t, spiller.getCount(), "谓词覆盖的 drop 决策不应读取 payload")
}

func TestComputeSpanPredicatesMarksAllZeroSummaryComplete(t *testing.T) {
	t.Parallel()

	payload := point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "span"})
	packet := &DataPacket{PointsPayload: payload}
	require.NoError(t, ComputeSpanPredicates(packet))
	assert.Equal(t, CurrentPredicateSummaryVersion, packet.PredicateSummaryVersion)
	assert.True(t, packetHasSpanPredicates(packet), "显式完成的全零摘要应可信")
}

func TestSamplingPipelineDoActionRejectsVersionedEmptyPayload(t *testing.T) {
	t.Parallel()

	pipeline := &SamplingPipeline{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1}
	packet := &DataPacket{PredicateSummaryVersion: CurrentPredicateSummaryVersion}

	matched, kept := pipeline.DoAction(packet)
	assert.False(t, matched)
	assert.Same(t, packet, kept)
	require.Error(t, ComputeSpanPredicates(packet))
}

func TestUnknownPredicateSummaryVersionFallsBackToPayload(t *testing.T) {
	t.Parallel()

	payload := point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "span"})
	pipeline := &SamplingPipeline{
		Name:      "keep-error",
		Type:      PipelineTypeCondition,
		Condition: `{ status = "error" }`,
		Action:    PipelineActionKeep,
	}
	require.NoError(t, pipeline.Apply())

	packet := &DataPacket{
		PointsPayload:           payload,
		PredError:               true,
		PredicateSummaryVersion: CurrentPredicateSummaryVersion + 1,
	}
	matched, _ := pipeline.DoAction(packet)
	assert.False(t, matched, "未知版本摘要不得按当前版本语义直接决策")

	require.NoError(t, ComputeSpanPredicates(packet))
	assert.Equal(t, CurrentPredicateSummaryVersion, packet.PredicateSummaryVersion)
	assert.False(t, packet.PredError, "未知版本摘要应从 payload 重新计算")
}
