# 尾采样与派生指标说明

本文档用于迁移到别的项目中复用，描述的是“当前代码现状 + 当前推荐接入方式”，不是未来设想。

## 1. 当前代码里有哪些对象

当前尾采样相关代码已经分成两层：

### 核心尾采样层

- `PickTrace()` / `PickLogging()` / `PickRUM()`
- `GlobalSampler`
- `AdvanceTime()`
- `TailSamplingOutcomes()`
- `TailSamplingData()`

这层负责：

- 组包
- 时间轮缓存
- 到期决策
- 返回保留结果

### 派生指标子层

- `DerivedMetricRecord`
- `DerivedMetricCollector`
- `TailSamplingBuiltinMetric`
- `TailSamplingProcessor`

这层负责：

- 在 ingest / decision 阶段收集 record
- 本地窗口汇聚
- flush 成 point

## 2. 当前真正的入口是谁

如果在别的项目里直接接当前功能，推荐入口是：

- `TailSamplingProcessor`

原因：

1. 它内部持有 `GlobalSampler`
2. 它会在 ingest 阶段记录派生指标
3. 它会在 decision 阶段记录派生指标
4. 它可以定时 flush 派生指标 point

所以当前不要把：

- `derived_metric_*`

这些文件理解成“整个尾采样入口”。  
真正的对外入口是：

- `TailSamplingProcessor`

## 3. 当前最简单初始化方式

最简单的初始化方式：

```go
processor := aggregate.NewDefaultTailSamplingProcessor(shardCount, waitTime)
```

例如：

```go
processor := aggregate.NewDefaultTailSamplingProcessor(16, 5*time.Minute)
```

这个默认入口会自动创建：

1. `GlobalSampler`
2. `DerivedMetricCollector`
3. `DefaultTailSamplingBuiltinMetrics()`

默认值：

- 尾采样等待时间：由 `waitTime` 决定
- 派生指标 flush 窗口：`15s`

如果想手动组装，也可以：

```go
sampler := aggregate.NewGlobalSampler(16, 5*time.Minute)
collector := aggregate.NewDerivedMetricCollector(15 * time.Second)
metrics := aggregate.DefaultTailSamplingBuiltinMetrics()

processor := aggregate.NewTailSamplingProcessor(sampler, collector, metrics)
```

## 4. 当前调用顺序

### 4.1 下发配置

```go
processor.UpdateConfig(token, cfg)
```

这里 `cfg` 类型是：

```go
*aggregate.TailSamplingConfigs
```

### 4.2 数据进入时

先组包，再送进 processor。

#### trace

```go
grouped := aggregate.PickTrace(source, pts, version)
for _, packet := range grouped {
    packet.Token = token
    processor.IngestPacket(packet)
}
```

#### logging

```go
grouped, passedThrough := dim.PickLogging(source, pts)
_ = passedThrough

for _, packet := range grouped {
    packet.Token = token
    processor.IngestPacket(packet)
}
```

#### rum

```go
grouped, passedThrough := dim.PickRUM(source, pts)
_ = passedThrough

for _, packet := range grouped {
    packet.Token = token
    processor.IngestPacket(packet)
}
```

这里 `IngestPacket()` 会同时做两件事：

1. 记录 ingest 阶段 builtin 派生指标
2. 把 packet 送进 `GlobalSampler.Ingest()`

### 4.3 秒级驱动时间轮

```go
expired := processor.AdvanceTime()
kept := processor.TailSamplingData(expired)
```

这里也会同时做两件事：

1. 通过 `GlobalSampler.TailSamplingOutcomes()` 做真实 tail-sampling 决策
2. 根据 outcome 记录 decision 阶段 builtin 派生指标

返回值 `kept` 是：

```go
map[uint64]*aggregate.DataPacket
```

也就是采样保留后的 trace / logging / rum 数据。

## 5. 当前派生指标链路是什么

当前派生指标已经不再依赖 `AggregationBatch`。

它现在走的是：

1. `DataPacket/Event -> DerivedMetricRecord`
2. `DerivedMetricRecord -> DerivedMetricCollector`
3. `DerivedMetricCollector.Flush() -> point`

也就是说：

- 不走 `AggregatorConfigure`
- 不走 `PickPoints()`
- 不走 `AggregationBatch`
- 直接在尾采样本地收集后 flush 成 point

## 6. 当前 flush 出来的是什么

```go
derived := processor.FlushDerivedMetrics(time.Now())
```

返回类型：

```go
[]*aggregate.DerivedMetricPoints
```

结构：

```go
type DerivedMetricPoints struct {
    Token string
    PTS   []*point.Point
}
```

这意味着：

1. 派生指标点已经按 token 分组
2. 外部项目可以直接按 token 发中心
3. token 不需要再从 point 里反解

## 7. 当前 point 形式

当前 flush 后的 point 规则是：

1. measurement 固定为：
   `tail_sampling`
2. field key 是具体指标名
3. field value 是聚合值
4. `stage` / `decision` 作为 tag
5. 业务标签也作为 tag 带上

例如：

```text
measurement = tail_sampling
field       = trace_total_count=123
tags        = stage=ingest, service=checkout, data_type=tracing
```

## 8. 当前默认 builtin 指标

当前 `DefaultTailSamplingBuiltinMetrics()` 已经接入的第一批计数类指标如下。

### trace

- `trace_total_count`
- `trace_error_count`
- `span_total_count`
- `trace_kept_count`
- `trace_dropped_count`

### logging

- `logging_total_count`
- `logging_error_count`
- `logging_kept_count`
- `logging_dropped_count`

### rum

- `rum_total_count`
- `rum_kept_count`
- `rum_dropped_count`

## 9. 当前 `DataPacket` 摘要字段

### trace

当前已经补齐：

- `HasError`
- `PointCount`
- `TraceStartTimeUnixNano`
- `TraceEndTimeUnixNano`

### logging / rum

当前已经补齐：

- `HasError`
- `PointCount`
- `GroupKey`

需要特别说明：

- logging / rum 当前不补 trace 风格起止时间
- 这是当前设计选择，不是遗漏

## 10. 当前还没完成的地方

当前还没完成的主要是：

1. builtin 指标还不是配置驱动启用
2. 自定义 `derived_metrics` 还没实现
3. 更复杂的分布类指标还没接入

所以现在最准确的理解是：

- 尾采样主链路已完成
- 派生指标最小闭环已完成
- 派生指标能力还没完全产品化

## 11. 别的项目里推荐的最小接入方式

最小接入可以按下面理解：

1. 初始化 `TailSamplingProcessor`
2. 为每个 token 更新尾采样配置
3. 业务侧先组包
4. packet 进入 `processor.IngestPacket()`
5. 每秒调一次 `AdvanceTime()` + `TailSamplingData()`
6. 每 15 秒调一次 `FlushDerivedMetrics()`
7. 把：
   - `kept DataPacket`
   - `DerivedMetricPoints`
   分别发出去

## 12. 一句话总结

当前最合理的理解是：

- `GlobalSampler` 仍然是尾采样核心
- `TailSamplingProcessor` 是新的对外入口
- 派生指标现在走的是 `record -> collector -> point`
- 它已经不再依赖 `AggregationBatch`
