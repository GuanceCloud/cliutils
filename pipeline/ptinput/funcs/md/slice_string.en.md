### `slice_string()` {#fn_slice_string}

Function prototype: `fn slice_string(name: str, start: int, end: int, step: int = 1) -> str`

Function description: Returns the substring of the string from the index start to end, supporting negative indices and automatic range adjustment, and allowing the specification of a step.

Function Parameters:

- `name`: The string to be sliced
- `start`: The starting index of the substring (inclusive)
- `end`: The ending index of the substring (exclusive)
- `step`: The step, optional parameter, default is 1, supports negative steps

Example:

```python
substring = slice_string("15384073392", 0, 3) 
# substring is "153" 
substring2 = slice_string("15384073392", 0, 100)
# substring2 is "15384073392"
# If `start` or `end` exceeds the range of the string, the function will automatically adjust to the boundaries of the string.
substring3 = slice_string("15384073392", -5, -1) 
# substring3 is "7339"
# Negative indices indicate counting from the end of the string.
substring4 = slice_string("15384073392", 0, -1, 2)
# substring4 is "13473"
substring5 = slice_string("15384073392", 9, 0, -2)
# substring5 is "93085"
# If `step` is positive, it slices from `start` to `end` with the specified step.
# If `step` is negative, it slices from `start` to `end` in reverse order with the specified step.
```