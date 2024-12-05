### `slice_string()` {#fn_slice_string}

函数原型：`fn slice_string(name: str, start: int, end: int, step: int = 1) -> str`

函数说明：返回字符串从索引 start 到 end 的子字符串，支持负数索引和自动调整范围，并且可以指定步长。

函数参数：

- `name`: 要截取的字符串
- `start`: 子字符串的起始索引（包含）
- `end`: 子字符串的结束索引（不包含）
- `step`: 步长，可选参数，默认为 1，支持负数步长

示例：

```python
substring = slice_string("15384073392", 0, 3) 
# substring 的值为 "153" 
substring2 = slice_string("15384073392", 0, 100)
# substring2 的值为 "15384073392"
# 如果 start 或 end 超出字符串的范围，函数会自动调整到字符串的边界。
substring3 = slice_string("15384073392", -5, -1) 
# substring3 的值为 "7339"
# 负数索引表示从字符串末尾开始计算。
substring4 = slice_string("15384073392", 0, -1, 2)
# substring4 的值为 "13473"
substring5 = slice_string("15384073392", 9, 0, -2)
# substring5 的值为 "93085"
# 如果 step 为正数，则从 start 到 end 按步长截取。
# 如果 step 为负数，则从 start 到 end 按步长反向截取。
```