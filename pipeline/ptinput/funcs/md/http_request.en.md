### `http_request()` {#fn-http-request}

Function prototype: `fn http_request(method: str, url: str, headers: map) map`

Function description: Send an HTTP request, receive the response, and encapsulate it into a map

Function parameters:

- `method`: GET|POST
- `url`: Request path
- `headers`: Additional header，the type is map[string]string

Example:

```python
resp = http_request("GET", "http://localhost:8080/testResp")
resp_body = load_json(resp["body"])

add_key(abc, resp["status_code"])
add_key(abc, resp_body["a"])
```