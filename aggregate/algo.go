package aggregate

import "github.com/GuanceCloud/cliutils/point"

const (
	// algorithms
	AlgoSumDelta      = "sum_delta"
	AlgoSumAccum      = "sum_accum"
	AlgoAvg           = "avg"
	AlgoCount         = "count"
	AlgoMin           = "min"
	AlgoMax           = "max"
	AlgoHistogram     = "histogram"
	AlgoExpoHistogram = "histogram_exponential"
	AlgoStdev         = "stdev"
	AlgoQuantiles     = "quantiles"
	AlgoCountDistinct = "count_distinct"
	AlgoCountLast     = "last"
	AlgoCountFirst    = "first"
)

type Algorithm string

// aggregateAlgoConfigure defines the algorithm used for specific field.
type aggregateAlgoConfigure struct {
	Algorithm Algorithm `toml:"algorithm" json:"algorithm"`

	// source fields used for current algorithm
	SourceField string `toml:"source_field,omitempty" json:"source_field,omitempty"`

	// for AlgoHistogram
	Buckets []float64 `toml:"buckets" json:"buckets"`

	// for AlgoExpoHistogram
	MaxScale     int  `toml:"max_scale" json:"max_scale"`
	MaxBucket    int  `toml:"max_buckets" json:"max_buckets"`
	RecordMinMax bool `toml:"record_min_max" json:"record_min_max"`

	// for quantiles
	Percentiles []float64 `toml:"percentiles" json:"percentiles"`

	AddTags map[string]string `toml:"add_tags,omitempty" json:"add_tags,omitempty"`
}

type Calculator interface {
	addNewPoints(pt []*point.Point)
	aggr() ([]*point.Point, error)
	reset()
}

type metricBase struct {
	aggrTags [][2]string // hash tags
	key,
	name string
	hash uint64
}

func (c *algoSumDelta) addNewPoints(pts []*point.Point) {
	for _, pt := range pts {
		c.count++
		if v, ok := pt.GetF(c.key); ok {
			c.delta += v
		}

		if x := pt.Time().UnixNano(); x > c.maxTime {
			c.maxTime = x
		}
	}
}

func (c *algoSumDelta) aggr() ([]*point.Point, error) {
	var kvs point.KVs
	kvs = kvs.Add(c.key, c.delta)
	for _, kv := range c.aggrTags {
		kvs = kvs.SetTag(kv[0], kv[1])
	}

	return []*point.Point{
		point.NewPoint(c.name, kvs, point.WithTimestamp(c.maxTime)),
	}, nil
}

func (c *algoSumDelta) reset() {
	c.delta = 0
	c.maxTime = 0
	c.count = 0
}

type (
	algoSumDelta struct {
		metricBase
		delta          float64
		maxTime, count int64
	}

	algoSumAccum struct {
		metricBase
		// TODO
	}
	algoAvg struct {
		metricBase
		sum   float64
		count int64
	}
	algoCount struct {
		metricBase
		count int64
	}
	algoMin struct {
		metricBase
		min float64
	}
	algoMax struct {
		metricBase
		max float64
	}
	algoHistogram struct {
		metricBase
		min, max, sum float64
		count         int64
		bounds        []float64
		buckets       []uint64
	}

	explicitBounds struct {
		metricBase
		index  int64
		cnt    uint64
		lb, ub float64
		pos    bool
	}

	algoExpoHistogram struct {
		metricBase
		min, max, sum    float64
		zeroCount, count int64
		scale            int
		maxTime, minTime int64
		negBucketCounts,
		posBucketCounts []uint64
		bounds []*explicitBounds
	}

	algoStdev struct {
		metricBase
		// TODO
	}

	algoQuantiles struct {
		metricBase
		// TODO
	}

	algoCountDistinct struct {
		metricBase
		// TODO
	}
	algoCountLast struct {
		metricBase
		// TODO
	}
	algoCountFirst struct {
		metricBase
		// TODO
	}
)
