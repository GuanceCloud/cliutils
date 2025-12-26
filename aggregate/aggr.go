package aggregate

import "time"

type (
	Action    string
	Algorithm string
)

const (
	// actions
	actionPassThrough = "passthrough"
	actionDrop        = "drop"

	// algorithms
	AlgoSumDelta      = "sum_delta"
	AlgoSumAccum      = "sum_accum"
	AlgoAvg           = "avg"
	AlgoCount         = "count"
	AlgoMin           = "min"
	AlgoMax           = "max"
	AlgoHistogram     = "histogram"
	AlgoStdev         = "stdev"
	AlgoQuantiles     = "quantiles"
	AlgoCountDistinct = "count_distinct"
	AlgoCountLast     = "last"
	AlgoCountFirst    = "first"
)

type Aggregator struct {
	DefaultWindow  time.Duration    `toml:"default_window" json:"default_window"`
	AggregateRules []*AggregateRule `toml:"aggregate_rules" json:"aggregate_rules"`
	DefaultAction  Action           `toml:"action" json:"action"`
}

type AggregateRule struct {
	Name string `toml:"name" json:"name"`

	// override default window
	Window time.Duration `toml:"window,omitempty" json:"window,omitempty"`

	Select     *RuleSelect           `toml:"select" json:"select"`
	Groupby    []string              `toml:"group_by" json:"group_by"`
	Aggregates map[string]*Aggregate `toml:"aggregates" json:"aggregates"`
}

// RuleSelect used to select measurements and fields among points.
type RuleSelect struct {
	Category     string   `toml:"category" json:"category"`
	Measurements []string `toml:"measurements" json:"measurements"`
	Fields       []string `toml:"fields" json:"fields"`
	Condition    string   `toml:"conditon" json:"condition"`
}

// Aggregate defines the algorithm used for specific field.
type Aggregate struct {
	Algorithm   Algorithm `toml:"algorithm" json:"algorithm"`
	SourceField string    `toml:"source_field,omitempty" json:"source_field,omitempty"`

	// for histogram
	Buckets []float64 `toml:"buckets" json:"buckets"`
	// for quantiles
	Percentiles []float64 `toml:"percentiles" json:"percentiles"`

	AddTags map[string]string `toml:"add_tags,omitempty" json:"add_tags,omitempty"`
}
