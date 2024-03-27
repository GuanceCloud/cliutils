### `http_request()` {#fn-http-request}

函数原型: `fn http_request(method: str, url: str, headers: map) map`

函数说明: 发送HTTP请求，接收响应并封装成map

参数：

- `method`: GET|POST
- `url`: 请求路径
- `headers`: 附加的header，类型为map[string]string

示例：

```python
resp = http_request("GET", "http://localhost:8080/testResp")
resp_body = load_json(resp["body"])

add_key(abc, resp["status_code"])
add_key(abc, resp_body["a"])
```