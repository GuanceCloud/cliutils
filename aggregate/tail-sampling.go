package aggregate

import "time"

type TailSampling struct {
	TraceTTL       time.Duration    `toml:"trace_ttl" json:"trace_ttl"`
	DerivedMetrics []*DerivedMetric `toml:"derived_metrics" json:"derived_metrics"`
}

type DerivedMetric struct {
	Name      string     `toml:"name" json:"name"`
	Aggregate *Aggregate `toml:"aggregate" json:"aggregate"`
	Condition string     `toml:"condition" json:"condition"`
	Groupby   []string   `toml:"group_by" json:"group_by"`
}

type (
	PipelineType   string
	PipelineAction string
)

const (
	PipelineTypeCondition = "condition"
	PipelineTypeSampling  = "probabilistic"

	PipelineActionKeep = "keep"
	PipelineActionDrop = "drop"
)

type SamplingPipeline struct {
	Name      string       `toml:"name" json:"name"`
	Type      PipelineType `toml:"type" json:"type"`
	Condition string       `toml:"condition,omitempty" json:"condition,omitempty"`

	Action PipelineAction `toml:"action,omitempty" json:"action,omitempty"`

	// for sampling
	Rate     float64  `toml:"rate,omitempty" json:"rate,omitempty"`
	HashKeys []string `toml:"hash_keys" json:"hash_keys"`
}

var (
	// predefined pipeline
	SlowTracePipeline = &SamplingPipeline{
		Name:      "slow_trace",
		Type:      PipelineTypeCondition,
		Condition: `{ $trace_duration > 5000000 }`, // larger than 5s, user can override this value
		Action:    PipelineActionKeep,
	}

	NoiseTracePipeline = &SamplingPipeline{
		Name:      "noise_trace",
		Type:      PipelineTypeCondition,
		Condition: `{ resource IN ["/healthz", "/ping"] }`, // user can override these resouce values
		Action:    PipelineActionDrop,
	}

	TailSamplingPipeline = &SamplingPipeline{
		Name:     "sampling",
		Type:     PipelineTypeSampling,
		Rate:     0.1,                  // user can override this rate
		HashKeys: []string{"trace_id"}, // always hash on trace ID, user should not override this
	}

	// predefined derived metrics
	TraceDuration = &DerivedMetric{
		Name:      "trace_duration",
		Condition: "",                              // user specific or empty
		Groupby:   []string{"service", "resource"}, // user can add more tag keys here.

		Aggregate: &Aggregate{
			Algorithm:   AlgoHistogram,
			SourceField: "$trace_duration",
			Buckets: []float64{
				10_000,     // 10ms
				50_000,     // 50ms
				100_000,    // 100ms
				500_000,    // 500ms
				1_000_000,  // 1s
				5_000_000,  // 5s
				10_000_000, // 10s
			}, // duration us
		},
	}

	CounterOnTrace = &DerivedMetric{
		Name:      "trace_counter",
		Condition: "",                              // user specific or empty
		Groupby:   []string{"service", "resource"}, // user can add more tag keys here.

		Aggregate: &Aggregate{
			Algorithm:   AlgoCount,
			SourceField: "<USER-SPECIFIED>",
		},
	}

	CounterOnError = &DerivedMetric{
		Name:      "trace_error",
		Condition: `{status="error"}`,
		Groupby:   []string{"service", "resource"}, // user can add more tag keys here.

		Aggregate: &Aggregate{
			Algorithm:   AlgoCount,
			SourceField: "status",
		},
	}
)
