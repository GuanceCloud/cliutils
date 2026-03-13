// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package aggregate provides data aggregation and tail sampling functionality.
// This file contains built-in derived metrics for tail sampling.
package aggregate

// Built-in derived metrics for tail sampling
// 内置的尾采样派生指标

var (
	// TraceErrorCount 统计错误链路个数
	// 条件：链路中包含错误状态.
	TraceErrorCount = &DerivedMetric{
		Name:      "trace_error_count",
		Condition: `{error=true}`,
		Groupby:   []string{"service", "resource", "status"}, // 按服务、资源和状态分组

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "error", // 使用error字段进行计数
		},
	}

	// TraceTotalCount 统计总链路条数
	// 无条件：统计所有链路.
	TraceTotalCount = &DerivedMetric{
		Name:      "trace_total_count",
		Condition: "",                              // 无条件，统计所有链路
		Groupby:   []string{"service", "resource"}, // 按服务和资源分组

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "$trace_id", // 使用特殊变量$trace_id进行计数
		},
	}

	// SpanTotalCount 统计总Span个数
	// 无条件：统计所有Span.
	SpanTotalCount = &DerivedMetric{
		Name:      "span_total_count",
		Condition: "",                                           // 无条件，统计所有Span
		Groupby:   []string{"service", "resource", "span_name"}, // 按服务、资源和Span名称分组

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "span_id", // 使用span_id字段进行计数
		},
	}

	// TraceErrorRate 计算错误链路率（百分比）
	// 需要结合TraceErrorCount和TraceTotalCount计算.
	TraceErrorRate = &DerivedMetric{
		Name:      "trace_error_rate",
		Condition: "",                              // 无条件，基于所有链路计算
		Groupby:   []string{"service", "resource"}, // 按服务和资源分组

		Algorithm: &AggregationAlgo{
			Method:      AVG,           // 使用平均值算法计算错误率
			SourceField: "$error_flag", // 使用特殊变量$error_flag（0或1）
		},
	}

	// TraceDurationSummary 链路耗时统计摘要
	// 提供P50、P95、P99等百分位数.
	TraceDurationSummary = &DerivedMetric{
		Name:      "trace_duration_summary",
		Condition: "",                              // 无条件，统计所有链路
		Groupby:   []string{"service", "resource"}, // 按服务和资源分组

		Algorithm: &AggregationAlgo{
			Method:      QUANTILES,
			SourceField: "$trace_duration", // 使用特殊变量$trace_duration
			Options: &AggregationAlgo_QuantileOpts{
				QuantileOpts: &QuantileOptions{
					Percentiles: []float64{0.5, 0.75, 0.90, 0.95, 0.99}, // P50, P75, P90, P95, P99
				},
			},
		},
	}

	// SlowTraceCount 统计慢链路个数（耗时超过阈值）
	// 条件：链路耗时超过指定阈值.
	SlowTraceCount = &DerivedMetric{
		Name:      "slow_trace_count",
		Condition: `{$trace_duration > 5000000}`,   // 耗时超过5秒（5000000微秒）
		Groupby:   []string{"service", "resource"}, // 按服务和资源分组

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "$trace_duration", // 使用特殊变量$trace_duration
		},
	}

	// SpanErrorCount 统计错误Span个数
	// 条件：Span中包含错误状态.
	SpanErrorCount = &DerivedMetric{
		Name:      "span_error_count",
		Condition: `{error=true}`,
		Groupby:   []string{"service", "resource", "span_name", "span_kind"}, // 按服务、资源、Span名称和类型分组

		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "error",
		},
	}

	// TraceSizeDistribution 链路大小分布（Span数量）
	// 统计每条链路包含的Span数量分布.
	TraceSizeDistribution = &DerivedMetric{
		Name:      "trace_size_distribution",
		Condition: "",                              // 无条件，统计所有链路
		Groupby:   []string{"service", "resource"}, // 按服务和资源分组

		Algorithm: &AggregationAlgo{
			Method:      HISTOGRAM,
			SourceField: "$span_count", // 使用特殊变量$span_count
			Options: &AggregationAlgo_HistogramOpts{
				HistogramOpts: &HistogramOptions{
					Buckets: []float64{1, 5, 10, 20, 50, 100, 200, 500}, // Span数量桶
				},
			},
		},
	}
)

// GetBuiltinDerivedMetrics 获取所有内置派生指标
// 返回一个映射表，键为指标名称，值为派生指标配置.
func GetBuiltinDerivedMetrics() map[string]*DerivedMetric {
	return map[string]*DerivedMetric{
		"trace_error_count":       TraceErrorCount,
		"trace_total_count":       TraceTotalCount,
		"span_total_count":        SpanTotalCount,
		"trace_error_rate":        TraceErrorRate,
		"trace_duration_summary":  TraceDurationSummary,
		"slow_trace_count":        SlowTraceCount,
		"span_error_count":        SpanErrorCount,
		"trace_size_distribution": TraceSizeDistribution,
	}
}

// GetTraceBuiltinDerivedMetrics 获取链路相关的内置派生指标
// 适用于trace数据类型的预定义指标.
func GetTraceBuiltinDerivedMetrics() []*DerivedMetric {
	return []*DerivedMetric{
		TraceErrorCount,
		TraceTotalCount,
		TraceErrorRate,
		TraceDurationSummary,
		SlowTraceCount,
		TraceSizeDistribution,
	}
}

// GetSpanBuiltinDerivedMetrics 获取Span相关的内置派生指标
// 适用于span级别的预定义指标.
func GetSpanBuiltinDerivedMetrics() []*DerivedMetric {
	return []*DerivedMetric{
		SpanTotalCount,
		SpanErrorCount,
	}
}

// BuiltinMetricNames 内置指标名称列表.
var BuiltinMetricNames = []string{
	"trace_error_count",
	"trace_total_count",
	"span_total_count",
	"trace_error_rate",
	"trace_duration_summary",
	"slow_trace_count",
	"span_error_count",
	"trace_size_distribution",
}

// IsBuiltinMetric 检查是否为内置指标.
func IsBuiltinMetric(name string) bool {
	for _, builtinName := range BuiltinMetricNames {
		if builtinName == name {
			return true
		}
	}
	return false
}

// GetBuiltinMetric 根据名称获取内置指标.
func GetBuiltinMetric(name string) *DerivedMetric {
	metrics := GetBuiltinDerivedMetrics()
	return metrics[name]
}
