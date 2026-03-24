# aggregate 功能总览

这份文档是 `aggregate` 包的主文档，面向两类读者：

- 开发人员：需要知道当前实现到底怎么工作，入口在哪里，改代码时应该看哪些文件和测试
- 测试人员：需要知道配置长什么样、结果应该长什么样、哪些行为是当前代码的真实边界

如果你只读一份文档，优先读这份。

## 1. 这一个包里其实有两套系统

`aggregate` 不是单一的“指标聚合器”，而是两套能力放在同一个包里：

1. 指标聚合
2. trace / logging / RUM 尾采样

两套能力都属于“先按规则分组，再延迟处理，再输出结果”的系统，但执行载体不同：

- 指标聚合：`RuleSelector -> AggregationBatch -> Calculator -> Cache/Windows -> point`
- 尾采样：`Pick* -> DataPacket -> GlobalSampler -> TailSamplingProcessor -> kept packet / derived point`

## 2. 当前代码状态

先把当前状态讲清楚，后面读代码和测试时就不容易误判：

- 指标聚合主链路已可用
- 尾采样主链路已可用
- 尾采样 builtin 派生指标已可用
- 自定义 `derived_metrics` 配置结构仍保留，但当前运行时还没有实现，初始化阶段会直接报错
- `expo_histogram` 不会实现，当前会在配置校验阶段直接报错
- `last` / `first` 已实现，但当前仍走聚合器的数值路径，不是通用“任意类型取首尾值”
- `quantiles` 当前按样本集合做线性插值计算，且配置会校验 `percentiles` 必须落在 `[0,1]`

## 3. 目录和入口

最值得先看的文件：

- `aggregate/aggr.go`
  指标聚合配置、规则筛选、分组、批次构建
- `aggregate/calculator.go`
  聚合算子工厂 `newCalculators()`
- `aggregate/windows.go`
  聚合窗口缓存和到期输出
- `aggregate/algo_*.go`
  各个聚合方法的具体实现
- `aggregate/tail-sampling.go`
  尾采样配置结构、trace/logging/RUM 分组逻辑
- `aggregate/timewheel.go`
  `GlobalSampler` 和时间轮决策
- `aggregate/tail_sampling_processor.go`
  当前推荐的尾采样对外入口
- `aggregate/tail_sampling_builtin_metrics.go`
  builtin 派生指标定义
- `aggregate/derived_metric_collector.go`
  builtin 派生指标的本地窗口汇聚和 flush

## 4. 指标聚合

### 4.1 聚合到底在做什么

指标聚合会把原始 point 按规则筛出来，按 `group_by` 聚到同一个实例里，等窗口到期后输出新的聚合点。

整体流程：

`原始 point -> SelectPoints() -> GroupbyBatch() -> newCalculators() -> Cache.AddBatch() -> GetExpWidows() -> WindowsToData()`

### 4.2 当前真实配置结构

这里先强调两个最容易被旧文档误导的点：

1. 当前规则字段名是 `algorithms`，不是 `aggregate`
2. 当前分组字段是 `group_by = ["tag1", "tag2"]`，不是 `[group_by] tags = [...]`

当前 Go 结构体真相来自 `AggregateRule`：

```go
type AggregateRule struct {
    Name       string                      `toml:"name" json:"name"`
    Selector   *RuleSelector               `toml:"select" json:"select"`
    Groupby    []string                    `toml:"group_by" json:"group_by"`
    Algorithms map[string]*AggregationAlgo `toml:"algorithms" json:"algorithms"`
}
```

顶层配置真相来自 `AggregatorConfigure`：

- `default_window`
- `aggregate_rules`
- `action`
- `delete_rules_point`

### 4.3 聚合配置示例

下面这个 TOML 形状和当前 struct tag 对齐：

```toml
default_window = "10s"
action = "passthrough"
delete_rules_point = false

[[aggregate_rules]]
name = "aggregate_nginx_access_logs"
group_by = ["fields.http_host", "fields.upstream_service"]

  [aggregate_rules.select]
  category = "logging"
  measurements = ["nginx"]
  metric_name = [
    "fields.request_time_ms",
    "fields.body_bytes_sent",
    "fields.client_ip",
    "fields.upstream_service",
  ]
  condition = """
  { fields.status_code >= 200 AND fields.status_code < 300 }
  """

  [aggregate_rules.algorithms.request_count]
  method = "count"

  [aggregate_rules.algorithms.total_bytes_sent]
  source_field = "fields.body_bytes_sent"
  method = "sum"

  [aggregate_rules.algorithms.unique_client_ips]
  source_field = "fields.client_ip"
  method = "count_distinct"

  [aggregate_rules.algorithms.latest_request_time_ms]
  source_field = "fields.request_time_ms"
  method = "last"

  [aggregate_rules.algorithms.latest_request_time_ms.add_tags]
  aggregated_by = "my-aggregator"
```

如果是 Go 侧直接构造，最可靠的方式仍然是 struct literal：

```go
cfg := &aggregate.AggregatorConfigure{
    DefaultWindow: 10 * time.Second,
    DefaultAction: aggregate.ActionPassThrough,
    AggregateRules: []*aggregate.AggregateRule{
        {
            Name:    "aggregate_nginx_access_logs",
            Groupby: []string{"fields.http_host", "fields.upstream_service"},
            Selector: &aggregate.RuleSelector{
                Category:     point.Logging.String(),
                Measurements: []string{"nginx"},
                MetricName: []string{
                    "fields.request_time_ms",
                    "fields.body_bytes_sent",
                    "fields.client_ip",
                    "fields.upstream_service",
                },
                Condition: `{ fields.status_code >= 200 AND fields.status_code < 300 }`,
            },
            Algorithms: map[string]*aggregate.AggregationAlgo{
                "request_count": {
                    Method: "count",
                },
                "total_bytes_sent": {
                    SourceField: "fields.body_bytes_sent",
                    Method:      "sum",
                },
                "latest_request_time_ms": {
                    SourceField: "fields.request_time_ms",
                    Method:      "last",
                },
            },
        },
    },
}
```

### 4.4 `method` 怎么写

当前 `AggregationAlgo.method` 已经是字符串字段，不再是 enum/int。

可直接使用：

- `sum`
- `avg`
- `count`
- `min`
- `max`
- `histogram`
- `merge_histogram`
- `stdev`
- `quantiles`
- `count_distinct`
- `last`
- `first`
- `expo_histogram`

其中：

- `merge_histogram` 会在运行时被归一化成 `histogram`
- `expo_histogram` 当前会在 `Setup()` 阶段直接报错
- 未识别的方法名在正常配置路径下会被 `Setup()` 直接拒绝；如果绕过 `Setup()`，工厂默认分支仍不会创建算子

### 4.5 当前已实现的聚合方法

当前 `newCalculators()` 真正会创建实例的方法有：

- `sum`
- `avg`
- `count`
- `min`
- `max`
- `histogram`
- `quantiles`
- `stdev`
- `count_distinct`
- `last`
- `first`

当前仍未落地：

- `expo_histogram`

### 4.5.1 当前配置校验会拦哪些错误

聚合侧现在会在 `AggregatorConfigure.Setup()` 里直接拒绝这些配置：

- 未知 `method`
- `method = "expo_histogram"`
- `method = "quantiles"` 但没配 `quantile_opts.percentiles`
- `quantile_opts.percentiles` 中出现不在 `[0,1]` 的值

### 4.6 选择器的真实语义

这是聚合配置里最容易踩坑的地方。

`RuleSelector` 只认这几个字段：

- `category`
- `measurements`
- `metric_name`
- `condition`

几个关键事实：

1. `metric_name` 不是装饰项，而是决定会不会 fork 出待聚合 field 的关键配置
2. 当前实现会把一个 point 拆成多个“每个点只有一个非 tag field”的 point
3. `group_by` 中的 key 会被重新附着到拆分后的新点上

换句话说：

- 如果 `metric_name` 为空，当前 `selectKVS()` 不会产出新的待聚合 point
- 如果 `source_field` 对应的 field 没有先在选择阶段被 fork 出来，后面也不会 magically 出现

### 4.7 `group_by` 的真实语义

`group_by` 是一个字符串数组，不是带 `tags` 子字段的 table。

它同时影响两件事：

1. 参与聚合 hash，决定哪些点进入同一个聚合实例
2. 作为 tag 或 field 重新挂到拆分后的新点上

这里还有一个实现细节：

- 如果 `group_by` 中的 key 在原 point 上是 tag，就会作为 tag 附到新点
- 如果它在原 point 上是 field，也会被附回去

### 4.8 聚合方法的当前边界

当前工厂里，字段值会先尝试走数值路径。

真实边界是：

- `count` 和 `count_distinct` 可以接受非 float/int 的原始值
- 其余方法当前仍要求值最终能走到数值路径
- `last` / `first` 目前虽然已实现，但本质上仍是“数值型首尾值”

所以不要把当前 `last` / `first` 理解成“任意字段类型都可用”的通用版本。

### 4.9 聚合输出长什么样

大部分聚合方法最终输出一个或多个 `*point.Point`。

通常形状是：

- 主字段：输出字段名本身
- 辅助字段：`<field>_count`
- tag：原始点上的相关 tag + `add_tags`

例子：

- `sum` 输出：`latency_sum` 不是固定命名，而是你配置的输出字段名，比如 `latency`
- `count` 输出：
  - `request_count`
  - `request_count_count`

当前 `_count` 字段的含义要以具体算法实现为准，不要笼统理解成“总是样本数”。

### 4.10 聚合窗口和缓存

聚合缓存结构是：

- `Cache`
- `Windows`
- `Window`
- `map[uint64]Calculator`

工作方式：

1. `AddBatch()` 用 `newCalculators()` 把 batch 变成算子
2. 算子用 `nextWallTime + Expired` 决定过期桶
3. `Cache.GetAndSetBucket()` 把算子放进对应过期时间桶
4. `GetExpWidows()` 取出到期窗口
5. `WindowsToData()` 把窗口结果转成 point

### 4.11 聚合侧当前最值得测试的地方

如果你在改聚合逻辑，优先看这些测试：

- `aggregate/aggr_test.go`
  规则筛选、批次构建、HTTP batch 发送
- `aggregate/windows_test.go`
  `Cache` 并发写入和到期输出
- `aggregate/calculator_test.go`
  窗口时间对齐
- `aggregate/algo_test.go`
  `count_distinct`
- `aggregate/algo_first_last_test.go`
  `last` / `first`

## 5. 尾采样

### 5.1 当前推荐入口

如果你要在别的项目里直接接当前尾采样能力，首选入口是：

- `TailSamplingProcessor`

原因很直接：

- 它封装了 `GlobalSampler`
- 它会在 ingest / pre-decision / decision 三个阶段记录 builtin 派生指标
- 它负责 `FlushDerivedMetrics()`

直接只用 `GlobalSampler` 也能做采样决策，但拿不到 builtin 派生指标闭环。

### 5.2 尾采样整体流程

整体流程：

`point -> PickTrace/PickLogging/PickRUM -> DataPacket -> TailSamplingProcessor.IngestPacket() -> GlobalSampler -> AdvanceTime() -> TailSamplingData() -> kept packets + derived metric points`

### 5.3 三类数据的分组方式

- trace：固定按 `trace_id`
- logging：按 `group_dimensions[].group_key`
- RUM：按 `group_dimensions[].group_key`

logging / RUM 有一个非常实际的行为：

- 如果点上缺少 `group_key` 对应字段，它不会进入采样缓存，而是被 `PickLogging()` / `PickRUM()` 作为 `passedThrough` 返回

### 5.4 尾采样配置结构

顶层配置结构：

```toml
[trace]
[logging]
[rum]
```

三类配置分别对应：

- `TraceTailSampling`
- `LoggingTailSampling`
- `RUMTailSampling`

几个需要记住的默认值：

- trace 默认 `data_ttl = 5m`
- logging 默认 `data_ttl = 1m`
- RUM 默认 `data_ttl = 1m`
- trace 默认 `group_key = "trace_id"`
- builtin 指标如果不写，`Init()` 会补默认并全部开启

### 5.5 trace 配置示例

```toml
[trace]
data_ttl = "5m"

  [[trace.builtin_metrics]]
  name = "trace_total_count"
  enabled = true

  [[trace.builtin_metrics]]
  name = "trace_duration"
  enabled = true

  [[trace.sampling_pipeline]]
  name = "keep_errors"
  type = "condition"
  condition = "{ status = 'error' }"
  action = "keep"

  [[trace.sampling_pipeline]]
  name = "default_sample"
  type = "probabilistic"
  condition = "{ 1 = 1 }"
  rate = 0.1
```

### 5.6 logging 配置示例

```toml
[logging]
data_ttl = "1m"

  [[logging.builtin_metrics]]
  name = "logging_total_count"
  enabled = true

  [[logging.group_dimensions]]
  group_key = "fields.user_id"

    [[logging.group_dimensions.pipelines]]
    name = "keep_error_logs"
    type = "condition"
    condition = "{ status = 'error' }"
    action = "keep"

    [[logging.group_dimensions.pipelines]]
    name = "sample_normal_logs"
    type = "probabilistic"
    condition = "{ 1 = 1 }"
    rate = 0.1
```

### 5.7 RUM 配置示例

```toml
[rum]
data_ttl = "1m"

  [[rum.builtin_metrics]]
  name = "rum_total_count"
  enabled = true

  [[rum.group_dimensions]]
  group_key = "fields.session.id"

    [[rum.group_dimensions.pipelines]]
    name = "keep_error_sessions"
    type = "condition"
    condition = "{ status = 'error' }"
    action = "keep"
```

### 5.8 采样管道的真实语义

当前 `SamplingPipeline` 支持两类：

- `condition`
- `probabilistic`

真实执行语义：

1. pipeline 顺序执行
2. 第一条给出最终决定的 pipeline 生效
3. 后面的 pipeline 不再执行

另外两个容易误判的点：

- 如果条件没有成功解析，`DoAction()` 不会做决定
- 很有可能一个都没有匹配到就结束了，也会被删除掉。所以 **要有百分比采样兜底**

所以如果你想做“默认概率采样”，不要留空条件，写一个始终为真的条件更安全。

### 5.9 时间轮和 TTL

时间轮在 `GlobalSampler` 里，关键行为：

- 每个 shard 有 `activeMap + 3600 slots`
- `AdvanceTime()` 每调用一次，时间轮前进一步
- 每次推进会吐出当前槽位到期的 `DataGroup`

这意味着：

- 调度粒度是秒
- TTL 最大有效范围约 3600 秒
- 超过 3600 秒会被压到 3599 秒附近

### 5.10 builtin 派生指标

当前 builtin 派生指标已经接入，不再是历史文档里的 TODO。

真正的运行链路是：

`TailSamplingProcessor -> TailSamplingBuiltinMetrics -> DerivedMetricRecord -> DerivedMetricCollector -> point`

它不再依赖 `AggregationBatch`。

### 5.11 builtin 指标列表

trace：

- `trace_total_count`
- `trace_kept_count`
- `trace_dropped_count`
- `trace_error_count`
- `span_total_count`
- `trace_duration`

logging：

- `logging_total_count`
- `logging_error_count`
- `logging_kept_count`
- `logging_dropped_count`

rum：

- `rum_total_count`
- `rum_kept_count`
- `rum_dropped_count`

### 5.12 builtin 指标输出格式

flush 后得到的是：

```go
type DerivedMetricPoints struct {
    Token string
    PTS   []*point.Point
}
```

point 的通用特点：

- measurement 固定为 `tail_sampling`
- field 是具体指标名
- `stage` 是 tag
- `decision` 在 decision 阶段会作为 tag 输出
- 还会带上 `data_type`、`source`、`group_key`、`service` 等 tag

`trace_duration` 是 histogram，会输出：

- `trace_duration_bucket`
- `trace_duration_sum`
- `trace_duration_count`

默认 bucket：

- `[1, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000]`

### 5.13 派生指标当前还没实现的部分

当前仍未完成的是：

- 用户自定义 `derived_metrics` 运行时
- 更复杂的自定义分布型指标产品化能力

所以现在最准确的理解是：

- builtin 指标已闭环
- 自定义 `derived_metrics` 仍是 TODO，且配置初始化会直接拒绝这类配置

### 5.14 尾采样侧最值得测试的地方

如果你在改尾采样逻辑，优先看这些测试：

- `aggregate/tail-sampling_test.go`
  trace / logging / RUM 分组与 pipeline 行为
- `aggregate/timewheel_test.go`
  TTL 到期和时间轮吐数据
- `aggregate/derived_metric_collector_test.go`
  builtin 派生指标 flush、时间窗口和 histogram 输出

## 6. 当前最容易踩坑的地方

### 6.1 聚合侧

- `metric_name` 没配，当前不会 fork 出待聚合字段
- `group_by` 是字符串数组，不是旧文档里的 table 结构
- 当前真实配置字段叫 `algorithms`，不是旧文档里的 `aggregate`
- `action` 是顶层字符串，不是 `[action]` table
- `last` / `first` 虽然已实现，但还不是“任意类型首尾值”
- `quantiles` 只接受 `[0,1]` 范围内的百分位配置
- `expo_histogram` 不是“暂时没测”，而是当前明确不支持

### 6.2 尾采样侧

- 只用 `GlobalSampler` 不会自动得到 builtin 指标
- `hash_keys` 的语义是“存在即保留”，不是“参与 hash”
- logging / RUM 缺少分组键的数据会直接旁路
- custom `derived_metrics` 结构存在，但当前配置会被直接拒绝

## 7. 建议的阅读顺序

如果要快速建立全局理解，建议按这个顺序读：

1. 本文
2. `aggregate/aggr.go`
3. `aggregate/calculator.go`
4. `aggregate/windows.go`
5. `aggregate/tail-sampling.go`
6. `aggregate/timewheel.go`
7. `aggregate/tail_sampling_processor.go`
8. 对应测试文件

## 8. 文档整理说明

为了减少重复和冲突：

- `aggregate/docs/feature-overview.md` 现在是主文档
- 旧的 `aggregate/docs/config.md` 和 `aggregate/docs/docs.md` 不再保留

如果以后发现实现变化，优先更新这份文档，再决定是否同步扩展阅读材料。
