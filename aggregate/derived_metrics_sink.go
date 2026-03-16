package aggregate

import "errors"

const DefaultDerivedMetricWindowSeconds int64 = 15

var errNilAggregateCache = errors.New("nil aggregate cache")

// DerivedMetricsSink bridges derived metric aggregation batches back into the aggregation path.
type DerivedMetricsSink interface {
	ConsumeDerivedMetrics(token string, batchs []*AggregationBatch) error
}

// AggregateDerivedMetricsSink routes derived metric batches directly into the aggregate cache.
type AggregateDerivedMetricsSink struct {
	cache *Cache
}

func NewAggregateDerivedMetricsSink(cache *Cache) *AggregateDerivedMetricsSink {
	return &AggregateDerivedMetricsSink{
		cache: cache,
	}
}

func (s *AggregateDerivedMetricsSink) ConsumeDerivedMetrics(token string, batchs []*AggregationBatch) error {
	if len(batchs) == 0 {
		return nil
	}
	if s.cache == nil {
		return errNilAggregateCache
	}

	s.cache.AddBatchs(token, batchs)
	return nil
}
