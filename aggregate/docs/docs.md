# 聚合和尾采样配置和流程说明文档

**聚合：**
    多个采集器采集的指标、日志和 RUM 按照配置好的 tag 进行聚合，聚合后支持多种运算（max,min,avg,count,histogram,sum,等等）。
    将相同的指标聚合到一个算子中，日志提取指标和RUM提取指标也是一样道理。

**尾采样：**
    链路数据通过采集后按照hash值转发到尾采样器上，确保同一个 trace 落到同一个采样器中。
    采样规则大致可以归类：耗时长、有错误、包含特定的字段、字段等于某些特定值 等等。

TODO： **日志尾采样：**

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



