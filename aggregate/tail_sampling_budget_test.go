// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalSamplerStateBudgetDropsWholeGroupAndTombstonesIt(t *testing.T) {
	budget := NewStateBudget(StateBudgetConfig{
		Mode: StateBudgetEnforce,
		Kinds: map[StateKind]StateLimit{
			StateKindTailSamplingPayload: {MaxBytesPerReservation: 5},
		},
	})
	sampler := NewGlobalSamplerWithStateBudget(1, time.Second, budget)
	require.NoError(t, sampler.UpdateConfig("token-a", testTraceTailSamplingConfig(time.Second)))

	first := &DataPacket{
		GroupIdHash:   1,
		Token:         "token-a",
		DataType:      point.STracing,
		GroupKey:      "trace_id",
		PointsPayload: []byte("abc"),
	}
	assert.True(t, sampler.IngestWithResult(first).Accepted)

	second := &DataPacket{
		GroupIdHash:   first.GroupIdHash,
		Token:         first.Token,
		DataType:      first.DataType,
		GroupKey:      first.GroupKey,
		PointsPayload: []byte("def"),
	}
	result := sampler.IngestWithResult(second)
	require.NotNil(t, result.Rejection)
	assert.Equal(t, StateBudgetScopeObject, result.Rejection.Scope)
	assert.True(t, result.Tombstoned)
	assert.False(t, result.Accepted)
	assert.Empty(t, sampler.shards[0].activeMap)
	assert.Len(t, sampler.shards[0].tombstones, 1)

	result = sampler.IngestWithResult(second)
	assert.True(t, result.Tombstoned)
	otherGroup := &DataPacket{
		GroupIdHash:   2,
		Token:         first.Token,
		DataType:      first.DataType,
		GroupKey:      first.GroupKey,
		PointsPayload: []byte("12345"),
	}
	assert.True(t, sampler.IngestWithResult(otherGroup).Accepted, "a group ceiling must not consume another group's quota")
	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1, "only the accepted group may reach a sampling decision")
	assert.Equal(t, otherGroup.GroupIdHash, expired[tailSamplingGroupMapKey(otherGroup)].packet.GroupIdHash)

	snapshot := sampler.StateBudgetSnapshot()
	assert.Equal(t, StateCost{}, snapshot.ByKind[StateKindTailSamplingGroup].Cost)
	assert.Equal(t, StateCost{}, snapshot.ByKind[StateKindTailSamplingPayload].Cost)
	assert.Equal(t, StateCost{}, snapshot.ByKind[StateKindTailSamplingTombstone].Cost)
}

func TestGlobalSamplerStateBudgetLimitsGroupsAndConfig(t *testing.T) {
	budget := NewStateBudget(StateBudgetConfig{
		Mode: StateBudgetEnforce,
		Kinds: map[StateKind]StateLimit{
			StateKindTailSamplingConfig: {MaxObjects: 1},
		},
		WorkspaceKindLimit: map[StateKind]StateLimit{
			StateKindTailSamplingGroup: {MaxObjects: 1},
		},
	})
	sampler := NewGlobalSamplerWithStateBudget(1, time.Second, budget)
	require.NoError(t, sampler.UpdateConfig("token-a", testTraceTailSamplingConfig(time.Second)))
	require.NoError(t, sampler.UpdateConfig("token-a", testTraceTailSamplingConfig(2*time.Second)))
	var rejection *StateBudgetError
	require.ErrorAs(t, sampler.UpdateConfig("token-b", testTraceTailSamplingConfig(time.Second)), &rejection)

	first := &DataPacket{GroupIdHash: 1, Token: "token-a", DataType: point.STracing, GroupKey: "trace_id"}
	second := &DataPacket{GroupIdHash: 2, Token: "token-a", DataType: point.STracing, GroupKey: "trace_id"}
	assert.True(t, sampler.IngestWithResult(first).Accepted)
	result := sampler.IngestWithResult(second)
	require.NotNil(t, result.Rejection)
	assert.Equal(t, StateBudgetScopeWorkspaceKind, result.Rejection.Scope)
	assert.True(t, result.Tombstoned)

	snapshot := sampler.StateBudgetSnapshot()
	assert.EqualValues(t, 1, snapshot.ByKind[StateKindTailSamplingGroup].Cost.Objects)
	assert.EqualValues(t, 1, snapshot.ByKind[StateKindTailSamplingTombstone].Cost.Objects)
	sampler.Close()
	assert.Equal(t, StateCost{}, sampler.StateBudgetSnapshot().Total.Cost)
}

func testTraceTailSamplingConfig(ttl time.Duration) *TailSamplingConfigs {
	return &TailSamplingConfigs{
		Version: time.Now().UnixNano(),
		Tracing: &TraceTailSampling{
			DataTTL:  ttl,
			GroupKey: "trace_id",
			Pipelines: []*SamplingPipeline{{
				Name: "keep-all",
				Type: PipelineTypeSampling,
				Rate: 1,
			}},
		},
	}
}
