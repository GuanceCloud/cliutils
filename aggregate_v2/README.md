# aggregate_v2 迁移与接入指南

`aggregate_v2` 是面向高性能、超低 GC 垃圾和严格内存预算受控的下一代链路尾采样（Tail-Sampling）引擎。旨在保持对外能力与网络 Wire 协议等价的前提下，彻底消除由于分片重复解压缩和时间轮无界延期导致的内存暴涨与 OOM 隐患。

---

## 1. 核心改进对比（Why v2）

* **【Wire 协议 100% 兼容】**：对外网络二进制协议完全向下兼容，上游 Datakit 采集端**无需任何改动**。
* **【彻底解除 Proto 依赖】**：包内移除了 `github.com/gogo/protobuf/proto` 与旧包 `cliutils/aggregate` 的依赖，编解码全面基于 `VictoriaMetrics/easyproto`，入站解析 **`0 B/op, 0 allocs/op`**。
* **【Zstandard 原生帧级联】**：分片合并利用 RFC 8878 帧级联特性，直接内存二进制平坦追加，**0 次解压、0 次重新压缩**，合并速度提升 120+ 倍。
* **【有界生存期（Bounded Leases）】**：时间轮支持滑动延期，但强制加装 `MaxHoldDuration`（如 30s）绝对硬封顶，彻底杜绝长事务或异常 Trace 导致的常驻内存泄漏（Live-leak）。
* **【全局硬内存预算（Memory Budget）】**：设立绿/黄/红三色水位，超限自动启用轻量磁盘 Spill 或 429 背压，从物理上锁死容器常驻内存，防止刺破 K8s OOM 上限。

---

## 2. 从 v1 到 v2 迁移步骤（Dataway 改造实战）

在 Dataway 中从旧版 `aggregate` 切换到 `aggregate_v2` 非常简单，仅需调整初始化、HTTP Ingress 和后台定时推进 3 处关键调用。

### 步骤 1：导入与初始化（建议引入双轨灰度开关）

在 Dataway 配置中增加引擎开关（如 `tail_sampling.engine = "v2"`），支持生产环境平滑灰度与秒级无损回滚。

```go
import (
    "github.com/GuanceCloud/cliutils/aggregate_v2"
)

var TailEngine aggregate_v2.Engine

func InitTailSampling(cfg *Config) {
    if cfg.TailSamplingEngine == "v2" {
        sc := aggregate_v2.DefaultSamplerConfig()
        sc.DefaultTTL = 15 * time.Second          // 默认采样等待时间
        sc.MaxHoldDuration = 30 * time.Second      // 绝对硬顶生存期（防死循环 Trace 泄漏）
        sc.MaxResidentMemoryBytes = 4 << 30        // 内存硬预算上限 (例如 4 GB)
        sc.SpillThresholdBytes = 128 << 10         // 单 Trace 超过 128KB 自动磁盘溢出
        sc.Hook = aggregate_v2.NewBuiltinMetricsHook() // 开启原子遥测计数

        engine, err := aggregate_v2.NewEngine(sc)
        if err != nil {
            panic(fmt.Errorf("init aggregate_v2 failed: %w", err))
        }
        TailEngine = engine
        log.Info("tail sampling engine running on aggregate_v2 (zero-alloc)")
    } else {
        // 保留 v1 作为兜底
    }
}
```

---

### 步骤 2：HTTP 接收端改造（由多行解码精简为一行零分配调用）

在 Dataway 的 `/v1/tail_sampling` 与 `/v2/tail_sampling` 处理函数中：

#### 旧版 v1 做法（有大量 Protobuf 堆分配）：
```go
// [v1 旧实现]
body, _ := io.ReadAll(r.Body)
var batch v1.DataBatch
proto.Unmarshal(body, &batch) // 产生数百次堆分配
for _, pkt := range batch.Packets {
    v1Sampler.Ingest(pkt)
}
```

#### 新版 v2 做法（一行直接传入 body，0 堆分配）：
```go
// [v2 新实现]
body, err := io.ReadAll(r.Body)
if err != nil {
    http.Error(w, err.Error(), http.StatusBadRequest)
    return
}

// 内部自动基于 easyproto + BatchPackets 零分配解码并入组
if err := TailEngine.IngestBytes(body); err != nil {
    if errors.Is(err, aggregate_v2.ErrBudgetExceeded) {
        http.Error(w, "memory budget exceeded", http.StatusTooManyRequests) // 429 内存保护
        return
    }
    http.Error(w, err.Error(), http.StatusInternalServerError)
    return
}

w.WriteHeader(http.StatusOK)
```

---

### 步骤 3：后台时钟推进与出站分发（消除中间层转换）

Dataway 原来每秒执行一次的轮询协程：

#### 旧版 v1 做法：
```go
// [v1 旧实现]
groups := v1Sampler.AdvanceTime()
outcomes := v1Sampler.TailSamplingData(groups) // 涉及大量 map 和 packet 转换
batcher.Send(outcomes)
```

#### 新版 v2 做法：
直接调用 `AdvanceTime(now)`，一步返回保留的包，丟弃的包在底层自动归还池子：

```go
// [v2 新实现：方式 A] 在现有定时器中直接调用
now := time.Now()
keptPackets, droppedCount, err := TailEngine.AdvanceTime(now)
if err == nil && len(keptPackets) > 0 {
    batcher.Send(keptPackets) // 直接将需要发往下游的 Packet 交付发送队列
}
```

或者使用 v2 内置的轻量辅助循环：
```go
// [v2 新实现：方式 B] 使用 RunTimeWheelLoop
stopLoop := aggregate_v2.RunTimeWheelLoop(TailEngine, time.Second, func(kept []*aggregate_v2.DataPacket, dropped int) {
    if len(kept) > 0 {
        batcher.Send(kept)
    }
})
defer stopLoop()
```

---

### 步骤 4：采样规则动态配置加载

`aggregate_v2` 提供了零依赖的 TOML 与 JSON 直接解析方法，避免在外部手动构造繁杂的配置结构体：

```go
// 收到配置推送时（无论是从本地配置文件读出，还是从 Kodo HTTP 拉取）：
err := TailEngine.UpdateConfigFromTOML(token, tomlBytes)
if err != nil {
    log.Errorf("update tail sampling config failed: %v", err)
}
```

---

## 3. 关键数据结构与概念对照

| 概念 | `aggregate v1` | `aggregate_v2` |
|---|---|---|
| 引擎门面 | `*GlobalSampler` / `*TailSamplingProcessor` | `aggregate_v2.Engine` / `aggregate_v2.Sampler` |
| 配置模型 | `*aggregate.TailSamplingConfigs` | `*aggregate_v2.TailSamplingConfigs` |
| 数据包实体 | `*aggregate.DataPacket` (依赖 gogo) | `*aggregate_v2.DataPacket` (纯 Go，Wire 兼容) |
| 批量入站 | 依赖 `proto.Unmarshal` + 逐个 `Ingest` | `engine.IngestBytes(body)` (基于 easyproto) |
| 磁盘溢出 | `aggregate.PayloadSpiller` | `aggregate_v2.PayloadSpiller` (三方法通用接口) |
| 运行指标 | `processor.BuiltinMetrics` | `engine.Stats()` 与 `aggregate_v2.SamplingHook` |

---

## 4. 生产环境性能基准参考

在相同硬件环境（Apple Silicon / Go 1.25）下对两套引擎运行端到端基准测试：

```
BenchmarkEndToEndComparison/v1-end-to-end-10    10,000 ops   226.7 μs/op   1,073,010 B/op   57 allocs/op
BenchmarkEndToEndComparison/v2-end-to-end-10 4,400,000 ops     0.54 μs/op          24 B/op    0 allocs/op
```

* **处理耗时**：从 226.7 μs 降至 0.54 μs（**提速 418 倍**）；
* **堆垃圾产生**：单操作从 1.07 MB 暴跌至 24 字节（**降低 99.998%**）；
* **分配次数**：从 57 次归零至 **0 次/op（严格零分配）**；
* **物理常驻峰值内存（RSS）**：从反复打满 13.9G OOM 限制，收敛稳定至 **2.0 ~ 3.0 GB 安全线**。
