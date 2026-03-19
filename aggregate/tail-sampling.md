# 尾采样与派生指标说明

本文档用于放到别的项目中复用，重点说明三件事：

1. 当前 `aggregate` 目录里的尾采样主链路怎么工作
2. 当前新增的派生指标代码已经做到哪一步
3. 在别的项目里应该怎样初始化和调用

## 1. 先分清现状

当前代码里有两条相关链路：

1. 尾采样主链路
2. 派生指标子链路

它们的关系是：

- 尾采样主链路已经可用
- 派生指标子链路已经有最小骨架
- 但还没有做到“配置驱动、完整闭环、全部 builtin 都可控启用”

也就是说，当前状态不是“完全没有派生指标”，也不是“派生指标已经全部完成”，而是：

- ingest / decision 两个阶段已经能产出 record
- record 已经能本地 collect
- collect 已经能定时 flush 成 point
- 但当前 builtin 仍是代码内置默认集合，不是配置驱动

## 2. 当前代码结构

### 2.1 尾采样主链路

当前主链路仍然是：

1. `PickTrace()` / `PickLogging()` / `PickRUM()`
2. `GlobalSampler.Ingest()`
3. `AdvanceTime()`
4. `TailSamplingData()`

这里面：

- `Pick*` 负责组包成 `DataPacket`
- `GlobalSampler` 负责时间轮缓存
- `TailSamplingData()` 负责保留 / 丢弃决策

### 2.2 派生指标子链路

当前新增的派生指标子链路是：

1. `DerivedMetricRecord`
2. `DerivedMetricCollector`
3. `TailSamplingBuiltinMetric`
4. `TailSamplingProcessor`

职责分别是：

- `DerivedMetricRecord`
  派生指标的轻量事件对象
- `DerivedMetricCollector`
  本地按固定窗口汇聚 record
- `TailSamplingBuiltinMetric`
  builtin 指标接口
- `TailSamplingProcessor`
  对外入口，负责把尾采样主链路和派生指标子链路串起来

## 3. 当前推荐入口

在别的项目里，不建议直接把 `GlobalSampler` 当作唯一入口，而是优先使用：

- `TailSamplingProcessor`

原因是它现在已经负责了两件事：

1. 驱动 `GlobalSampler`
2. 在 ingest / decision 两个阶段收集派生指标 record

### 3.1 最简单初始化方式

当前最直接的初始化入口是：

```go
processor := aggregate.NewDefaultTailSamplingProcessor(shardCount, waitTime)
```

这个默认入口会同时初始化：

- `GlobalSampler`
- `DerivedMetricCollector`
- 默认 builtin 指标集合 `DefaultTailSamplingBuiltinMetrics()`

其中：

- 默认派生指标 flush 窗口是 `15s`
- collector flush 输出的是 `point.Point`
- 输出 measurement 固定为 `tail_sampling`

### 3.2 自定义初始化方式

如果调用方想自己控制依赖，也可以手动组装：

```go
sampler := aggregate.NewGlobalSampler(shardCount, waitTime)
collector := aggregate.NewDerivedMetricCollector(30 * time.Second)
metrics := aggregate.DefaultTailSamplingBuiltinMetrics()

processor := aggregate.NewTailSamplingProcessor(sampler, collector, metrics)
```

如果还要下发配置：

```go
processor.UpdateConfig(token, cfg)
```

## 4. 当前调用顺序

外部项目里建议按下面顺序使用。

### 4.1 数据进入时

先组包，再进入 processor：

```go
for _, packet := range packets {
    processor.IngestPacket(packet)
}
```

这里会同时做两件事：

1. 记录 ingest 阶段的派生指标 record
2. 把 packet 送进 `GlobalSampler`

### 4.2 时间轮推进时

```go
expired := processor.AdvanceTime()
kept := processor.TailSamplingData(expired)
```

这里会同时做两件事：

1. 触发 `GlobalSampler.TailSamplingData()`
2. 根据结果为每个 packet 记录 decision 阶段的 record

返回值 `kept` 仍然是原来的尾采样保留结果：

- `map[uint64]*DataPacket`

### 4.3 派生指标 flush 时

```go
pts := processor.FlushDerivedMetrics(time.Now())
```

当前返回的是：

```go
[]*aggregate.DerivedMetricPoints
```

也就是按 token 分组后的 point 列表：

```go
type DerivedMetricPoints struct {
    Token string
    PTS   []*point.Point
}
```

这意味着外部项目可以直接按 token 把这些 point 发往中心，而不需要再走 `AggregationBatch`。

## 5. 当前 builtin 指标

当前默认 builtin 指标集合由：

- `DefaultTailSamplingBuiltinMetrics()`

提供。

第一批已经接通的计数类指标有：

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

## 6. 当前数据模型约束

### 6.1 `DataPacket` 当前摘要字段

当前代码已经准备好的 packet 级信息：

### trace

- `HasError`
- `PointCount`
- `TraceStartTimeUnixNano`
- `TraceEndTimeUnixNano`

### logging / rum

- `HasError`
- `PointCount`
- `GroupKey`

这里要特别说明：

- logging / RUM 当前不补 trace 风格起止时间
- 这是当前设计选择，不是遗漏

### 6.2 record 到 point 的规则

当前 flush 成 point 时遵循：

1. measurement 固定为 `tail_sampling`
2. field key 是具体指标名
3. field value 是聚合后的数值
4. `stage` / `decision` 作为 tag 输出
5. token 不放在 point 里，而是通过 `DerivedMetricPoints.Token` 单独返回

## 7. 当前设计为什么不再依赖 `AggregationBatch`

现在派生指标已经明确和聚合模块解耦：

- 不走 `AggregatorConfigure`
- 不走 `PickPoints()`
- 不走 `AggregationBatch`

改成：

- `DataPacket/Event -> DerivedMetricRecord`
- `DerivedMetricRecord -> Collector`
- `Collector Flush -> point`
- `point 直接发送中心`

这样做的原因：

1. 尾采样代码不需要额外依赖聚合规则
2. 外部项目不需要为了派生指标再准备一套 `AggregatorConfigure`
3. 高频路径对象更轻
4. 语义更适合“尾采样本地统计，中心直接消费 point”

## 8. 当前还没做完的地方

当前代码还没有完成的点主要是：

1. builtin 指标还不是配置驱动，当前是默认集合
2. 分布类指标还没接入
3. 自定义 `derived_metrics` 还没实现
4. 文档里的更大目标仍然包括继续压缩对象创建和完善 flush/发送链路

## 9. 下一步建议

如果在别的项目里继续推进，我建议顺序是：

1. 先把 `TailSamplingProcessor` 作为唯一入口接进去
2. 先消费当前默认 builtin 的 point 输出
3. 再把 builtin 指标改成配置驱动启用
4. 最后再考虑分布类指标和自定义指标

## 10. 一句话总结

当前最合理的理解是：

- `GlobalSampler` 仍然是尾采样核心
- `TailSamplingProcessor` 是新的对外入口
- 派生指标现在走的是 `record -> collector -> point` 链路
- 它已经不再依赖 `AggregationBatch`
