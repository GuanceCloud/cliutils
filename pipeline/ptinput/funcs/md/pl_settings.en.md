### `pl_settings()` {#fn-pl-settings}

Function prototype: `fn pl_settings(status_mappings: bool = true)`

Function description: Modify Pipeline settings

Function parameters:

- `status_mapping`: Set the mapping function of the `status` field of log data, enabled by default

Example:

```py
add_key("status", "warn")

# Make the result of the status field warn instead of warning
pl_settings(false)
```
