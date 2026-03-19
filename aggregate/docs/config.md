## 配置


## 指标配置

以下是指标配置 toml 和 json 都支持:

```toml
# Global default setting
# 全局默认设置
default_window = "10s"

# Define a list of aggregation rules.
# Rules are processed in order. The first rule that a data point matches
# will be used. Data that matches no rules can be configured to be
# dropped or passed through.
# 定义聚合规则列表，按顺序处理。数据点匹配的第一个规则将被使用。
# 未匹配任何规则的数据可以配置为丢弃或通过。
[[aggregate_rules]]
  # A human-readable name for this rule.
  # 规则的可读名称
  name = "aggregate_api_latency_metrics"
  window = "60s"  # <--- Per-rule override  # 规则级别的窗口覆盖

  # --- SELECTION & FILTERING ---
  # This section defines which data points this rule applies to.
  # All conditions must be met (implicit AND).
  # 此部分定义此规则适用于哪些数据点，所有条件必须满足（隐式AND）
  [aggregate_rules.select]
    # Select data of type "metric".
    # 选择类型为"metric"的数据
    category = "metric"
    measurements = ["xxx", "yyy", "reg:some.*"]
    # Further filter by the metric's name.
    # This could support glob patterns like "http.server.*".
    # 按指标名称进一步过滤，支持通配符模式如"http.server.*"
    metric_name = [ "http.server.duration", "reg:some.*" ]

  # --- GROUPING ---
  # Define which tags to keep and group the aggregation by.
  # All other tags will be dropped.
  # 定义要保留哪些标签并按此分组聚合，所有其他标签将被丢弃
  [aggregate_rules.group_by]
    tags = ["service.name", "http.route", "http.status_code"] # regex not allowed  # 不允许正则表达式

  # --- AGGREGATION ALGORITHMS ---
  # Define what to calculate for which fields.
  # The key is the NEW field name in the aggregated output.
  # The value specifies the source field and the algorithm.
  # 定义要为哪些字段计算什么，键是聚合输出中的新字段名，值指定源字段和算法
  [aggregate_rules.aggregate]
    # The 'value' field of the OTel histogram is its merged distribution.
    # OTel直方图的'value'字段是其合并的分布
    latency_dist = { source_field = "value", algorithm = "merge_histogram" }

    # We can also generate new tags based on the aggregation.
    # 我们还可以基于聚合生成新标签
    [aggregate_rules.aggregate.add_tags]
      "agg_version" = "2.1"


[[aggregate_rules]]
  name = "aggregate_nginx_access_logs"

  [aggregate_rules.select]
    category = "logging"
    # Select logs where the source is 'nginx' or others.
    # This implies your ingestion adds this metadata.
    # 选择源为'nginx'或其他来源的日志，这意味着您的摄取添加了此元数据
    measurements = ["nginx", "yyy", "reg:some.*"]
    # Only aggregate successful requests.
    # 仅聚合成功的请求
    condition = """
    { fields.status_code >= 200 AND fields.status_code < 300 }
    """

  [aggregate_rules.group_by]
    # Group by the HTTP host and the upstream service it routed to.
    # 按HTTP主机和其路由到的上游服务分组
    tags = ["fields.http_host", "fields.upstream_service"]

  [aggregate_rules.aggregate]
    # Calculate the total number of requests.
    # 计算请求总数
    request_count = { algorithm = "count" }
    # Sum the bytes sent.
    # 汇总发送的字节数
    total_bytes_sent = { source_field = "fields.body_bytes_sent", algorithm = "sum" }
    # Calculate p95 and p99 latency from the raw request_time field.
    # 从原始request_time字段计算p95和p99延迟
    latency_p95_ms = { source_field = "fields.request_time_ms", algorithm = "quantiles", options = { percentiles = [0.95] } }
    latency_p99_ms = { source_field = "fields.request_time_ms", algorithm = "quantiles", options = { percentiles = [0.99] } }
    # Count the number of unique client IPs.
    # 计算唯一客户端IP的数量
    unique_client_ips = { source_field = "fields.client_ip", algorithm = "count_distinct" }

    # Rename a field directly. The 'last' algorithm is perfect for this.
    # 直接重命名字段，'last'算法非常适合此操作
    service = { source_field = "fields.upstream_service", algorithm = "last" }

    [aggregate_rules.aggregate.add_tags]
      "aggregated_by" = "my-aggregator"


[[aggregate_rules]]
  name = "aggregate_rum_user_actions"

  [aggregate_rules.select]
    category = "RUM" # Real User Monitoring data  # 真实用户监控数据
    fields.action.type = "click"

  [aggregate_rules.group_by]
    tags = ["fields.application.id", "fields.view.name", "fields.device.type"]

  [aggregate_rules.aggregate]
    click_count = { algorithm = "count" }
    unique_users = { source_field = "fields.user.id", algorithm = "count_distinct" }


# --- DEFAULT ACTION ---
# What to do with data that doesn't match any of the rules above.
# 对不匹配上述任何规则的数据执行的操作
[action]
  # Can be "drop" or "passthrough"
  # 可以是"drop"或"passthrough"
  action = "passthrough"

```

有几点补充：

- [aggregate_rules.aggregate] 中 source_field 必须是 Field 不可以是 tag. 特别注意: logging 类型


## 尾采样

配置文件示例：


```toml
# The maximum time to hold a trace in memory before forcing a decision.
# This defines the "aggregation window" for this tenant's traces.
# 在强制做出决策之前在内存中保存trace的最大时间，定义此租户trace的"聚合窗口"
trace_ttl = "1m"  # 1 minute  # 1分钟

# ===================================================================
# SECTION 1: DERIVED METRICS (Span-to-Metrics)
# Generate metrics from 100% of traffic, regardless of whether
# the trace is eventually kept or dropped.
# ===================================================================
# 第1部分：派生指标（Span到指标转换）
# 从100%的流量生成指标，无论trace最终是否被保留或丢弃

# 1. Total Trace Duration (Histogram)
# "count trace duration total duration, there may be a histogram or summary metric"
# 1. 总Trace持续时间（直方图）
# "统计trace持续时间总时长，可能是一个直方图或摘要指标"
[[derived_metrics]]
  name = "trace.duration"
  type = "histogram"
  # What value to measure? 'trace.duration_ms' is calculated by the aggregator
  # 测量什么值？'trace.duration_ms'由聚合器计算
  value_source = "trace.duration_ms"

  # Buckets for the histogram (milliseconds)
  # 直方图的桶（毫秒）
  buckets = [10, 50, 100, 500, 1000, 5000, 10000]

  # Tags to attach to the metric.
  # Using 'root_span' allows grouping by the entry point of the request.
  # 附加到指标的标签，使用'root_span'允许按请求的入口点分组
  group_by = ["service", "resource", "http_status_code"] # only from root span  # 仅来自根span

# 2. Conditional Metric: Specific Business Event
# "for some conditon, we may create a time series point on exist trace data"
# 2. 条件指标：特定业务事件
# "对于某些条件，我们可以在现有trace数据上创建时间序列点"
[[derived_metrics]]
  name = "checkout.high_value.count"
  type = "counter"

  # Only generate this metric if the condition is true
  # 仅当条件为真时生成此指标
  condition = """
    { resource = '/api/checkout' AND cart_total > 1000 }
  """

  group_by = ["service"] # only from root span  # 仅来自根span

# 3. Error Counter
# 3. 错误计数器
[[derived_metrics]]
  name = "trace.error.count"
  type = "counter"
  # Generate if ANY span in the trace has an error
  # 如果trace中的任何span有错误，则生成
  condition = """
    { error = true }
  """
  group_by = ["service"] # only from root span  # 仅来自根span


# ===================================================================
# SECTION 2: SAMPLING PIPELINE (Tail-Based Sampling)
# Rules are evaluated in order. The first rule that returns a decision
# (Keep or Drop) wins. If a rule returns 'Next', it continues.
# ===================================================================
# 第2部分：采样管道（基于尾部的采样）
# 规则按顺序评估，返回决策（保留或丢弃）的第一个规则获胜。
# 如果规则返回'Next'，则继续评估。

# Rule 1: Safety Net - Always keep errors
# "if there are errors, always keep the trace"
# 规则1：安全网 - 始终保留错误
# "如果有错误，始终保留trace"
[[sampling_pipeline]]
  name = "keep_errors"
  type = "condition"
  condition = """ { error = true } """
  action = "keep"

# Rule 2: Keep Slow Traces (Performance Bottlenecks)
# "for some condition, we may keep/drop single trace"
# 规则2：保留慢速Trace（性能瓶颈）
# "对于某些条件，我们可能保留/丢弃单个trace"
[[sampling_pipeline]]
  name = "keep_slow_traces"
  type = "condition"
  # Keep traces taking longer than 5 seconds
  # 保留持续时间超过5秒的trace
  condition = """ { trace.duration_ms > 5000 } """
  action = "keep"

# Rule 3: Drop Health Checks (Noise Reduction)
# "for some condition, we may keep/drop single trace"
# 规则3：丢弃健康检查（减少噪音）
# "对于某些条件，我们可能保留/丢弃单个trace"
[[sampling_pipeline]]
  name = "drop_health_checks"
  type = "condition"
  condition = """ { resource = '/healthz' OR resource = '/ping' }"""
  action = "drop"

# Rule 4: Business Logic Retention
# 规则4：业务逻辑保留
[[sampling_pipeline]]
  name = "keep_vip_customers"
  type = "condition"
  # Assuming you have an attribute for user tier
  # 假设您有用户层级的属性
  condition = """ { user_tier = 'gold' } """
  action = "keep"

# Rule 5: Default Probabilistic Sampling
# "configure a tail sampling rate... default rule for all traces"
# 规则5：默认概率采样
# "配置尾部采样率...所有trace的默认规则"
[[sampling_pipeline]]
  name = "default_global_sample"
  type = "probabilistic"
  # Keep 10% of the remaining traffic
  # 保留剩余流量的10%
  rate = 0.1
  # Deterministic hashing ensures consistent decisions based on TraceID
  # 确定性哈希确保基于TraceID的一致决策
  hash_key = "trace_id"

```
