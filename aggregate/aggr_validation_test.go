package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"
)

func TestAggregatorConfigureSetupRejectsUnsupportedMethod(t *testing.T) {
	cfg := &AggregatorConfigure{
		DefaultWindow: time.Second * 10,
		AggregateRules: []*AggregateRule{
			{
				Name:    "unsupported",
				Groupby: []string{"service"},
				Selector: &RuleSelector{
					Category:   point.Metric.String(),
					MetricName: []string{"latency"},
				},
				Algorithms: map[string]*AggregationAlgoConfig{
					"latency": {
						Method:      string(EXPO_HISTOGRAM),
						SourceField: "latency",
					},
				},
			},
		},
	}

	err := cfg.Setup()
	require.Error(t, err)
	require.ErrorContains(t, err, `method "expo_histogram" is not supported`)
}

func TestAggregatorConfigureSetupRejectsInvalidQuantiles(t *testing.T) {
	cfg := &AggregatorConfigure{
		DefaultWindow: time.Second * 10,
		AggregateRules: []*AggregateRule{
			{
				Name:    "invalid-quantiles",
				Groupby: []string{"service"},
				Selector: &RuleSelector{
					Category:   point.Metric.String(),
					MetricName: []string{"latency"},
				},
				Algorithms: map[string]*AggregationAlgoConfig{
					"latency": {
						Method:      string(QUANTILES),
						SourceField: "latency",
						QuantileOpts: &QuantileOptions{
							Percentiles: []float64{1.2},
						},
					},
				},
			},
		},
	}

	err := cfg.Setup()
	require.Error(t, err)
	require.ErrorContains(t, err, "out of range [0,1]")
}

func TestAggregatorConfigureSetupConvertsAlgorithmConfigsFromTOML(t *testing.T) {
	const doc = `
	default_window = 10000000000

[[aggregate_rules]]
name = "trace_root_span"
group_by = ["service"]

  [aggregate_rules.select]
  category = "tracing"
  metric_name = ["span_id", "latency"]

  [aggregate_rules.algorithms.root_span_count]
  method = "count"

  [aggregate_rules.algorithms.latency_p95]
  method = "quantiles"
  source_field = "latency"

    [aggregate_rules.algorithms.latency_p95.quantile_opts]
    percentiles = [0.95]
`

	var cfg AggregatorConfigure
	require.NoError(t, toml.Unmarshal([]byte(doc), &cfg))
	require.Len(t, cfg.AggregateRules, 1)
	require.Len(t, cfg.AggregateRules[0].Algorithms, 2)

	require.NoError(t, cfg.Setup())

	rule := cfg.AggregateRules[0]
	require.Len(t, rule.aggregationOpts, 2)

	countAlgo, ok := rule.aggregationOpts["root_span_count"]
	require.True(t, ok)
	require.Equal(t, string(COUNT), countAlgo.Method)
	require.Equal(t, int64(10*time.Second), countAlgo.Window)

	quantileAlgo, ok := rule.aggregationOpts["latency_p95"]
	require.True(t, ok)
	quantileOpts, ok := quantileAlgo.Options.(*AggregationAlgo_QuantileOpts)
	require.True(t, ok)
	require.Equal(t, []float64{0.95}, quantileOpts.QuantileOpts.Percentiles)
}
