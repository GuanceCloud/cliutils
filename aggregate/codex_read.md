# aggregate 模块阅读笔记

本文档是我在完整阅读 `aggregate/` 目录代码、测试和 `docs/` 文档后的理解整理。目标不是重复注释，而是给以后继续读这块代码时提供一个高密度、可回溯的知识入口。

## 1. 模块在做什么

`aggregate` 同时承载两类能力：

1. 指标聚合
2. trace/logging/RUM 的尾采样，以及从这些数据里生成派生指标

这两类能力都围绕“先按规则分组，再延迟/聚合，再输出结果”展开，但它们的执行载体不同：

- 指标聚合依赖 `Cache -> Windows -> Window -> Calculator`
- 尾采样依赖 `GlobalSampler -> Shard -> time wheel(3600 slots) -> DataGroup`

---

## 2. 目录怎么读

### 2.1 核心实现

- `aggr.go`
  规则配置、点筛选、分组、批次构建。
- `calculator.go`
  `Calculator` 接口、`AlignNextWallTime()`、`newCalculators()` 工厂。
- `windows.go`
  聚合缓存结构，按过期时间和 token 管理窗口。
- `metricbase.go`
  各类聚合算子的公共元数据。
- `tail-sampling.go`
  尾采样配置、数据分组、采样管道、预置规则。
- `timewheel.go`
  全局尾采样器、时间轮缓存、到期决策入口。
- `derived_metric_generator.go`
  从 `DataPacket` 生成派生指标点。
- `builtin_derived_metrics.go`
  内置派生指标定义。

### 2.2 算法实现

- `algo_sum.go`
- `algo_avg.go`
- `algo_count.go`
- `algo_min.go`
- `algo_max.go`
- `algo_histogram.go`
- `algo_quantiles.go`
- `aggr_count_distinct.go`
- `aggr_stdev.go`

### 2.3 协议和生成代码

- `aggrbatch.proto` / `aggrbatch.pb.go`
  聚合批次和算法配置协议。
- `tsdata.proto` / `tsdata.pb.go`
  尾采样 `DataPacket` 协议。
- `pb.sh`
  `proto` 生成脚本。

### 2.4 测试和文档

- `*_test.go`
  行为样例和边界说明，很多真实语义要以测试为准。
- `docs/*.md`
  设计说明、配置示例、派生指标示例。

---

## 3. 先建立两个心智模型

### 3.1 指标聚合心智模型

流程：

`原始 point -> RuleSelector 筛选/拆分 -> AggregateRule 分组 -> AggregationBatch -> newCalculators -> Cache/Windows 存储 -> 到期后 Aggr() -> 输出聚合 point`

关键点：

- 一个原始 point 可能被拆成多个只含单个 field 的 point。
- group by 决定聚合粒度。
- 算法配置决定每个 field 生成哪种 `Calculator`。
- 聚合结果不是马上输出，而是对齐到窗口边界后等待到期。

### 3.2 尾采样心智模型

流程：

`原始 trace/logging/RUM points -> PickTrace / PickLogging / PickRUM 分组为 DataPacket -> Ingest 进入时间轮 -> TTL 到期 -> TailSamplingData() 做规则决策 -> 保留部分 DataPacket + 生成派生指标`

关键点：

- 尾采样不是立即决策，而是等待一个 TTL，让同组数据尽量收齐。
- group key 对 trace 是 `trace_id`，对 logging/RUM 可配置。
- 采样和派生指标是两条逻辑线：理论上派生指标可独立于最终是否保留。

---

## 4. 指标聚合模块怎么工作

## 4.1 配置入口

顶层配置是 `AggregatorConfigure`，核心字段：

- `DefaultWindow`
- `AggregateRules`
- `DefaultAction`
- `DeleteRulesPoint`

每条规则是 `AggregateRule`：

- `Name`
- `Selector`
- `Groupby`
- `Algorithms`

`Selector` 负责选点：

- `Category`
- `Measurements`
- `MetricName`
- `Condition`

`Setup()` 会做这些事：

- 校验 `Category`
- 解析条件表达式
- 编译 measurement / field 白名单
- 给算法补默认窗口
- 对 `Groupby` 排序，保证 hash 稳定

## 4.2 点的筛选和拆分

`RuleSelector.doSelect()` 是理解聚合入口的关键：

1. 按 measurement 白名单过滤
2. 按 `Condition` 过滤
3. 用 `selectKVS()` 把一个 point 拆成多个只有单 field 的新 point
4. 把 `group_by` 中声明的 tag 附着到新 point 上
5. 对 histogram 这类需要 `le`/`bucket` 的情况，也会把 field 带过去

这个设计意味着：

- 聚合系统本质上希望“一个聚合点只有一个主要 field”
- `source_field` 必须来自 field，不是 tag
- logging/RUM 也能进入聚合，只要规则能选出来

## 4.3 分组和路由 hash

有两个 hash 概念：

- `hash(pt, groupby)`
  用 measurement + group by tag 值 + 第一个非 tag field 计算，决定真正的聚合实例。
- `pickHash(pt, groupby)`
  用 measurement + group by tag key 计算，不带 tag value，作为批次 pick key。

我的理解：

- `RoutingKey` 用于把同一聚合实例尽量送到同一节点。
- `PickKey` 用于把同一类聚合需求打包。

## 4.4 Calculator 工厂

`newCalculators()` 根据 `AggregationBatch.AggregationOpts` 生成具体算子实例。

目前代码真正创建实例的算法有：

- `SUM`
- `AVG`
- `COUNT`
- `MIN`
- `MAX`
- `HISTOGRAM`
- `QUANTILES`
- `STDEV`
- `COUNT_DISTINCT`

协议里虽然也有：

- `EXPO_HISTOGRAM`
- `LAST`
- `FIRST`

但在 `newCalculators()` 里仍是 `TODO`，当前没有落地实现。

## 4.5 窗口与缓存结构

`windows.go` 的结构：

- `Cache`
  `WindowsBuckets map[int64]*Windows`
- `Windows`
  一个过期时间桶里的多租户集合
- `Window`
  某个 token 的一个窗口，内部是 `map[uint64]Calculator`

工作方式：

1. `AddBatch()` 通过 `newCalculators()` 得到算子
2. 对每个算子，用 `nextWallTime + Expired` 计算过期时间
3. `Cache.GetAndSetBucket()` 按过期时间桶存进去
4. `GetExpWidows()` 扫描到期桶并取出所有 `Window`
5. `WindowsToData()` 调每个 `Calculator.Aggr()` 输出点

这里的 `Expired` 是一个容忍延迟，用于等待可能迟到的数据。

## 4.6 聚合结果长什么样

各算法最终都返回 `[]*point.Point`，输出字段规律大致是：

- 主结果字段：通常是算法目标字段名
- 统计字段：通常额外带一个 `<field>_count`
- 标签：原 point 的 tag + 算法配置中的 `AddTags`

`algo_histogram` 特殊一些：

- 每个桶会输出一个带 `le` tag 的点
- 另外还会输出一个仅带 `<field>_count` 的点

---

## 5. 已实现算法的真实语义

### 5.1 普通数值算法

- `algoSum`
  累加数值。
- `algoAvg`
  累加后求均值。
- `algoMin`
  求最小值。
- `algoMax`
  求最大值。
- `algoCount`
  只计数，不看原字段值。
- `algoStdev`
  计算样本标准差，样本数不足 2 会报错。

### 5.2 分布型算法

- `algoHistogram`
  依赖原点中的 `le` 标签和值来合并 bucket。
- `algoQuantiles`
  收集全部值，排序后按百分位求值。
- `algoCountDistinct`
  用 `map[any]struct{}` 保持去重值集合。

### 5.3 算法实现里值得记住的代码现实

这些是后续继续改这里时必须防守的点：

1. `COUNT/AVG/SUM/MIN/MAX` 的 `count` 初始值普遍从 0 开始，只在 `Add()` 时递增。
   这意味着单样本窗口的 `_count` 很可能是 0，`algoAvg` 甚至存在单样本除零风险。这看起来更像当前实现风险，而不是设计目标。

2. `algoCount.doHash()` 里使用的是 `"avg"` 作为算法名参与 hash，而不是 `"count"`。
   这意味着 `COUNT` 和 `AVG` 的 hash 可能发生冲突，是一个应被记住的实现问题。

3. `algoHistogram.Add()` 里对已有 bucket 累加时使用了 `c.val`，而不是 `inst.val`。
   当前行为需要仔细复核，读代码时不能直接假设 histogram 合并逻辑完全正确。

4. `EXPO_HISTOGRAM/LAST/FIRST` 还没实现，不要只看 proto 以为它们已可用。

---

## 6. 尾采样模块怎么工作

## 6.1 尾采样数据模型

协议定义在 `tsdata.proto`，核心结构是 `DataPacket`：

- `group_id_hash`
- `raw_group_id`
- `token`
- `source`
- `data_type`
- `config_version`
- `has_error`
- `group_key`
- `point_count`
- `first_seen_nano`
- `last_seen_nano`
- `points`

这是一个通用包，不只服务 trace，也服务 logging / RUM。

## 6.2 数据怎么先被分组

### Trace

`PickTrace()`：

- 读取 `trace_id`
- `hashTraceID()` 做 group hash
- 把同 trace 的 points 合成一个 `DataPacket`
- 检查 tag `status=="error"` 来设置 `HasError`
- `PointCount++`

### Logging / RUM

`pickByGroupKey()`：

- 按传入的 `groupKey` 取字段值
- 值缺失或无法转字符串的点直接进入 `passedThrough`
- 能分组的点会组装成 `DataPacket`

因此 logging / RUM 的一个重要语义是：

- 不是所有数据都会进入尾采样
- 没有分组键的数据天然旁路

## 6.3 尾采样配置

顶层是 `TailSamplingConfigs`：

- `Tracing`
- `Logging`
- `RUM`
- `Version`

初始化逻辑在 `Init()`：

- 默认 TTL
  - trace: 5m
  - logging: 1m
  - RUM: 1m
- trace 默认 `group_key = trace_id`
- 对每条 pipeline 调 `Apply()` 解析条件

要注意：

- `LoggingTailSampling` 本身没有顶层 `DerivedMetrics` 字段，派生指标是挂在每个 `LoggingGroupDimension` 上的。
- `RUMTailSampling` 既有顶层 `DerivedMetrics`，也有每个 group dimension 的 `DerivedMetrics`。
  但当前 `TailSamplingData()` 对 RUM 走的是 group dimension 级派生指标。

## 6.4 采样管道

`SamplingPipeline` 关键字段：

- `Type`
  - `condition`
  - `probabilistic`
- `Condition`
- `Action`
  - `keep`
  - `drop`
- `Rate`
- `HashKeys`

`DoAction()` 的真实语义：

1. 如果 `conds == nil`，直接返回 `(false, td)`
2. 如果配置了 `HashKeys`，只要任一 point 含这些 key，就立刻返回 `(true, td)`
3. 否则逐点执行条件表达式
4. 条件命中后：
   - `probabilistic` 根据 `GroupIdHash % 10000` 做确定性采样
   - `condition` 根据 `keep/drop` 返回结果

因此要记住几个非常实际的点：

1. 概率采样也依赖 `Condition` 已经解析成功。
   想做“默认采样”，要写类似 `{ 1 = 1 }` 这样的条件，而不是留空。

2. `HashKeys` 当前实现更像“存在这些字段就直接保留”，而不是“用这些字段参与 hash 决策”。
   这是代码语义，不要用文档想当然理解。

3. 采样结果是“第一条返回决定的 pipeline 生效”，后续 pipeline 不再执行。

## 6.5 时间轮缓存

`timewheel.go` 里的 `GlobalSampler` 是尾采样决策核心。

结构：

- `shards []*Shard`
- 每个 `Shard` 有
  - `activeMap map[uint64]*DataGroup`
  - `slots [3600]*list.List`
  - `currentPos`

`Ingest()` 做的事情：

1. 按 `GroupIdHash % shardCount` 选 shard
2. 按数据类型读取对应租户配置
3. 根据 `DataTTL` 计算秒级 TTL
4. 计算 `expirePos = (currentPos + ttlSec) % 3600`
5. 键使用 `HashToken(token, GroupIdHash)`，避免跨租户冲突
6. 如果该组已经存在：
   - 追加 points
   - 合并 `HasError`
   - 从旧槽移到新槽
7. 否则创建 `DataGroup` 并挂到目标槽

`AdvanceTime()` 每调用一次，时间轮前进一格，并返回当前槽位到期的全部 `DataGroup`。

这意味着：

- 时间轮最大只支持 3600 秒
- TTL 超过 3600 秒会被截到 3599
- 调度粒度是秒

---

## 7. 尾采样决策和派生指标

## 7.1 `TailSamplingData()` 做什么

对到期的 `DataGroup`：

- trace:
  - 取 trace config
  - 顺序执行 pipelines
  - 命中保留则写入返回的 `DataPacket` map
  - 调用 `handleDerivedMetrics()`
- logging:
  - 找到匹配 `GroupKey` 的维度配置
  - 执行 pipelines
  - 调用 `handleDerivedMetrics()`
- RUM:
  - 同 logging

最后统一：

- `dg.Reset()`
- 放回 `dataGroupPool`

## 7.2 派生指标生成器

入口：

- `GenerateDerivedMetrics(packet, metrics)`
- `MetricGenerator.GenerateFromDataPacket()`

执行顺序：

1. `evaluateCondition()`
2. `extractGroupTags()`
3. 按算法生成指标点

支持算法：

- `COUNT`
- `HISTOGRAM`
- `QUANTILES`
- `AVG`
- `SUM`
- `MIN`
- `MAX`
- `COUNT_DISTINCT`

特殊变量：

- `$trace_id`
- `$span_count`
- `$trace_duration`
- `$error_flag`

## 7.3 当前派生指标实现和设计文档的差异

这是本模块最重要的注意项之一：

1. `timewheel.go` 里虽然已经调用 `GenerateDerivedMetrics()`，但只打 debug log，后续发送仍是 `TODO`。
   结论：当前代码会“生成派生指标点对象”，但没有真正把这些点送入后续指标链路。

2. `$trace_duration` 的代码和文档单位不一致。
   `extractSourceValues()` 用 `(LastSeenNano - FirstSeenNano) / 1e6`，实际算出来是毫秒；但注释、文档和内置条件多处写的是微秒。

3. `evaluateCondition()` 是在单个 point 上跑 filter 条件，并不天然支持 `DataPacket` 级特殊变量。
   所以像 `{$trace_duration > 5000000}` 这类条件，从当前代码看很可能无法按预期工作。

4. `PickTrace()` / `pickByGroupKey()` 当前并没有填充 `FirstSeenNano` 和 `LastSeenNano`。
   如果外部没有补这两个字段，那么依赖 `$trace_duration` 的派生指标根本没有输入。

这几个点叠加起来，说明“派生指标的设计已经成形，但当前实现还没有完全闭环”。

---

## 8. 内置派生指标

`builtin_derived_metrics.go` 提供了一组预定义指标：

- `trace_error_count`
- `trace_total_count`
- `span_total_count`
- `trace_error_rate`
- `trace_duration_summary`
- `slow_trace_count`
- `span_error_count`
- `trace_size_distribution`

对应能力大致是：

- 错误数
- 总量
- span 数量
- 错误率
- 时延分位数
- 慢 trace 数
- span 错误数
- trace 大小分布

测试文件 `builtin_derived_metrics_test.go` 主要验证：

- 名称是否完整
- method / source_field / options 是否匹配预期
- group by 维度是否正确

---

## 9. 配置和协议层面应该记住的事

## 9.1 `aggrbatch.proto`

聚合算法枚举已经定义得比较全：

- `SUM`
- `AVG`
- `COUNT`
- `MIN`
- `MAX`
- `HISTOGRAM`
- `EXPO_HISTOGRAM`
- `STDEV`
- `QUANTILES`
- `COUNT_DISTINCT`
- `LAST`
- `FIRST`

但“协议支持”不等于“代码实现完毕”，这一点必须和 `newCalculators()` 对照着看。

## 9.2 `tsdata.proto`

`DataPacket` 已经被设计成跨 trace/logging/RUM 通用，因此后续扩展更多“按 group key 延迟决策”的数据类型是可行的。

## 9.3 `pb.sh`

当前 protobuf 使用 `gogoslick` 生成，脚本同时构建：

- `aggregate/aggrbatch.proto`
- `aggregate/tsdata.proto`

后续如果改协议，优先看这个脚本和生成包映射。

---

## 10. 测试告诉我们的真实边界

### 10.1 聚合侧

- `aggr_test.go`
  演示了：
  - 规则筛选
  - 批次构建
  - HTTP 发 batch
  - histogram 类型的聚合输入样式

- `windows_test.go`
  演示了：
  - `Cache` 的并发写入
  - 到期窗口提取
  - `WindowsToData()` 输出

- `calculator_test.go`
  主要验证窗口时间对齐逻辑。

- `algo_test.go`
  主要补了 `count_distinct` 的行为测试。

### 10.2 尾采样侧

- `tail-sampling_test.go`
  覆盖了：
  - `PickTrace()`
  - pipeline 条件保留/丢弃
  - hash key 行为
  - 概率采样
  - `TailSamplingConfigs.Init()`
  - logging 的按维度分组

- `timewheel_test.go`
  证明了时间轮在 TTL 秒数到达后会把数据吐出来。

- `derived_metric_generator_test.go`
  目前更偏“基础可用性测试”，覆盖不深，但至少说明这块逻辑已经被显式纳入测试。

---

## 11. 我认为最重要的几个代码事实

如果以后只记 10 件事，我会记这些：

1. 这是一个“双系统”目录，不只是指标聚合，也包含尾采样。
2. 聚合主链路核心是 `RuleSelector -> AggregationBatch -> Calculator -> Cache/Windows -> Aggr()`.
3. 尾采样主链路核心是 `Pick* -> DataPacket -> timewheel -> TailSamplingData()`.
4. logging / RUM 尾采样不是按 trace_id，而是按配置的 `group_key`.
5. logging / RUM 缺少分组键的数据会直接旁路，不进入采样缓存。
6. 尾采样的 pipeline 是顺序短路执行，第一条决定结果的规则获胜。
7. 当前派生指标还没有真正输出到后续处理链路，只是本地生成后打日志。
8. `$trace_duration` 相关实现现在存在单位和可用性问题，不能盲信文档。
9. 聚合算法里有几个实现层风险，尤其是 `count` 初值、`algoCount` hash、histogram 合并逻辑。
10. proto 里定义的算法不等于都已实现，落地能力要看 `newCalculators()`.

---

## 12. 以后继续读这块代码，建议顺序

我建议的阅读顺序是：

1. `aggregate/codex_read.md`
2. `aggregate/docs/config.md`
3. `aggregate/aggr.go`
4. `aggregate/calculator.go`
5. `aggregate/windows.go`
6. `aggregate/tail-sampling.go`
7. `aggregate/timewheel.go`
8. `aggregate/derived_metric_generator.go`
9. 对应 `*_test.go`

这样先建立总结构，再进实现，再看测试修正理解，成本最低。

---

## 13. 我对当前模块成熟度的判断

### 聚合模块

主体结构已经成型：

- 配置模型明确
- 路由和窗口管理明确
- 算法扩展点明确

但实现细节里仍有值得复核的地方，尤其是部分算法的计数语义。

### 尾采样模块

架构方向已经明确：

- 通用 `DataPacket`
- 多数据类型支持
- 时间轮缓存
- 规则驱动决策
- 派生指标能力

但派生指标链路目前还不算闭环，且 packet 级特殊变量支持不完整。

---

## 14. 一句话总结

`aggregate` 是一个把“规则筛选、延迟聚合、按租户隔离、分布式友好路由”这些能力揉在一起的模块；它的框架已经足够清晰，但尾采样派生指标和部分聚合算法实现上，仍然存在几个必须带着怀疑去读的代码现实。
