package aggregate

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

type (
	algoSumDelta struct {
		delta          float64
		maxTime, count int64
	}
	algoSumAccum struct {
		// TODO
	}
	algoAvg struct {
		sum   float64
		count int64
	}
	algoCount struct {
		count int64
	}
	algoMin struct {
		min float64
	}
	algoMax struct {
		max float64
	}
	algoHistogram struct {
		min, max, sum float64
		count         int64
		bounds        []float64
		buckets       []uint64
	}

	explicitBounds struct {
		index  int64
		cnt    uint64
		lb, ub float64
		pos    bool
	}

	algoExpoHistogram struct {
		min, max, sum    float64
		zeroCount, count int64
		scale            int
		maxTime, minTime int64
		negBucketCounts,
		posBucketCounts []uint64
		bounds []*explicitBounds
	}

	algoStdev struct {
		// TODO
	}

	algoQuantiles struct {
		// TODO
	}

	algoCountDistinct struct {
		// TODO
	}
	algoCountLast struct {
		// TODO
	}
	algoCountFirst struct {
		// TODO
	}
)
