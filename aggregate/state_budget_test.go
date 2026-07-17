// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateBudgetEnforcesAndReleasesAllScopes(t *testing.T) {
	budget := NewStateBudget(StateBudgetConfig{
		Mode:      StateBudgetEnforce,
		Process:   StateLimit{MaxBytes: 100, MaxObjects: 3},
		Workspace: StateLimit{MaxBytes: 100, MaxObjects: 2},
		Kinds: map[StateKind]StateLimit{
			StateKindTailSamplingGroup: {MaxObjects: 1},
		},
		WorkspaceKindLimit: map[StateKind]StateLimit{
			StateKindTailSamplingPayload: {MaxBytes: 60},
		},
	})

	group, err := budget.Reserve(StateReservation{
		Workspace: "workspace-a",
		Kind:      StateKindTailSamplingGroup,
		Cost:      StateCost{Bytes: 20, Objects: 1},
	})
	require.Nil(t, err)
	require.NotNil(t, group)

	_, err = budget.Reserve(StateReservation{
		Workspace: "workspace-a",
		Kind:      StateKindTailSamplingGroup,
		Cost:      StateCost{Bytes: 1, Objects: 1},
	})
	require.NotNil(t, err)
	assert.Equal(t, StateBudgetScopeKind, err.Scope)

	payload, err := budget.Reserve(StateReservation{
		Workspace: "workspace-a",
		Kind:      StateKindTailSamplingPayload,
		Cost:      StateCost{Bytes: 40, Objects: 1},
	})
	require.Nil(t, err)

	err = budget.Resize(payload, StateCost{Bytes: 61, Objects: 1})
	require.NotNil(t, err)
	assert.Equal(t, StateBudgetScopeWorkspaceKind, err.Scope)

	snapshot := budget.Snapshot()
	assert.Equal(t, StateCost{Bytes: 60, Objects: 2}, snapshot.Total.Cost)
	assert.EqualValues(t, 2, snapshot.Total.Rejected)

	budget.Release(group)
	budget.Release(payload)
	snapshot = budget.Snapshot()
	assert.Equal(t, StateCost{}, snapshot.Total.Cost)
}

func TestStateBudgetObserveRecordsWouldRejectAndKeepsUsage(t *testing.T) {
	budget := NewStateBudget(StateBudgetConfig{
		Mode:    StateBudgetObserve,
		Process: StateLimit{MaxBytes: 10},
	})

	lease, err := budget.Reserve(StateReservation{
		Workspace: "workspace-a",
		Kind:      StateKindAggregationWindow,
		Cost:      StateCost{Bytes: 11, Objects: 1},
	})
	require.Nil(t, err)
	require.NotNil(t, lease)

	snapshot := budget.Snapshot()
	assert.Equal(t, StateCost{Bytes: 11, Objects: 1}, snapshot.Total.Cost)
	assert.EqualValues(t, 1, snapshot.Total.WouldRejected)
	assert.EqualValues(t, 1, snapshot.ByKind[StateKindAggregationWindow].WouldRejected)
}

func TestStateBudgetResizeReleasedLease(t *testing.T) {
	budget := NewStateBudget(StateBudgetConfig{})
	lease, err := budget.Reserve(StateReservation{Kind: StateKindAggregationCalculator, Cost: StateCost{Objects: 1}})
	require.Nil(t, err)
	budget.Release(lease)

	err = budget.Resize(lease, StateCost{Objects: 2})
	require.NotNil(t, err)
	assert.Equal(t, StateBudgetScopeReleasedLease, err.Scope)
}
