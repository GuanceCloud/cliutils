# 聚合和尾采样模块技术文档

## 1. 模块概述
### 1.1 这个模块是做什么的？
- 一句话说明：实时数据聚合 + 链路尾采样
- 解决什么问题：海量数据的实时计算和智能采样
- 在系统中的位置：数据处理层，介于采集和存储之间

### 1.2 核心功能
- 指标聚合：max/min/avg/count/sum/histogram/quantiles/count_distinct/stdev
- 尾采样：条件采样 + 概率采样
- 派生指标：从链路数据生成监控指标

## 2. 代码结构速览
### 2.1 目录结构
```
aggregate/
├── algo_*.go          # 各种算法实现
├── aggr_*.go         # 聚合相关
├── calculator.go      # 计算器接口
├── metricbase.go      # 基础结构
├── windows.go         # 滑动窗口实现
├── tail-sampling.go   # 尾采样实现
├── docs/             # 文档
└── *_test.go         # 测试文件
```

### 2.2 核心文件说明
- **必读文件**：calculator.go, windows.go, tail-sampling.go
- **算法文件**：algo_max.go, algo_avg.go 等（模式相同）
- **配置解析**：aggr.go, batch.proto

## 3. 核心概念（5分钟理解）
### 3.1 聚合三要素
1. **Calculator**：计算器接口，每个算法都要实现
2. **Window**：时间窗口，管理一组Calculator
3. **MetricBase**：基础数据结构，包含key/name/tags等

### 3.2 尾采样三要素
1. **SamplingPipeline**：采样规则管道
2. **TraceDataPacket**：Trace数据包
3. **DerivedMetric**：派生指标配置

## 4. 关键流程（代码怎么跑的）
### 4.1 聚合流程
```
数据点 → 规则匹配 → 创建Calculator → 放入对应Window → 窗口到期 → 执行Aggr() → 输出结果
```

### 4.2 尾采样流程
```
Trace数据 → PickTrace()分组 → 管道规则评估 → 决定保留/丢弃 → 生成派生指标
```

## 5. 如何添加新算法（以stdev为例）
### 5.1 步骤 checklist
- [ ] 在 batch.proto 的 AlgoMethod 枚举中添加 STDEV = 8
- [ ] 实现 algoStdev 结构体（参考 algo_max.go）
- [ ] 实现 Calculator 接口的所有方法
- [ ] 在 calculator.go 的 newCalculators() 中添加 case STDEV
- [ ] 编写测试（参考 aggr_test.go）

### 5.2 关键代码片段
```go
// 1. 结构体定义
type algoStdev struct {
    MetricBase
    data    []float64
    maxTime int64
}

// 2. 在switch中添加
case STDEV:
    calc := &algoStdev{
        data:        []float64{f64},
        maxTime:    ptwrap.Time().UnixNano(),
        MetricBase: mb,
    }
```

## 6. 配置系统（怎么配的）
### 6.1 配置结构
- **AggregatorConfigure**：顶层配置
- **AggregateRule**：单个规则
- **AggregationAlgo**：算法配置

### 6.2 配置示例（快速理解）
```toml
[[aggregate_rules]]
name = "示例规则"
[aggregate_rules.select]
category = "metric"
fields = ["response_time"]
[aggregate_rules.aggregate]
avg_time = { algorithm = "avg" }
p95_time = { algorithm = "quantiles", options = { percentiles = [0.95] } }
```

## 7. 数据结构关系图
```
Cache (按过期时间分桶)
    ↓
Windows (按用户token分组)
    ↓
Window (一个时间窗口)
    ↓
map[uint64]Calculator (按hash分组)
    ↓
Calculator (具体算法：max/min/avg...)
```

## 8. 并发和锁策略
### 8.1 三级锁结构
1. **Cache.lock**：保护 WindowsBuckets
2. **Windows.lock**：保护 IDs 和 WS 列表
3. **Window.lock**：保护 cache map

### 8.2 设计原则
- 细粒度锁，减少竞争
- 锁持有时间尽量短
- 使用 sync.Pool 复用对象

## 9. 性能考虑
### 9.1 内存优化
- Window 对象池化
- map 预分配容量
- 数据及时清理

### 9.2 计算优化
- 哈希计算复用
- 批量处理
- 惰性计算

---