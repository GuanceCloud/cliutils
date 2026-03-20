# aggregate 接入文档

本文档用于在别的项目中接入当前 `aggregate` 目录里的两类能力：

1. 指标聚合
2. 尾采样与派生指标收集

目标场景是：

- 项目侧持续接收原始 point / trace / logging / RUM 数据
- 初始化 `Cache` 和 `TailSamplingProcessor`
- 定时从聚合缓存和派生指标 collector 中取出结果并发送

## 1. 当前能力边界

接入前先记住当前代码状态：

### 已可用

1. 指标聚合主链路
2. 尾采样主链路
3. 派生指标的 `record -> collector -> point` 子链路
4. 默认一批 builtin 计数类指标

### 还没做完

1. 派生指标还不是配置驱动启用
2. 自定义 `derived_metrics` 还没实现
3. 分布类派生指标还没接入

所以接入时要理解成：

- 聚合和尾采样是当前稳定能力
- 派生指标是“最小闭环已打通”的能力

## 2. 你需要初始化什么

外部项目里通常需要初始化两个对象：

1. `Cache`
2. `TailSamplingProcessor`

### 2.1 初始化指标聚合

```go
cache := aggregate.NewCache(expired)
```

这里的 `expired` 是聚合窗口容忍延迟，例如：

```go
cache := aggregate.NewCache(30 * time.Second)
```

### 2.2 初始化尾采样入口

最简单的方式：

```go
processor := aggregate.NewDefaultTailSamplingProcessor(shardCount, waitTime)
```

例如：

```go
processor := aggregate.NewDefaultTailSamplingProcessor(16, 5*time.Minute)
```

这个默认入口会自动带上：

1. `GlobalSampler`
2. `DerivedMetricCollector`
3. 默认 builtin 指标集合
4. 默认派生指标 flush 窗口 `15s`

### 2.3 下发尾采样配置

按 token 更新：

```go
processor.UpdateConfig(token, cfg)
```

其中 `cfg` 类型是：

```go
*aggregate.TailSamplingConfigs
```

## 3. 指标聚合怎么接

指标聚合链路是独立的，不依赖尾采样。

外部项目需要自己准备：

1. `AggregatorConfigure`
2. 原始 metric points

### 3.1 初始化聚合配置

```go
var cfg aggregate.AggregatorConfigure
// 加载配置...
if err := cfg.Setup(); err != nil {
    return err
}
```

### 3.2 喂给聚合链路

```go
batchMap := cfg.PickPoints(category, pts)
for _, batchs := range batchMap {
    cache.AddBatchs(token, batchs.Batchs)
}
```

这里：

- `category` 是当前点的类别，例如 metric / logging / tracing
- `pts` 是 `[]*point.Point`
- `token` 是租户标识

## 4. 尾采样怎么接

尾采样链路需要先把数据组包，再喂给 `TailSamplingProcessor`。

### 4.1 trace 组包

```go
packets := aggregate.PickTrace(source, pts, version)
for _, packet := range packets {
    packet.Token = token
    processor.IngestPacket(packet)
}
```

### 4.2 logging 组包

```go
grouped, passedThrough := dim.PickLogging(source, pts)
_ = passedThrough // 按业务侧逻辑旁路处理

for _, packet := range grouped {
    packet.Token = token
    processor.IngestPacket(packet)
}
```

### 4.3 rum 组包

```go
grouped, passedThrough := dim.PickRUM(source, pts)
_ = passedThrough

for _, packet := range grouped {
    packet.Token = token
    processor.IngestPacket(packet)
}
```

## 5. 定时任务怎么驱动

当前建议至少有两个定时任务：

1. 秒级 tick，驱动时间轮
2. flush tick，取聚合结果和派生指标 point

### 5.1 秒级 tick

```go
expired := processor.AdvanceTime()
keptPackets := processor.TailSamplingData(expired)
```

这里 `keptPackets` 类型是：

```go
map[uint64]*aggregate.DataPacket
```

这些就是采样后保留下来的 trace / logging / RUM 分组数据，外部项目可以直接发送到中心。

### 5.2 聚合 flush tick

```go
windows := cache.GetExpWidows()
pointsData := aggregate.WindowsToData(windows)
```

这里 `pointsData` 类型是：

```go
[]*aggregate.PointsData
```

每个元素里有：

- `Token`
- `PTS []*point.Point`

这些就是聚合后的指标点。

### 5.3 派生指标 flush tick

```go
derived := processor.FlushDerivedMetrics(time.Now())
```

这里返回的是：

```go
[]*aggregate.DerivedMetricPoints
```

每个元素里有：

- `Token`
- `PTS []*point.Point`

这些 point 的 measurement 固定为：

```go
tail_sampling
```

field key 是具体指标名，例如：

- `trace_total_count`
- `trace_kept_count`
- `logging_total_count`

## 6. 当前 builtin 派生指标

当前默认接通的是第一批计数类指标。

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

## 7. 发送时你会拿到什么

外部项目最终会拿到三类发送对象：

### 7.1 尾采样保留结果

来自：

```go
keptPackets := processor.TailSamplingData(expired)
```

类型：

```go
map[uint64]*aggregate.DataPacket
```

### 7.2 指标聚合结果

来自：

```go
pointsData := aggregate.WindowsToData(windows)
```

类型：

```go
[]*aggregate.PointsData
```

### 7.3 派生指标结果

来自：

```go
derived := processor.FlushDerivedMetrics(time.Now())
```

类型：

```go
[]*aggregate.DerivedMetricPoints
```

## 8. 推荐的整体伪代码

```go
type Runtime struct {
    cache     *aggregate.Cache
    processor *aggregate.TailSamplingProcessor
    aggrCfg   map[string]*aggregate.AggregatorConfigure
}

func NewRuntime() *Runtime {
    return &Runtime{
        cache:     aggregate.NewCache(30 * time.Second),
        processor: aggregate.NewDefaultTailSamplingProcessor(16, 5*time.Minute),
        aggrCfg:   map[string]*aggregate.AggregatorConfigure{},
    }
}

func (r *Runtime) UpdateTailSamplingConfig(token string, cfg *aggregate.TailSamplingConfigs) {
    r.processor.UpdateConfig(token, cfg)
}

func (r *Runtime) UpdateAggregateConfig(token string, cfg *aggregate.AggregatorConfigure) error {
    if err := cfg.Setup(); err != nil {
        return err
    }
    r.aggrCfg[token] = cfg
    return nil
}

func (r *Runtime) IngestMetric(token string, category string, pts []*point.Point) {
    cfg := r.aggrCfg[token]
    if cfg == nil {
        return
    }

    batchMap := cfg.PickPoints(category, pts)
    for _, batchs := range batchMap {
        r.cache.AddBatchs(token, batchs.Batchs)
    }
}

func (r *Runtime) IngestTrace(token string, source string, pts []*point.Point, version int64) {
    grouped := aggregate.PickTrace(source, pts, version)
    for _, packet := range grouped {
        packet.Token = token
        r.processor.IngestPacket(packet)
    }
}

func (r *Runtime) Tick1s() map[uint64]*aggregate.DataPacket {
    expired := r.processor.AdvanceTime()
    return r.processor.TailSamplingData(expired)
}

func (r *Runtime) Flush() ([]*aggregate.PointsData, []*aggregate.DerivedMetricPoints) {
    windows := r.cache.GetExpWidows()
    aggrPoints := aggregate.WindowsToData(windows)
    derivedPoints := r.processor.FlushDerivedMetrics(time.Now())
    return aggrPoints, derivedPoints
}
```

## 9. 一句话总结

在别的项目里接当前 `aggregate` 能力时，可以这样理解：

- `Cache` 负责指标聚合
- `TailSamplingProcessor` 负责尾采样和派生指标 record/collect/flush
- 秒级驱动时间轮
- 周期性 flush 聚合结果和派生指标 point
