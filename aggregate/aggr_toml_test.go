package aggregate

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"
)

func TestAggregatorConfigureDecodeFile(t *testing.T) {
	path := filepath.Join("testdata", "aggr.toml")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg AggregatorConfigure
	md, err := toml.Decode(string(data), &cfg)
	require.NoError(t, err)
	require.NotNil(t, md)

	require.Equal(t, 15*time.Second, cfg.DefaultWindow)
	require.Equal(t, Action(""), cfg.DefaultAction)
	require.False(t, cfg.DeleteRulesPoint)
	require.Len(t, cfg.AggregateRules, 6)

	rule0 := cfg.AggregateRules[0]
	require.Equal(t, "otel-jvm-memory", rule0.Name)
	require.Equal(t, []string{"service_name", "id"}, rule0.Groupby)
	require.Equal(t, "metric", rule0.Selector.Category)
	require.Equal(t, []string{"otel_service"}, rule0.Selector.Measurements)
	require.Equal(t, []string{"jvm.buffer.memory.used"}, rule0.Selector.MetricName)
	require.Len(t, rule0.Algorithms, 1)
	require.Equal(t, "max", rule0.Algorithms["jvm.buffer.memory.used.max"].Method)
	require.Equal(t, "jvm.buffer.memory.used", rule0.Algorithms["jvm.buffer.memory.used.max"].SourceField)
	require.Equal(t, map[string]string{"method": "max"}, rule0.Algorithms["jvm.buffer.memory.used.max"].AddTags)

	rule1 := cfg.AggregateRules[1]
	require.Equal(t, "trace_root_span_count", rule1.Name)
	require.Equal(t, []string{"service", "resource"}, rule1.Groupby)
	require.Equal(t, "tracing", rule1.Selector.Category)
	require.Equal(t, `{ parent_id = "0" }`, rule1.Selector.Condition)
	require.Equal(t, "count", rule1.Algorithms["root_span.count"].Method)
	require.Equal(t, "span_id", rule1.Algorithms["root_span.count"].SourceField)
	require.Equal(t, map[string]string{"metric": "root_span_count"}, rule1.Algorithms["root_span.count"].AddTags)

	rule5 := cfg.AggregateRules[5]
	require.Equal(t, "otel_jvm_threads_live_sum", rule5.Name)
	require.Equal(t, "sum", rule5.Algorithms["jvm_threads_live_sum"].Method)
	require.Equal(t, "jvm.threads.live", rule5.Algorithms["jvm_threads_live_sum"].SourceField)

	require.NoError(t, cfg.Setup())
}
