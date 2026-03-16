# 内置派生指标使用示例

本文档说明如何在尾采样配置中显式开启内置派生指标，以及这些指标如何回到指标聚合模块形成闭环。

## 1. 当前模型

当前尾采样阶段支持的是：

- 通过 `builtin_derived_metrics` 显式开启内置派生指标
- 启用后的指标会在尾采样数据到期时生成 point
- 运行时会将内置派生指标直接构造成 `AggregationBatch` 并通过 sink 写入 `Cache`

当前暂不支持的是：

- 用户自定义 `derived_metrics` 在尾采样运行时生效

也就是说，现阶段“能配置并生效”的是 builtin，不是自定义指标。

## 2. 内置派生指标列表

系统当前提供以下内置派生指标：

| 指标名称 | 描述 | 适用数据类型 |
|----------|------|--------------|
| `trace_error_count` | 错误链路个数统计 | Trace |
| `trace_total_count` | 总链路条数统计 | Trace |
| `span_total_count` | 总Span个数统计 | Trace |
| `trace_error_rate` | 错误链路率 | Trace |
| `trace_duration_summary` | 链路耗时分位数摘要 | Trace |
| `slow_trace_count` | 慢链路个数统计 | Trace |
| `span_error_count` | 错误Span个数统计 | Trace |
| `trace_size_distribution` | 链路大小分布 | Trace |
| `logging_total_count` | 日志分组总数统计 | Logging |
| `logging_error_count` | 错误日志分组总数统计 | Logging |
| `rum_total_count` | RUM分组总数统计 | RUM |

注意：

- builtin 指标虽然定义在同一个文件里，但运行时会按 data type 过滤
- trace/logging/RUM 只能启用各自支持的指标名

## 3. 配置方式

### 3.1 Trace

Trace 的内置派生指标配置在 `trace.builtin_derived_metrics` 下：

```toml
[trace]
data_ttl = "5m"
version = 1
group_key = "trace_id"

[[trace.builtin_derived_metrics]]
name = "trace_total_count"
enabled = true

[[trace.builtin_derived_metrics]]
name = "trace_error_count"
enabled = true

[[trace.builtin_derived_metrics]]
name = "trace_duration_summary"
enabled = true

[[trace.sampling_pipeline]]
name = "keep_errors"
type = "condition"
condition = "{error=true}"
action = "keep"

[[trace.sampling_pipeline]]
name = "default_sampling"
type = "probabilistic"
condition = "{ 1 = 1 }"
rate = 0.1
hash_keys = ["trace_id"]
```

### 3.2 Logging

Logging 的内置派生指标挂在具体 `group_dimensions` 下：

```toml
[logging]
data_ttl = "1m"
version = 1

[[logging.group_dimensions]]
group_key = "user_id"

[[logging.group_dimensions.pipelines]]
name = "sample_user_logs"
type = "probabilistic"
condition = "{ 1 = 1 }"
rate = 0.1

[[logging.group_dimensions.builtin_derived_metrics]]
name = "logging_total_count"
enabled = true

[[logging.group_dimensions.builtin_derived_metrics]]
name = "logging_error_count"
enabled = true
```

### 3.3 RUM

RUM 的内置派生指标同样挂在 `group_dimensions` 下：

```toml
[rum]
data_ttl = "1m"
version = 1

[[rum.group_dimensions]]
group_key = "session_id"

[[rum.group_dimensions.pipelines]]
name = "sample_sessions"
type = "probabilistic"
condition = "{ 1 = 1 }"
rate = 0.1

[[rum.group_dimensions.builtin_derived_metrics]]
name = "rum_total_count"
enabled = true
```

## 4. 常见启用组合

### 4.1 Trace 错误与流量监控

```toml
[[trace.builtin_derived_metrics]]
name = "trace_total_count"
enabled = true

[[trace.builtin_derived_metrics]]
name = "trace_error_count"
enabled = true

[[trace.builtin_derived_metrics]]
name = "trace_error_rate"
enabled = true
```

### 4.2 Trace 性能监控

```toml
[[trace.builtin_derived_metrics]]
name = "trace_duration_summary"
enabled = true

[[trace.builtin_derived_metrics]]
name = "slow_trace_count"
enabled = true

[[trace.builtin_derived_metrics]]
name = "trace_size_distribution"
enabled = true
```

### 4.3 Logging 流量与错误监控

```toml
[[logging.group_dimensions.builtin_derived_metrics]]
name = "logging_total_count"
enabled = true

[[logging.group_dimensions.builtin_derived_metrics]]
name = "logging_error_count"
enabled = true
```

### 4.4 RUM 流量监控

```toml
[[rum.group_dimensions.builtin_derived_metrics]]
name = "rum_total_count"
enabled = true
```

## 5. 代码 API

可以通过 API 枚举和查询内置指标：

```go
package main

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/aggregate"
)

func main() {
	allMetrics := aggregate.GetBuiltinDerivedMetrics()
	fmt.Printf("内置指标数量: %d\n", len(allMetrics))

	traceMetric := aggregate.GetBuiltinMetric("trace_error_count")
	if traceMetric != nil {
		fmt.Printf("name=%s group_by=%v\n", traceMetric.Name, traceMetric.Groupby)
	}

	for _, metric := range aggregate.GetTraceBuiltinDerivedMetrics() {
		fmt.Println("trace builtin:", metric.Name)
	}

	for _, metric := range aggregate.GetLoggingBuiltinDerivedMetrics() {
		fmt.Println("logging builtin:", metric.Name)
	}

	for _, metric := range aggregate.GetRUMBuiltinDerivedMetrics() {
		fmt.Println("rum builtin:", metric.Name)
	}
}
```

## 6. 运行时行为

启用 builtin 后，尾采样运行时的处理顺序是：

1. `TailSamplingData()` 对到期数据执行采样 pipeline
2. 根据数据类型读取配置中的 `builtin_derived_metrics`
3. 解析出允许的内置指标定义
4. 调 `GenerateDerivedMetrics()` 生成指标点
5. 通过 sink 将这些聚合批次写入 `Cache`

这意味着 builtin 派生指标不会直接绕过聚合模块发出，而是继续走现有的窗口聚合链路。

## 7. 特殊变量说明

内置派生指标目前会用到这些特殊变量：

| 变量名 | 描述 |
|--------|------|
| `$trace_duration` | 链路持续时间，当前实现按 `start/end` 计算 |
| `$trace_id` | 用于“每个分组计 1”这类计数场景 |
| `$span_count` | 链路中的 span 数量 |
| `$error_flag` | 错误标记，错误为 1，否则为 0 |

注意：

- `$trace_duration` 的单位和条件表达式支持仍需继续收敛，使用时要结合当前实现看

## 8. 最佳实践

1. 先启用 builtin，再考虑自定义派生指标。
2. trace/logging/RUM 分别只启用本类型支持的 builtin 名称。
3. logging / RUM 优先在稳定的 `group_key` 维度下启用 builtin。
4. 不要在文档或配置里继续使用旧的 `trace.derived_metrics` 写法来表示 builtin。

## 9. 当前限制

1. `builtin_derived_metrics` 已实现并可生效。
2. 用户自定义 `derived_metrics` 目前仍是 TODO。
3. builtin 指标的最终行为仍依赖派生指标生成器对特殊变量的支持情况。
