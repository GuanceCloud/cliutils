// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPostScriptDo(t *testing.T) {
	// {"response":{"status_code":200,"header":{"header1":["value1","value2"]},"body":"body"},"vars":{}}
	script := `

	result["is_failed"] = true
	result["error_message"] = "error"

	body = load_json(response["body"])

	vars["token"] = body["token"]
	vars["header"] = response["header"]["header1"]

	`

	body := []byte(`{"token": "token"}`)

	resp := &http.Response{
		Header: http.Header{
			"header1": []string{"value1", "value2"},
		},
	}

	result, err := postScriptDo(script, body, resp)
	assert.NoError(t, err)

	assert.True(t, result.Result.IsFailed)

	assert.True(t, reflect.DeepEqual([]interface{}([]interface{}{"value1", "value2"}), result.Vars["header"]))
	assert.Equal(t, "token", result.Vars["token"])
}
