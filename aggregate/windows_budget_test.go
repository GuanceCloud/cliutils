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

func TestCacheStateBudgetDropsWholeAggregationWindow(t *testing.T) {
	budget := NewStateBudget(StateBudgetConfig{
		Mode: StateBudgetEnforce,
		WorkspaceKindLimit: map[StateKind]StateLimit{
			StateKindAggregationCalculator: {MaxObjects: 1},
		},
	})
	cache := NewCacheWithOptions(time.Second, CacheOptions{StateBudget: budget})

	first := newAggregationBudgetBatch("latency", SUM, 10)
	second := newAggregationBudgetBatch("count", SUM, 1)
	assert.Equal(t, 1, cache.AddBatchWithResult("token-a", first).Accepted)
	result := cache.AddBatchWithResult("token-a", second)
	require.NotNil(t, result.Rejection)
	assert.Equal(t, 1, result.Dropped)
	assert.Zero(t, result.Accepted)

	for _, bucket := range cache.WindowsBuckets {
		data := WindowsToData(bucket.Close())
		assert.Empty(t, data, "a resource-rejected window must not emit partial aggregation output")
	}
	snapshot := cache.StateBudgetSnapshot()
	assert.Equal(t, StateCost{}, snapshot.Total.Cost)
}

func TestCacheStateBudgetDegradesQuantilePrecisionWithoutDroppingWindow(t *testing.T) {
	first := newAggregationBudgetBatch("latency", QUANTILES, 10)
	first.AggregationOpts["latency"].Options = &AggregationAlgo_QuantileOpts{
		QuantileOpts: &QuantileOptions{Percentiles: []float64{0.5}},
	}
	calculator := newCalculators(first)[0]
	calculatorLimit := estimateCalculatorCost(calculator).Bytes
	budget := NewStateBudget(StateBudgetConfig{
		Mode: StateBudgetEnforce,
		Kinds: map[StateKind]StateLimit{
			StateKindAggregationCalculator: {MaxBytes: calculatorLimit},
		},
	})
	cache := NewCacheWithOptions(time.Second, CacheOptions{StateBudget: budget})

	result := cache.AddBatchWithResult("token-a", first)
	assert.Equal(t, 1, result.Accepted)
	second := newAggregationBudgetBatch("latency", QUANTILES, 20)
	second.AggregationOpts["latency"].Options = first.AggregationOpts["latency"].Options
	result = cache.AddBatchWithResult("token-a", second)
	require.NotNil(t, result.Rejection)
	assert.Equal(t, 1, result.Accepted)
	assert.Equal(t, 1, result.PrecisionDegraded)
	assert.Zero(t, result.Dropped)

	for _, bucket := range cache.WindowsBuckets {
		data := WindowsToData(bucket.Close())
		require.Len(t, data, 1)
		require.Len(t, data[0].PTS, 1)
		count, ok := data[0].PTS[0].GetI("latency_count")
		require.True(t, ok)
		assert.EqualValues(t, 2, count)
	}
	assert.Equal(t, StateCost{}, cache.StateBudgetSnapshot().Total.Cost)
}

func TestCacheStateBudgetRejectsOversizedWindowSpan(t *testing.T) {
	cache := NewCacheWithOptions(time.Second, CacheOptions{MaxWindowSpan: time.Minute})
	result := cache.AddBatchWithResult("token-a", newAggregationBudgetBatch("latency", SUM, 10))
	assert.Equal(t, 1, result.WindowSpanExceeded)
	assert.Empty(t, cache.WindowsBuckets)
}

func newAggregationBudgetBatch(key string, method AlgoMethod, value float64) *AggregationBatch {
	pt := point.NewPoint(
		"request",
		point.KVs{}.Add(key, value),
		point.WithTime(time.Now().Truncate(time.Second)),
		point.WithPrecheck(false),
	)
	return &AggregationBatch{
		RoutingKey: 1,
		Points:     &point.PBPoints{Arr: []*point.PBPoint{pt.PBPoint()}},
		AggregationOpts: map[string]*AggregationAlgo{
			key: {Method: string(method), Window: int64(time.Hour)},
		},
	}
}
