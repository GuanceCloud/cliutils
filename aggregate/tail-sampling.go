package aggregate

import (
	fp "github.com/GuanceCloud/cliutils/filter"
	"github.com/GuanceCloud/cliutils/point"
	"math"
	"time"
)

type TailSampling struct {
	TraceTTL       time.Duration       `toml:"trace_ttl" json:"trace_ttl"`
	DerivedMetrics []*DerivedMetric    `toml:"derived_metrics" json:"derived_metrics"`
	Pipelines      []*SamplingPipeline `toml:"sampling_pipeline" json:"pipelines"`
	Version        int64               `toml:"version" json:"version"`
}

func (ts *TailSampling) Init() {
	if ts.TraceTTL == 0 {
		ts.TraceTTL = 5 * time.Minute
	}
	for _, pipeline := range ts.Pipelines {
		if err := pipeline.Apply(); err != nil {
			l.Errorf("failed to apply sampling pipeline: %s", err)
		}
	}
}

type DerivedMetric struct {
	Name      string           `toml:"name" json:"name"`
	Algorithm *AggregationAlgo `toml:"aggregate" json:"aggregate"`
	Condition string           `toml:"condition" json:"condition"`
	Groupby   []string         `toml:"group_by" json:"group_by"`
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
	sampleRange        = 10000
)

type SamplingPipeline struct {
	Name      string         `toml:"name" json:"name"`
	Type      PipelineType   `toml:"type" json:"type"`
	Condition string         `toml:"condition,omitempty" json:"condition,omitempty"`
	Action    PipelineAction `toml:"action,omitempty" json:"action,omitempty"`
	Rate      float64        `toml:"rate,omitempty" json:"rate,omitempty"`
	HashKeys  []string       `toml:"hash_keys" json:"hash_keys"`

	conds fp.WhereConditions
}

func (sp *SamplingPipeline) Apply() error {
	if ast, err := fp.GetConds(sp.Condition); err != nil {
		return err
	} else {
		sp.conds = ast
		return nil
	}
}

func (sp *SamplingPipeline) DoAction(td *TraceDataPacket) (bool, *TraceDataPacket) {
	ptw := &ptWrap{}
	if len(sp.HashKeys) > 0 {
		for _, key := range sp.HashKeys {
			for _, span := range td.Spans {
				ptw.Point = point.FromPB(span)
				if _, has := ptw.Get(key); has {
					return true, td
				}
			}
		}
	}
	if sp.conds == nil { // condition are required to do actions.
		return false, td
	}

	matched := false

	for _, span := range td.Spans {
		ptw.Point = point.FromPB(span)
		if x := sp.conds.Eval(ptw); x < 0 {
			continue
		} // else: matched, fall through...

		matched = true
		//r.mached++
		if sp.Type == PipelineTypeSampling {
			if sp.Rate > 0.0 {
				if td.TraceIdHash%sampleRange < uint64(math.Floor(sp.Rate*float64(sampleRange))) {
					return true, td // keep
				} else {
					// sp.dropped++
					return true, nil
				}
			}
		}

		if sp.Type == PipelineTypeCondition {
			// check on action
			switch sp.Action {
			case PipelineActionDrop:
				// r.dropped++
				return true, nil
			case PipelineActionKeep:
				return true, td
			}
		}
	}

	return matched, td
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

		Algorithm: &AggregationAlgo{
			Method:      HISTOGRAM,
			SourceField: "$trace_duration",
			Options: &AggregationAlgo_HistogramOpts{
				HistogramOpts: &HistogramOptions{
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
			},
		},
	}

	CounterOnTrace = &DerivedMetric{
		Name:      "trace_counter",
		Condition: "",                              // user specific or empty
		Groupby:   []string{"service", "resource"}, // user can add more tag keys here.

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "<USER-SPECIFIED>",
		},
	}

	CounterOnError = &DerivedMetric{
		Name:      "trace_error",
		Condition: `{status="error"}`,
		Groupby:   []string{"service", "resource"}, // user can add more tag keys here.

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "status",
		},
	}
)
