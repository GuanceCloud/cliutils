# 尾采样设计说明

本文档用于约束后续尾采样和内置派生指标的代码改造方向。目标不是描述现状，而是明确下一步应按什么模型继续改。

## 1. 总体流程

1. Datakit 加载聚合配置和尾采样配置到内存。
2. Datakit 按 token / 路由规则将配置分发到后端尾采样器。
3. Datakit 将链路、日志、RUM 先按各自的分组规则组包，再发送到后端尾采样器。
4. 尾采样器收到 `DataPacket` 后，按 TTL 缓存到时间轮中。
5. 到期后执行 `pipeline.DoAction`，决定这一组数据保留还是丢弃。
6. 在尾采样过程中生成内置派生指标，并将其转成聚合批次继续进入聚合链路。

## 2. 先明确几个设计结论

这些结论应该视为后续改码时的约束，不再反复摇摆。

### 2.1 尾采样阶段只处理内置派生指标

当前阶段：

- 支持内置派生指标
- 用户自定义 `derived_metrics` 配置先不实现

也就是说，后续代码改造应该优先把 builtin 跑通，不要继续在用户自定义指标上分散精力。

### 2.2 派生指标不依赖 `AggregatorConfigure`

尾采样代码里没有 `AggregatorConfigure`，所以尾采样器不应该依赖：

- `PickPoints`
- 聚合规则筛选
- 外部额外初始化一套“专供派生指标使用”的 `AggregatorConfigure`

结论：

- 尾采样器后续会按 15 秒定时批量转成 `AggregationBatch`
- 当前代码里这部分实现已经移除，`TailSamplingData()` 只保留 `TODO: 生成派生指标`

### 2.3 `point.NewPoint()` 使用固定指标集

这是后续重做时仍然建议遵守的重要约束：

- `measurement` 使用固定的指标集
- `fields` 才是指标名和值

例如：

- measurement: `tail_sampling`
- field: `trace_total_count = 1`
- tags: `service=api, status=error`

这意味着：

- 不应该把具体指标名放在 measurement 上
- 不应该再用 measurement 后缀去区分 bucket / count / sum
- bucket / count / sum 应该体现在 field 名上

例如：

- `trace_duration_bucket`
- `trace_duration_count`
- `trace_duration_sum`

### 2.4 派生指标默认窗口时间为 15 秒

如果派生指标构造成 `AggregationBatch` 时缺少窗口时间，统一使用：

- `15s`

这个值应作为默认值存在，不依赖外部额外配置。

## 3. 现在真正需要解决的问题

### 3.1 指标统计应该分两个阶段

结合尾采样流程，内置派生指标不应该只在一个时刻统计。

至少应区分两类：

1. **进入采样器后的原始统计**
   用于描述流量、原始 point 数、原始 group 数、原始错误数。

2. **pipeline 决策后的结果统计**
   用于描述保留数、丢弃数、保留后的 point 数、保留后的 group 数。

如果只做决策后的指标，会丢失采样前的真实流量基线。
如果只做采样前的指标，又无法体现 tail sampling 的过滤效果。

所以后续代码设计上，应该显式支持这两类统计。

> 注意： 流量、个数、过滤后的各种指标可能作为收费的参考点。

### 3.2 分布式部署下，指标要按“本地先聚合，再中心汇总”的思路来设计

尾采样器是分布式部署的，因此 builtin 指标不应该依赖中心节点去重新理解一遍原始 trace/logging/RUM。

更合理的思路是：

1. 尾采样器本地把 builtin 指标先转成 `AggregationBatch`
2. 本地窗口聚合
3. 聚合结果再发往中心
4. 中心如果需要，只做更高层的 sum / merge

这样做的好处是：

- 尾采样器本地可观察
- 分布式下每个 sampler 的工作量是闭环的
- 中心只处理结果，不重新参与 tail sampling 语义判断

## 4. 推荐的代码抽象

### 4.1 不再围绕“生成 point”设计，而是围绕“生成 batch”设计

现在最核心的抽象不应该是：

- `DataPacket -> Point`

而应该是：

- `DataPacket -> AggregationBatch`

因为 tail sampling 的目标不是“临时产出一个指标点”，而是“把它继续接到聚合链路里”。

### 4.2 为内置派生指标定义统一接口

文档建议后续引入一个统一接口，类似：

- 输入：`DataPacket`
- 输出：`[]*AggregationBatch`

这个接口的职责是：

- 判断当前 builtin 是否适用于这个 `DataPacket`
- 提取所需字段
- 生成可直接进入聚合模块的 batch

这样 `trace_total_count`、`span_total_count`、`trace_error_count`、`logging_total_count` 等内置指标都走同一套抽象。

### 4.3 建议按“事件阶段”拆 builtin 处理器

后续实现上，builtin 处理器建议按两个阶段拆：

1. `OnIngest`
   数据刚进入采样器时触发，负责原始流量统计。

2. `OnDecision`
   pipeline 决策完成后触发，负责保留/丢弃结果统计。

这样后续扩展诸如：

- `trace_total_count`
- `trace_kept_count`
- `trace_dropped_count`
- `logging_total_count`
- `logging_kept_count`

都会比较自然。

## 5. `DataPacket` 层应该承担的职责

`DataPacket` 不应只是 point 容器，它还应该承载 packet 级摘要信息，供 builtin 指标直接使用。

至少包括：

- `HasError`
- `PointCount`
- `TraceStartTimeUnixNano`
- `TraceEndTimeUnixNano`

这些字段应该在组包阶段尽早填好，而不是等 builtin 指标处理时再重新扫一遍 points。

这样做的好处：

- pipeline 可以直接使用 packet 级事实
- builtin 指标可以直接使用 packet 级事实
- 逻辑不会在多个阶段重复扫描 points

## 6. 关于内置派生指标的分类建议

下一步代码改造时，建议先把 builtin 分成几类：

### 6.1 计数类

例如：

- `trace_total_count`
- `trace_error_count`
- `span_total_count`
- `logging_total_count`
- `logging_error_count`
- `rum_total_count`

这类最容易先稳定下来，应优先完成。

### 6.2 比率类

例如：

- `trace_error_rate`

这类通常依赖计数类结果，建议明确它是：

- 直接生成原始 0/1 样本再交给聚合模块算 AVG

而不是在 tail sampling 阶段直接算最终比率。

### 6.3 分布类

例如：

- `trace_duration_summary`
- `trace_size_distribution`

这类要特别小心：

- histogram / quantile 的表达形式必须和“固定 measurement + field 为指标名”模型一致
- 不要一边要求固定 measurement，一边又把 bucket/count/sum 塞回 measurement 名称

## 7. 当前文档建议的改码顺序

后续建议按下面顺序推进，而不是一次性大改：

1. 先稳定 `DataPacket` 的摘要字段
2. 再稳定 builtin 指标的统一接口
3. 先完成计数类 builtin
4. 再处理比率类
5.  histogram / quantile 这类分布指标先写对象，方法内部 使用 `// todo` 即可


这样可以减少“为了支持复杂指标，把整个框架一起搅乱”的风险。

## 8. 当前不建议做的事

1. 不要让尾采样逻辑依赖 `AggregatorConfigure`。
2. 不要再通过 `PickPoints()` 把派生指标接回聚合模块。
3. 不要把 measurement 当成具体派生指标名。
4. 不要同时推进 builtin 和自定义指标，两条线会互相干扰。

## 9. 下一步代码改造的目标

如果按本文档继续改，下一步代码应达到以下目标：

1. 内置派生指标都能直接输出 `AggregationBatch`
2. batch 默认窗口为 `15s`
3. measurement 固定为统一指标集
4. field 才是真正指标名
5. 先支持 ingest 阶段与 decision 阶段两类 builtin 统计
6. 自定义 `derived_metrics` 继续保留 TODO，不在本轮实现

## 10. 一句话总结

后续尾采样改造的核心，不是“再多写一些 if/else 生成指标”，而是把内置派生指标抽象成一套以 `DataPacket -> AggregationBatch` 为中心的稳定模型，并明确 measurement 与 field 的职责边界。
