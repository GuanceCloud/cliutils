# 派生指标设计思路备忘

本文档整理当前对尾采样派生指标的几个核心思路，目标不是直接定稿，而是作为下一步继续细化结构草图或代码实现前的中间设计材料。

## 1. 总体判断

不要再把派生指标当成“尾采样里的一个附属动作”，而要把它当成一条独立的数据生产链路。

更具体地说：

- 尾采样主链路负责组包、缓存、TTL 到期、保留/丢弃决策
- 派生指标链路负责从这些事件里提取统计事实，并在固定窗口内汇总后转成聚合批次

## 2. 用事件流驱动，而不是一次性计算

不要只在 `TailSamplingData()` 里做派生指标。

更合理的做法是定义两类事件：

1. `OnIngest(packet)`
2. `OnDecision(packet, kept|dropped)`

这样天然可以得到两类指标：

- 原始流量类指标
- 保留/丢弃结果类指标

它的好处是：

- 不会丢掉采样前基线
- 可以支持计费类指标
- 可以支持采样效果观察
- 语义比“TTL 到期后统一扫一遍”更清楚

## 3. 不直接产出 point，先产出轻量中间记录

当前思路已经从 `DataPacket -> Point` 往 `DataPacket -> AggregationBatch` 靠了，但还可以再往前压一层：

1. `DataPacket/Event -> DerivedMetricRecord`
2. `DerivedMetricRecord -> 15s flush -> AggregationBatch`

也就是说，在高频路径里不要急着构造 point 或 batch，而是先生成一个很轻的 record。

这样做的好处：

- 高并发路径对象更小
- 减少 point / batch 的频繁创建
- 后续更容易做本地 collector 聚合
- 更适合分布式部署

## 4. builtin 指标应该声明式定义

不要每个指标都写一套单独的处理框架。

更建议把 builtin 指标定义成声明式配置，至少包含：

- 指标名
- 触发阶段
- 取值函数
- 分组标签提取函数
- 聚合方法
- 默认窗口

这样很多计数类指标都只是不同配置，而不是不同框架。

例如这些都可以共用一套模型：

- `trace_total_count`
- `trace_kept_count`
- `trace_dropped_count`
- `trace_error_count`
- `span_total_count`
- `logging_total_count`
- `logging_error_count`
- `rum_total_count`

## 5. 派生指标缓存不要复用时间轮

时间轮是为“延迟决策”设计的，不是为“指标缓冲”设计的。

派生指标建议单独做一个固定窗口 collector，例如 15 秒：

- key: `metric_name + tags + token`
- value: 累加状态
- flush: 每 15 秒一次

这样职责更清楚：

- 时间轮只负责 TTL 和组到期
- collector 只负责派生指标窗口聚合

不要把两种缓存职责混在一起。

## 6. 第一批优先做计数类指标

第一批值得做的不是复杂分位数，而是这些稳定、直接、可解释的指标：

- `trace_total_count`
- `trace_kept_count`
- `trace_dropped_count`
- `trace_error_count`
- `span_total_count`
- `logging_total_count`
- `logging_error_count`
- `rum_total_count`

优先做它们的原因：

- 依赖字段简单
- 语义稳定
- 对运营、计费、采样效果分析都直接有用

## 7. 分位数和分布类单独设计

像下面这些指标不要和计数类一起推进：

- `trace_duration_summary`
- `trace_size_distribution`

原因是它们会立刻带来额外复杂度：

1. 本地聚合状态怎么存
2. 多节点结果怎么 merge
3. histogram / quantile 最终如何转成聚合批次

如果 merge 语义没有想清楚，宁可先不做。

## 8. 给外部项目一个统一 runtime 更省心

最终外部项目最好不是自己手工拼：

- `Cache`
- `GlobalSampler`
- `DerivedMetricCollector`

更建议由库提供一个统一入口，例如概念上的：

- `TailSamplingProcessor`

它可以负责：

- ingest packet
- advance time
- collect derived records
- flush batches

这样外部项目只负责：

- 初始化配置
- 驱动时钟
- 消费返回结果

不用自己拼内部流程。

## 9. 一句话结论

最好的思路不是继续补一个 generator，而是把派生指标改成“事件驱动 + 15 秒 collector + 批量转 `AggregationBatch`”的独立子系统，先把计数类做稳，再碰分布类。
