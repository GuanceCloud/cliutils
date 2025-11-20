// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostScriptDoGRPC(t *testing.T) {
	t.Run("success - extract message field", func(t *testing.T) {
		script := `
body = load_json(response["body"])
vars["message"] = body["message"]
result["is_failed"] = false
		`

		body := []byte(`{"message":"你好, test! 这是来自 gRPC 的问候"}`)

		result, err := postScriptDoGRPC(script, body)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Result.IsFailed)
		assert.Equal(t, "你好, test! 这是来自 gRPC 的问候", result.Vars["message"])
	})

	t.Run("success - extract multiple fields", func(t *testing.T) {
		script := `
body = load_json(response["body"])
vars["message"] = body["message"]
vars["status"] = body["status"]
result["is_failed"] = false
		`

		body := []byte(`{"message":"hello","status":"ok"}`)

		result, err := postScriptDoGRPC(script, body)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Result.IsFailed)
		assert.Equal(t, "hello", result.Vars["message"])
		assert.Equal(t, "ok", result.Vars["status"])
	})

	t.Run("failure - missing required field", func(t *testing.T) {
		script := `
body = load_json(response["body"])
if body["message"] != nil {
  vars["message"] = body["message"]
  result["is_failed"] = false
} else {
  result["is_failed"] = true
  result["error_message"] = "响应中缺少 message 字段"
}
		`

		body := []byte(`{"status":"ok"}`)

		result, err := postScriptDoGRPC(script, body)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Result.IsFailed)
		assert.Equal(t, "响应中缺少 message 字段", result.Result.ErrorMessage)
	})

	t.Run("failure - custom error", func(t *testing.T) {
		script := `
result["is_failed"] = true
result["error_message"] = "custom error message"
		`

		body := []byte(`{"message":"hello"}`)

		result, err := postScriptDoGRPC(script, body)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.True(t, result.Result.IsFailed)
		assert.Equal(t, "custom error message", result.Result.ErrorMessage)
	})

	t.Run("empty script", func(t *testing.T) {
		script := ""

		body := []byte(`{"message":"hello"}`)

		result, err := postScriptDoGRPC(script, body)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("nil body", func(t *testing.T) {
		script := `
vars["test"] = "value"
result["is_failed"] = false
		`

		result, err := postScriptDoGRPC(script, nil)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})

	t.Run("invalid JSON in response body", func(t *testing.T) {
		script := `
body = load_json(response["body"])
if body != nil {
  vars["message"] = body["message"]
  result["is_failed"] = false
} else {
  result["is_failed"] = true
  result["error_message"] = "invalid JSON"
}
		`

		body := []byte(`invalid json`)

		result, _ := postScriptDoGRPC(script, body)
		assert.NotNil(t, result)
	})

	t.Run("complex nested JSON", func(t *testing.T) {
		script := `
body = load_json(response["body"])
vars["user_name"] = body["user"]["name"]
vars["user_age"] = body["user"]["age"]
result["is_failed"] = false
		`

		body := []byte(`{"user":{"name":"test","age":25},"status":"ok"}`)

		result, err := postScriptDoGRPC(script, body)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.False(t, result.Result.IsFailed)
		assert.Equal(t, "test", result.Vars["user_name"])
		assert.Equal(t, float64(25), result.Vars["user_age"]) // JSON 数字会被解析为 float64
	})
}
