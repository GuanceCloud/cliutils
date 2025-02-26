### `pl_settings()` {#fn-pl-settings}

函数原型：`fn pl_settings(status_mappings: bool = true)`

函数说明：修改 Pipeline 的设置

函数参数：

- `status_mapping`: 设置日志类数据的 `status` 字段的映射功能，默认开启

示例：

```py
add_key("status", "warn")

# 使 status 字段的结果为 warn，而不是 warning
pl_settings(false)
```
