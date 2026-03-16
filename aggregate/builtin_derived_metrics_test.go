// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBuiltinDerivedMetrics(t *testing.T) {
	metrics := GetBuiltinDerivedMetrics()

	// 检查是否包含所有内置指标
	assert.NotNil(t, metrics)
	assert.Greater(t, len(metrics), 0)

	// 检查特定指标是否存在
	assert.Contains(t, metrics, "trace_error_count")
	assert.Contains(t, metrics, "trace_total_count")
	assert.Contains(t, metrics, "span_total_count")
	assert.Contains(t, metrics, "trace_error_rate")
	assert.Contains(t, metrics, "trace_duration_summary")
	assert.Contains(t, metrics, "slow_trace_count")
	assert.Contains(t, metrics, "span_error_count")
	assert.Contains(t, metrics, "trace_size_distribution")
	assert.Contains(t, metrics, "logging_total_count")
	assert.Contains(t, metrics, "logging_error_count")
	assert.Contains(t, metrics, "rum_total_count")

	// 检查指标配置
	traceErrorCount := metrics["trace_error_count"]
	assert.Equal(t, "trace_error_count", traceErrorCount.Name)
	assert.Equal(t, "", traceErrorCount.Condition)
	assert.Equal(t, COUNT, traceErrorCount.Algorithm.Method)
	assert.Equal(t, "$error_flag", traceErrorCount.Algorithm.SourceField)

	traceTotalCount := metrics["trace_total_count"]
	assert.Equal(t, "trace_total_count", traceTotalCount.Name)
	assert.Equal(t, "", traceTotalCount.Condition) // 无条件
	assert.Equal(t, COUNT, traceTotalCount.Algorithm.Method)
	assert.Equal(t, "$trace_id", traceTotalCount.Algorithm.SourceField)

	spanTotalCount := metrics["span_total_count"]
	assert.Equal(t, "span_total_count", spanTotalCount.Name)
	assert.Equal(t, "", spanTotalCount.Condition) // 无条件
	assert.Equal(t, COUNT, spanTotalCount.Algorithm.Method)
	assert.Equal(t, "span_id", spanTotalCount.Algorithm.SourceField)

	loggingTotalCount := metrics["logging_total_count"]
	assert.Equal(t, "logging_total_count", loggingTotalCount.Name)
	assert.Equal(t, "", loggingTotalCount.Condition)
	assert.Equal(t, COUNT, loggingTotalCount.Algorithm.Method)
	assert.Equal(t, "$trace_id", loggingTotalCount.Algorithm.SourceField)

	loggingErrorCount := metrics["logging_error_count"]
	assert.Equal(t, "logging_error_count", loggingErrorCount.Name)
	assert.Equal(t, `{status="error"}`, loggingErrorCount.Condition)
	assert.Equal(t, COUNT, loggingErrorCount.Algorithm.Method)
	assert.Equal(t, "$trace_id", loggingErrorCount.Algorithm.SourceField)

	rumTotalCount := metrics["rum_total_count"]
	assert.Equal(t, "rum_total_count", rumTotalCount.Name)
	assert.Equal(t, "", rumTotalCount.Condition)
	assert.Equal(t, COUNT, rumTotalCount.Algorithm.Method)
	assert.Equal(t, "$trace_id", rumTotalCount.Algorithm.SourceField)
}

func TestGetTraceBuiltinDerivedMetrics(t *testing.T) {
	traceMetrics := GetTraceBuiltinDerivedMetrics()

	assert.NotNil(t, traceMetrics)
	assert.Greater(t, len(traceMetrics), 0)

	// 检查是否包含trace相关指标
	foundTraceErrorCount := false
	foundTraceTotalCount := false
	foundTraceDurationSummary := false

	for _, metric := range traceMetrics {
		if metric.Name == "trace_error_count" {
			foundTraceErrorCount = true
		}
		if metric.Name == "trace_total_count" {
			foundTraceTotalCount = true
		}
		if metric.Name == "trace_duration_summary" {
			foundTraceDurationSummary = true
		}
	}

	assert.True(t, foundTraceErrorCount, "应该包含trace_error_count")
	assert.True(t, foundTraceTotalCount, "应该包含trace_total_count")
	assert.True(t, foundTraceDurationSummary, "应该包含trace_duration_summary")
}

func TestGetSpanBuiltinDerivedMetrics(t *testing.T) {
	spanMetrics := GetSpanBuiltinDerivedMetrics()

	assert.NotNil(t, spanMetrics)
	assert.Greater(t, len(spanMetrics), 0)

	// 检查是否包含span相关指标
	foundSpanTotalCount := false
	foundSpanErrorCount := false

	for _, metric := range spanMetrics {
		if metric.Name == "span_total_count" {
			foundSpanTotalCount = true
		}
		if metric.Name == "span_error_count" {
			foundSpanErrorCount = true
		}
	}

	assert.True(t, foundSpanTotalCount, "应该包含span_total_count")
	assert.True(t, foundSpanErrorCount, "应该包含span_error_count")
}

func TestIsBuiltinMetric(t *testing.T) {
	// 测试内置指标
	assert.True(t, IsBuiltinMetric("trace_error_count"))
	assert.True(t, IsBuiltinMetric("trace_total_count"))
	assert.True(t, IsBuiltinMetric("span_total_count"))
	assert.True(t, IsBuiltinMetric("trace_error_rate"))
	assert.True(t, IsBuiltinMetric("trace_duration_summary"))
	assert.True(t, IsBuiltinMetric("slow_trace_count"))
	assert.True(t, IsBuiltinMetric("span_error_count"))
	assert.True(t, IsBuiltinMetric("trace_size_distribution"))
	assert.True(t, IsBuiltinMetric("logging_total_count"))
	assert.True(t, IsBuiltinMetric("logging_error_count"))
	assert.True(t, IsBuiltinMetric("rum_total_count"))

	// 测试非内置指标
	assert.False(t, IsBuiltinMetric("custom_metric"))
	assert.False(t, IsBuiltinMetric("user_defined_metric"))
	assert.False(t, IsBuiltinMetric(""))
}

func TestGetBuiltinMetric(t *testing.T) {
	// 测试获取存在的内置指标
	metric := GetBuiltinMetric("trace_error_count")
	assert.NotNil(t, metric)
	assert.Equal(t, "trace_error_count", metric.Name)
	assert.Equal(t, "", metric.Condition)

	metric = GetBuiltinMetric("trace_total_count")
	assert.NotNil(t, metric)
	assert.Equal(t, "trace_total_count", metric.Name)
	assert.Equal(t, "", metric.Condition)

	metric = GetBuiltinMetric("trace_duration_summary")
	assert.NotNil(t, metric)
	assert.Equal(t, "trace_duration_summary", metric.Name)
	assert.Equal(t, QUANTILES, metric.Algorithm.Method)

	metric = GetBuiltinMetric("logging_total_count")
	assert.NotNil(t, metric)
	assert.Equal(t, "logging_total_count", metric.Name)

	metric = GetBuiltinMetric("logging_error_count")
	assert.NotNil(t, metric)
	assert.Equal(t, "logging_error_count", metric.Name)

	metric = GetBuiltinMetric("rum_total_count")
	assert.NotNil(t, metric)
	assert.Equal(t, "rum_total_count", metric.Name)

	// 测试获取不存在的指标
	metric = GetBuiltinMetric("non_existent_metric")
	assert.Nil(t, metric)

	metric = GetBuiltinMetric("")
	assert.Nil(t, metric)
}

func TestBuiltinMetricConfigurations(t *testing.T) {
	// 测试TraceErrorRate配置
	traceErrorRate := GetBuiltinMetric("trace_error_rate")
	assert.NotNil(t, traceErrorRate)
	assert.Equal(t, AVG, traceErrorRate.Algorithm.Method)
	assert.Equal(t, "$error_flag", traceErrorRate.Algorithm.SourceField)

	// 测试SlowTraceCount配置
	slowTraceCount := GetBuiltinMetric("slow_trace_count")
	assert.NotNil(t, slowTraceCount)
	assert.Equal(t, `{$trace_duration > 5000000}`, slowTraceCount.Condition)
	assert.Equal(t, COUNT, slowTraceCount.Algorithm.Method)

	// 测试TraceDurationSummary配置
	traceDurationSummary := GetBuiltinMetric("trace_duration_summary")
	assert.NotNil(t, traceDurationSummary)
	assert.Equal(t, QUANTILES, traceDurationSummary.Algorithm.Method)

	quantileOpts, ok := traceDurationSummary.Algorithm.Options.(*AggregationAlgo_QuantileOpts)
	assert.True(t, ok)
	assert.NotNil(t, quantileOpts.QuantileOpts)
	assert.Equal(t, []float64{0.5, 0.75, 0.90, 0.95, 0.99}, quantileOpts.QuantileOpts.Percentiles)

	// 测试TraceSizeDistribution配置
	traceSizeDistribution := GetBuiltinMetric("trace_size_distribution")
	assert.NotNil(t, traceSizeDistribution)
	assert.Equal(t, HISTOGRAM, traceSizeDistribution.Algorithm.Method)

	histogramOpts, ok := traceSizeDistribution.Algorithm.Options.(*AggregationAlgo_HistogramOpts)
	assert.True(t, ok)
	assert.NotNil(t, histogramOpts.HistogramOpts)
	assert.Equal(t, []float64{1, 5, 10, 20, 50, 100, 200, 500}, histogramOpts.HistogramOpts.Buckets)
}

func TestBuiltinMetricGroupBy(t *testing.T) {
	// 测试分组标签配置
	testCases := []struct {
		metricName      string
		expectedGroupBy []string
	}{
		{
			metricName:      "trace_error_count",
			expectedGroupBy: []string{"service", "resource", "status"},
		},
		{
			metricName:      "trace_total_count",
			expectedGroupBy: []string{"service", "resource"},
		},
		{
			metricName:      "span_total_count",
			expectedGroupBy: []string{"service", "resource", "span_name"},
		},
		{
			metricName:      "span_error_count",
			expectedGroupBy: []string{"service", "resource", "span_name", "span_kind"},
		},
		{
			metricName:      "trace_error_rate",
			expectedGroupBy: []string{"service", "resource"},
		},
		{
			metricName:      "trace_duration_summary",
			expectedGroupBy: []string{"service", "resource"},
		},
		{
			metricName:      "slow_trace_count",
			expectedGroupBy: []string{"service", "resource"},
		},
		{
			metricName:      "trace_size_distribution",
			expectedGroupBy: []string{"service", "resource"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.metricName, func(t *testing.T) {
			metric := GetBuiltinMetric(tc.metricName)
			assert.NotNil(t, metric)
			assert.Equal(t, tc.expectedGroupBy, metric.Groupby)
		})
	}
}
