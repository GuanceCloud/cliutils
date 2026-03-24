package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
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
				Algorithms: map[string]*AggregationAlgo{
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
				Algorithms: map[string]*AggregationAlgo{
					"latency": {
						Method:      string(QUANTILES),
						SourceField: "latency",
						Options: &AggregationAlgo_QuantileOpts{
							QuantileOpts: &QuantileOptions{
								Percentiles: []float64{1.2},
							},
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
