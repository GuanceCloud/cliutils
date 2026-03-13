# 聚合和尾采样配置和流程说明文档

**聚合：**
    多个采集器采集的指标、日志和 RUM 按照配置好的 tag 进行聚合，聚合后支持多种运算（max,min,avg,count,histogram,sum,等等）。
    将相同的指标聚合到一个算子中，日志提取指标和RUM提取指标也是一样道理。

**尾采样：**
    链路数据通过采集后按照hash值转发到尾采样器上，确保同一个 trace 落到同一个采样器中。
    采样规则大致可以归类：耗时长、有错误、包含特定的字段、字段等于某些特定值 等等。
    尾采样包括：链路，日志和RUM类型。


> 配置文件在： ./config.md

## 聚合

中心配置略过

配置通过 Kodo 查询和下发，由 DataKit 发起并携带 Token 请求。

DataKit:

- 定时发送配置请求并同步到本地
- io 模块收到指标/日志后，根据聚合配置，将数据分成两部分，一部分要聚合，一部分不用聚合，要聚合的数据，按照用户的聚合配置，在 point 中要标记聚合特征（哪些字段参与计算、每个字段的聚合方式、聚合 tag 是哪些）。同时，发给 dw 时，在 HTTP header 上标记 X-Aggregate: true

DataWay：

DW 有两种工作模式：proxy,standalone

- proxy: 收到请求转发到下一个standalone处理器上。
- standalone：存储到时间轮上，每个窗口上存储着一并到期的指标。

时间到期之后将算子转成 point 并发送到 Kodo.

### 聚合指标 point 的生命周期

select point（AggregatorConfigure.PickPoints）：

1. 指标按照配置的 select 中 category，measurements，metric_name 拆分成一个或多个指标，因为一个 point 中可能包含多个匹配到的field
2. 这里不局限于 metric logging rum。都可以pick。但是，metric_name必须存在于 point.filed 中，是不是float无所谓。
3. 拆分出来的每个point计算 pickKey: 根据 name groupBy 进行 hash
4. 根据 hash 值组装成 `map[uint64]*Batchs` 使用每一个hash值计算endpoint后发送。
5. 每一个 AggregationBatch 包括：hash pickHash 算子 point。
6. dataway 收到batch包序列化成对象，按照算子的算法添加到Windows中，如果已经存在同样hash的point,则进行计算后丢弃。
7. 窗口时间到，转成point,发送到kodo.

---
### 滑动窗口实现的技术说明：

****核心设计：

这是一个时间分桶的滑动窗口聚合系统，用于按时间窗口对指标数据进行实时聚合计算。

关键技术点

1. 三级缓存结构：
    - Cache → Windows → Window → Calculator
    - 按过期时间分桶，每个桶内按用户token分组

2. 时间窗口管理：
    - 每个计算器(Calculator)有nextWallTime（窗口对齐时间）
    - 加上容忍时间(Expired)确定最终过期时间
    - 定期清理过期窗口

3. 内存优化：
    - 使用sync.Pool复用Window对象
    - 预分配map容量减少扩容开销
    - 惰性删除保留哈希桶空间

4. 并发安全：
    - 每层都有独立的锁保护
    - Window级别细粒度锁减少竞争

**工作流程：**

1. 添加数据：AddBatch() → 计算过期时间 → 分配到对应时间桶 → 用户分组 → 聚合计算
2. 清理过期：GetExpWindows() 获取已过期窗口 → 生成聚合结果 → 释放资源
3. 结果输出：WindowsToData() 将窗口数据转换为点数据，按用户token分组输出
---


## 尾采样

配置：

通过配置可以看到，都是通过 Conditions 进行筛选。最后使用通用的采样率进行兜底。


设计由 **Datakit（采集层）**、**Dataway（传输层）** 与 **Sampler（决策层）** 组成，旨在解决全量链路存储成本高与异常发现率低的矛盾。

**核心流程：**

1. **数据采集与路由**：Datakit 捕获原始 Span，通过 gRPC Stream 实时推送至采样服务。系统基于 `TraceID` 进行一致性哈希，确保同一链路的所有片段汇聚至同一采样分片（Shard）。
2. **延迟缓冲（时间轮）**：利用 **3600 槽位时间轮** 维护内存缓存。数据进入 `activeMap` 后等待 5 分钟（配置可选），以确保分布式环境下链路片段的完整性。
3. **规则引擎决策**：当链路过期触发决策时，执行 **三态（Keep/Drop/Undecided）** 匹配：
   * **静态规则**：优先匹配 `hasError` 或特定 HTTP 状态码。
   * **自定义规则**：检索 `Point` 属性中的 Key-Value（如 `resource`）。
   * **概率采样**：对未命中上述规则的正常链路执行百分比截流。

4. **高效分发**：命中保留规则的数据经 **Token 聚合** 后，由 **Worker Pool** 异步并发推送到后座存储（Kodo）。

**技术优势**

* **零拷贝与对象池**：引入 `sync.Pool` 管理 `TraceData`，减少高并发下的 GC 压力。
* **非阻塞设计**：时间轮驱动与发送任务解耦，保证采样精度不受网络波动影响。
* **确定性采样**：基于 `TraceIDHash` 的计算确保了链路在采样过程中的完整性。

---

### 尾采样指标派生

指标派生（Derived Metrics）是尾采样的重要功能，它允许从原始链路、日志和RUM数据中生成新的监控指标，无论数据最终是否被采样保留。这些派生指标提供了对系统行为的量化洞察。

#### 1. 派生指标的核心概念

**为什么需要派生指标？**
- 从100%的原始数据中提取关键业务指标
- 即使trace被丢弃，关键指标仍被保留
- 提供实时监控和告警的基础数据

**派生指标的类型：**
- **计数器（Counter）**：统计事件发生次数，如错误数、请求数
- **直方图（Histogram）**：记录数值分布，如响应时间分布
- **摘要（Summary）**：计算百分位数，如P95、P99延迟

#### 2. 派生指标配置结构

派生指标配置支持多种数据源类型：

**2.1 链路（Trace）派生指标**
```toml
[[trace.derived_metrics]]
name = "trace_duration"
condition = ""  # 可选：过滤条件
group_by = ["service", "resource"]  # 分组标签

[[trace.derived_metrics.aggregate]]
method = "HISTOGRAM"
source_field = "$trace_duration"  # 特殊变量：trace持续时间

[[trace.derived_metrics.aggregate.histogram_opts]]
buckets = [10000, 50000, 100000, 500000, 1000000, 5000000, 10000000]
```

**2.2 日志（Logging）派生指标**
```toml
[[logging.derived_metrics]]
name = "trace_error"
condition = "{status=\"error\"}"  # 条件过滤
group_by = ["service", "resource"]

[[logging.derived_metrics.aggregate]]
method = "COUNT"
source_field = "status"
```

**2.3 RUM派生指标**
```toml
[[rum.derived_metrics]]
name = "page_load_time"
condition = "{ event = \"page_load\" }"
group_by = ["page_url", "device_type"]

[[rum.derived_metrics.aggregate]]
method = "HISTOGRAM"
source_field = "load_time_ms"
```

#### 3. 支持的聚合方法

| 方法 | 描述 | 适用场景 |
|------|------|----------|
| **COUNT** | 计数 | 统计事件发生次数 |
| **HISTOGRAM** | 直方图 | 数值分布分析 |
| **SUM** | 求和 | 累计值统计 |
| **AVG** | 平均值 | 平均性能指标 |
| **MAX/MIN** | 最大/最小值 | 极值监控 |
| **QUANTILES** | 百分位数 | P95/P99延迟 |

#### 4. 条件过滤语法

派生指标支持灵活的过滤条件：

```toml
# 基本条件
condition = "{ status = \"error\" }"

# 组合条件
condition = "{ status = \"error\" AND service = \"api\" }"

# 数值比较
condition = "{ response_time > 1000 }"

# IN操作符
condition = "{ resource IN [\"/healthz\", \"/ping\"] }"

# 特殊变量
condition = "{ $trace_duration > 5000000 }"  # trace持续时间>5秒
```

#### 5. 分组维度配置

派生指标支持按不同维度分组生成指标：

```toml
# 按用户ID分组
[[logging.group_dimensions]]
group_key = "user_id"

# 用户ID分组的派生指标
[[logging.group_dimensions.derived_metrics]]
name = "user_error_count"
condition = "{ level = \"error\" }"
group_by = ["user_id", "service"]

[[logging.group_dimensions.derived_metrics.aggregate]]
method = "COUNT"
source_field = "level"
```

#### 6. 实际应用场景

**场景1：API性能监控**
```toml
[[trace.derived_metrics]]
name = "api_response_time"
condition = "{ resource LIKE \"/api/*\" }"
group_by = ["service", "resource", "http_method"]

[[trace.derived_metrics.aggregate]]
method = "HISTOGRAM"
source_field = "$trace_duration"

[[trace.derived_metrics.aggregate.histogram_opts]]
buckets = [10, 50, 100, 500, 1000, 5000]
```

**场景2：业务错误监控**
```toml
[[logging.derived_metrics]]
name = "business_error_rate"
condition = "{ error_type = \"business_error\" }"
group_by = ["service", "error_code"]

[[logging.derived_metrics.aggregate]]
method = "COUNT"
source_field = "error_type"
```

**场景3：用户体验监控**
```toml
[[rum.derived_metrics]]
name = "user_interaction_time"
condition = "{ event_type = \"click\" }"
group_by = ["page_url", "element_id"]

[[rum.derived_metrics.aggregate]]
method = "HISTOGRAM"
source_field = "interaction_time_ms"
```

#### 7. 技术实现要点

1. **实时计算**：派生指标在数据到达时实时计算，不依赖采样决策
2. **内存高效**：使用聚合计算器（Calculator）减少内存占用
3. **时间窗口**：按配置的时间窗口聚合指标数据
4. **异步输出**：派生指标异步输出到监控系统

#### 8. 内置派生指标

系统提供了一系列内置派生指标，可以直接在配置中使用：

**错误监控指标：**
- `trace_error_count`：错误链路个数统计
- `span_error_count`：错误Span个数统计  
- `trace_error_rate`：错误链路率（百分比）

**性能监控指标：**
- `trace_duration_summary`：链路耗时统计摘要（P50/P95/P99）
- `slow_trace_count`：慢链路个数统计（>5秒）
- `trace_size_distribution`：链路大小分布（Span数量）

**流量监控指标：**
- `trace_total_count`：总链路条数统计
- `span_total_count`：总Span个数统计

**使用示例：**
```go
// 获取所有内置派生指标
metrics := aggregate.GetBuiltinDerivedMetrics()

// 获取特定内置指标
traceErrorCount := aggregate.GetBuiltinMetric("trace_error_count")

// 检查是否为内置指标
if aggregate.IsBuiltinMetric("trace_total_count") {
    // 是内置指标
}
```

详细使用示例请参考：`builtin_derived_metrics_example.md`

#### 9. 最佳实践

1. **明确业务目标**：根据监控需求设计派生指标
2. **合理分组**：避免分组维度过多导致指标爆炸
3. **条件优化**：使用精确的条件过滤减少不必要的计算
4. **桶配置**：根据业务特点合理设置直方图桶边界
5. **监控告警**：基于派生指标设置合理的告警阈值
6. **利用内置指标**：优先使用系统提供的内置派生指标



