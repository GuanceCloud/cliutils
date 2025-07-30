// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTask(t *testing.T) {
	httpTask := HTTPTask{
		URL:    "http://testing-openway.dataflux.cn",
		Method: "GET",
		Task: &Task{
			Name:              "http-test",
			PostURL:           "http://testing-openway.dataflux.cn/api/v1/post",
			Region:            "cn-hangzhou",
			DFLabel:           "http-test",
			WorkspaceLanguage: "go",
			Frequency:         "1m",
			CurStatus:         "ok",
		},
	}

	b, _ := json.Marshal(httpTask)

	task, _ := NewTask(string(b), &httpTask)
	task.SetStatus("stop")
	task.SetUpdateTime(1234567890)
	task.SetAk("ak.......")
	task.SetDisabled(1)

	newask := HTTPTask{
		URL:    "http://testing-openway.dataflux.cn",
		Method: "GET",
		Task: &Task{
			Name:              "http-test",
			PostURL:           "http://testing-openway.dataflux.cn/api/v1/post",
			Region:            "cn-hangzhou",
			DFLabel:           "http-test",
			WorkspaceLanguage: "go",
			Frequency:         "1m",
			CurStatus:         "stop",
			UpdateTime:        1234567890,
			AK:                "ak.......",
			Disabled:          1,
		},
	}

	newb, _ := json.Marshal(newask)

	assert.Equal(t, task.String(), string(newb))
}

func TestCreateTaskChild(t *testing.T) {
	ct, err := CreateTaskChild("http")
	assert.NoError(t, err)
	assert.NotNil(t, ct)

	task, err := NewTask("", ct)
	assert.NoError(t, err)
	assert.NotNil(t, task)
}
