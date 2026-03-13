# 内置派生指标使用示例

本文档展示了如何使用aggregate包中提供的内置派生指标。

## 1. 内置派生指标列表

系统提供了以下内置派生指标：

| 指标名称 | 描述 | 适用数据类型 |
|----------|------|--------------|
| `trace_error_count` | 错误链路个数统计 | Trace |
| `trace_total_count` | 总链路条数统计 | Trace |
| `span_total_count` | 总Span个数统计 | Span |
| `trace_error_rate` | 错误链路率（百分比） | Trace |
| `trace_duration_summary` | 链路耗时统计摘要（P50/P95/P99） | Trace |
| `slow_trace_count` | 慢链路个数统计（>5秒） | Trace |
| `span_error_count` | 错误Span个数统计 | Span |
| `trace_size_distribution` | 链路大小分布（Span数量） | Trace |

## 2. 在配置中使用内置派生指标

### 2.1 基本配置示例

```toml
# 尾采样配置中使用内置派生指标
[trace]
data_ttl = "5m"
version = 1
group_key = "trace_id"

# 使用内置派生指标
[[trace.derived_metrics]]
name = "trace_error_count"
condition = "{error=true}"
group_by = ["service", "resource", "status"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "error"

[[trace.derived_metrics]]
name = "trace_total_count"
condition = ""
group_by = ["service", "resource"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "$trace_id"

# 采样管道配置
[[trace.sampling_pipeline]]
name = "keep_errors"
type = "condition"
condition = "{error=true}"
action = "keep"

[[trace.sampling_pipeline]]
name = "default_sampling"
type = "probabilistic"
rate = 0.1
hash_keys = ["trace_id"]
```

### 2.2 使用代码API

```go
package main

import (
	"fmt"
	"github.com/GuanceCloud/cliutils/aggregate"
)

func main() {
	// 获取所有内置派生指标
	allMetrics := aggregate.GetBuiltinDerivedMetrics()
	fmt.Printf("内置指标数量: %d\n", len(allMetrics))
	
	// 获取特定内置指标
	traceErrorCount := aggregate.GetBuiltinMetric("trace_error_count")
	if traceErrorCount != nil {
		fmt.Printf("指标名称: %s\n", traceErrorCount.Name)
		fmt.Printf("条件: %s\n", traceErrorCount.Condition)
		fmt.Printf("分组标签: %v\n", traceErrorCount.Groupby)
	}
	
	// 检查是否为内置指标
	if aggregate.IsBuiltinMetric("trace_total_count") {
		fmt.Println("trace_total_count 是内置指标")
	}
	
	// 获取trace相关的内置指标
	traceMetrics := aggregate.GetTraceBuiltinDerivedMetrics()
	fmt.Printf("Trace相关指标数量: %d\n", len(traceMetrics))
	
	// 获取span相关的内置指标
	spanMetrics := aggregate.GetSpanBuiltinDerivedMetrics()
	fmt.Printf("Span相关指标数量: %d\n", len(spanMetrics))
}
```

## 3. 配置示例：完整的监控仪表板

### 3.1 错误监控仪表板

```toml
[trace]
data_ttl = "5m"
version = 1
group_key = "trace_id"

# 错误相关指标
[[trace.derived_metrics]]
name = "trace_error_count"
condition = "{error=true}"
group_by = ["service", "resource", "error_type"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "error"

[[trace.derived_metrics]]
name = "span_error_count"
condition = "{error=true}"
group_by = ["service", "resource", "span_name", "error_type"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "error"

[[trace.derived_metrics]]
name = "trace_error_rate"
condition = ""
group_by = ["service", "resource"]

[[trace.derived_metrics.aggregate]]
method = "AVG"
source_field = "$error_flag"
```

### 3.2 性能监控仪表板

```toml
[trace]
data_ttl = "5m"
version = 1
group_key = "trace_id"

# 性能相关指标
[[trace.derived_metrics]]
name = "trace_duration_summary"
condition = ""
group_by = ["service", "resource"]

[[trace.derived_metrics.aggregate]]
method = "QUANTILES"
source_field = "$trace_duration"

[[trace.derived_metrics.aggregate.quantile_opts]]
percentiles = [0.5, 0.75, 0.90, 0.95, 0.99]

[[trace.derived_metrics]]
name = "slow_trace_count"
condition = "{$trace_duration > 5000000}"
group_by = ["service", "resource"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "$trace_duration"

[[trace.derived_metrics]]
name = "trace_size_distribution"
condition = ""
group_by = ["service", "resource"]

[[trace.derived_metrics.aggregate]]
method = "HISTOGRAM"
source_field = "$span_count"

[[trace.derived_metrics.aggregate.histogram_opts]]
buckets = [1, 5, 10, 20, 50, 100, 200, 500]
```

### 3.3 流量监控仪表板

```toml
[trace]
data_ttl = "5m"
version = 1
group_key = "trace_id"

# 流量相关指标
[[trace.derived_metrics]]
name = "trace_total_count"
condition = ""
group_by = ["service", "resource", "http_method"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "$trace_id"

[[trace.derived_metrics]]
name = "span_total_count"
condition = ""
group_by = ["service", "resource", "span_name"]

[[trace.derived_metrics.aggregate]]
method = "COUNT"
source_field = "span_id"
```

## 4. 特殊变量说明

内置派生指标使用以下特殊变量：

| 变量名 | 描述 | 示例 |
|--------|------|------|
| `$trace_duration` | 链路总耗时（微秒） | `{$trace_duration > 5000000}` |
| `$trace_id` | 链路ID（用于计数） | `source_field = "$trace_id"` |
| `$span_count` | 链路中的Span数量 | `source_field = "$span_count"` |
| `$error_flag` | 错误标志（0或1） | `source_field = "$error_flag"` |

## 5. 最佳实践

### 5.1 指标命名规范
- 使用有意义的名称，如 `trace_error_count` 而不是 `error_count`
- 包含数据类型前缀，如 `trace_`、`span_`
- 使用下划线分隔单词

### 5.2 分组标签选择
- 避免使用过多的分组标签，防止指标爆炸
- 根据业务需求选择关键标签，如 `service`、`resource`、`status`
- 对于错误指标，可以添加 `error_type` 标签进行细分

### 5.3 条件过滤优化
- 使用精确的条件过滤减少不必要的计算
- 对于无条件统计，使用空字符串 `""`
- 使用特殊变量进行性能相关的过滤

### 5.4 采样策略配合
- 错误指标通常需要100%采样
- 性能指标可以配合采样策略
- 流量指标可以根据业务重要性设置不同的采样率

## 6. 与现有指标的兼容性

内置派生指标与现有预定义指标完全兼容：

```go
// 现有预定义指标
var (
	TraceDuration = &DerivedMetric{...}  // 链路耗时直方图
	CounterOnTrace = &DerivedMetric{...} // 链路计数器
	CounterOnError = &DerivedMetric{...} // 错误计数器
)

// 内置派生指标扩展了现有功能
// trace_error_count 提供了更详细的错误统计
// trace_total_count 提供了更灵活的总数统计
// trace_duration_summary 提供了百分位数统计
```

## 7. 故障排除

### 7.1 指标未生成
- 检查条件过滤是否正确
- 验证分组标签是否存在
- 确认数据是否匹配条件

### 7.2 指标数量爆炸
- 减少分组标签数量
- 合并相似的标签值
- 使用更粗粒度的分组

### 7.3 性能问题
- 优化条件过滤表达式
- 减少不必要的指标计算
- 调整采样率平衡精度和性能