// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"errors"
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

func TestCustomFunc(t *testing.T) {
	ct := &HTTPTask{
		URL: `{{date "iso8601"}}`,
		SuccessWhen: []*HTTPSuccess{
			{
				StatusCode: []*SuccessOption{
					{
						Is: "200",
					},
				},
			},
		},
	}

	task, err := NewTask("", ct)

	assert.NoError(t, err)

	err = task.RenderTemplateAndInit(nil)

	assert.NoError(t, err)

	assert.NotNil(t, ct.rawTask)
	assert.NotEqual(t, ct.URL, ct.rawTask.URL)
}

func TestScheduleType(t *testing.T) {
	cases := []struct {
		name         string
		crontab      string
		frequency    string
		scheduleType string
		isValid      bool
	}{
		{
			name:         "cron task",
			crontab:      "*/1 * * * *",
			scheduleType: ScheduleTypeCron,
			isValid:      true,
		},
		{
			name:         "cron task with invalid crontab",
			crontab:      "*/1 * * * * *",
			scheduleType: ScheduleTypeCron,
			isValid:      false,
		},
		{
			name:         "frequency task",
			frequency:    "1m",
			scheduleType: ScheduleTypeFrequency,
			isValid:      true,
		},
		{
			name:         "frequency task with invalid frequency",
			frequency:    "1ss",
			scheduleType: ScheduleTypeFrequency,
			isValid:      false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ct := &HTTPTask{
				URL: `http://localhost:8000`,
				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{
								Is: "200",
							},
						},
					},
				},
				Task: &Task{
					Crontab:      c.crontab,
					Frequency:    c.frequency,
					ExternalID:   "123",
					ScheduleType: c.scheduleType,
				},
			}
			task, err := NewTask("", ct)
			assert.NoError(t, err)

			err = task.Check()
			if c.isValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestSetBeforeRun(t *testing.T) {
	for _, ct := range []TaskChild{
		&TCPTask{},
		&HTTPTask{},
		&ICMPTask{},
		&WebsocketTask{},
	} {
		task, err := NewTask("", ct)
		assert.NoError(t, err)

		errString := "before run error"
		task.SetBeforeRun(func() error {
			return errors.New(errString)
		})

		err = task.Run()
		assert.NoError(t, err)
		tags, fields := task.GetResults()
		assert.Equal(t, "FAIL", tags["status"])
		assert.Contains(t, fields["message"], errString)
	}
}
