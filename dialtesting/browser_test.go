// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	browserevidence "github.com/GuanceCloud/cliutils/dialtesting/browserdial/evidence"
	browserrunner "github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBrowserTaskChild(t *testing.T) {
	ct, err := CreateTaskChild(ClassHeadless)
	require.NoError(t, err)
	require.IsType(t, &BrowserTask{}, ct)
}

func TestBrowserTaskRunEmbeddedByDefault(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	stubBrowserEngine(t, nil, nil)

	err = task.Run()
	require.NoError(t, err)

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, ClassHeadless, task.Class())
	assert.Equal(t, "browser_dial_testing", task.MetricName())
	assert.Equal(t, int64(1), fields["success"])
	assert.Greater(t, fields["response_time"], int64(0))
	assert.Equal(t, int64(2), fields["last_step"])
	assert.Contains(t, fields["steps"], "goto")
	assert.Contains(t, fields["steps"], "assert_title")
	assert.Contains(t, fields["steps"], "Example Domain")
	assert.Equal(t, "https://example.com", tags["url"])
	assert.Equal(t, "platform", tags["owner"])
	assert.Equal(t, "1920x1080", tags["viewport"])
	assert.Equal(t, int64(1920), fields["viewport_width"])
	assert.Equal(t, int64(1080), fields["viewport_height"])
	assert.Equal(t, int64(0), fields["retry_count"])
}

func TestBrowserTaskRunSetsLightpandaPath(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{
		optionLightpandaPath: "/opt/datakit/lightpanda",
	})

	var gotOptions browserrunner.EngineOptions
	browserTask.AdvanceOptions = &BrowserAdvanceOption{Engine: "lightpanda"}
	stubBrowserEngine(t, &gotOptions, nil)
	require.NoError(t, task.Run())

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["success"])
	assert.Equal(t, "/opt/datakit/lightpanda", gotOptions.LightpandaPath)
}

func TestBrowserTaskRunSetsChromePath(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{
		optionChromePath: "/opt/datakit-browser/chrome/chrome",
	})

	var gotOptions browserrunner.EngineOptions
	stubBrowserEngine(t, &gotOptions, nil)
	require.NoError(t, task.Run())

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["success"])
	assert.Equal(t, "/opt/datakit-browser/chrome/chrome", gotOptions.ChromePath)
}

func TestBrowserTaskRunEmbeddedFailure(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	stubBrowserEngine(t, nil, nil, "Wrong Title")
	err = task.Run()
	require.NoError(t, err)

	tags, fields := task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Equal(t, int64(-1), fields["success"])
	assert.Contains(t, fields["fail_reason"], "step_error")
	assert.Equal(t, "assertion_failed", fields["failure_type"])
	assert.Contains(t, fields["message"], "title assertion failed")
	assert.Equal(t, int64(2), fields["last_step"])
	assert.Contains(t, fields["steps"], "title")
}

func TestBrowserTaskRunEmbeddedScriptError(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	browserTask.BrowserConfig = "name: browser\nsteps:\n  - action: eval\n    text: window.fail()\n"
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	stubBrowserEngineWithFactory(t, func(context.Context, browserrunner.EngineOptions) (browserrunner.Engine, error) {
		return &fakeBrowserEngine{evalErr: errors.New("eval boom")}, nil
	})

	require.NoError(t, task.Run())
	_, fields := task.GetResults()
	assert.Equal(t, int64(-1), fields["success"])
	assert.Contains(t, fields["fail_reason"], "step_error")
	assert.Equal(t, "script_error", fields["failure_type"])
	assert.Contains(t, fields["message"], "eval boom")
	assert.NotEqual(t, browserSystemErrorMessage, fields["message"])
}

func TestBrowserTaskRunEmbeddedEngineError(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	oldChromeFactory := browserEmbeddedChromeEngineFactory
	browserEmbeddedChromeEngineFactory = func(context.Context, browserrunner.EngineOptions) (browserrunner.Engine, error) {
		return nil, errors.New("start chrome failed")
	}
	t.Cleanup(func() {
		browserEmbeddedChromeEngineFactory = oldChromeFactory
	})

	require.NoError(t, task.Run())
	_, fields := task.GetResults()
	assert.Equal(t, int64(-1), fields["success"])
	assert.Equal(t, browserSystemErrorMessage, fields["message"])
	assert.NotContains(t, fields["message"], "start chrome failed")
}

func TestBrowserTaskRenderTemplate(t *testing.T) {
	task := &BrowserTask{
		Task: &Task{
			Name: "browser",
			ConfigVars: []*ConfigVar{
				{Name: "host", Value: "example.com"},
				{Name: "title", Value: "Example"},
			},
		},
		URL:           "https://{{host}}/display",
		BrowserConfig: "name: browser\ntarget: https://{{host}}\nsteps:\n  - action: goto\n  - action: assert_title\n    contains: {{title}}\n",
	}
	itask, err := NewTask("", task)
	require.NoError(t, err)
	require.NoError(t, itask.RenderTemplateAndInit(nil))

	assert.Equal(t, "https://example.com/display", task.URL)
	assert.Contains(t, task.BrowserConfig, "target: https://example.com")
	assert.Contains(t, task.BrowserConfig, "contains: Example")
}

func TestBrowserTaskURLIsDisplayOnlyResultTag(t *testing.T) {
	task := &BrowserTask{
		Task: &Task{Name: "browser"},
		URL:  "https://display.example.com",
		BrowserConfig: strings.Join([]string{
			"name: browser",
			"target: https://runtime.example.com",
			"steps:",
			"  - action: goto",
			"",
		}, "\n"),
	}
	itask, err := NewTask("", task)
	require.NoError(t, err)

	tags, _ := itask.GetResults()
	assert.Equal(t, "https://display.example.com", tags["url"])
}

func TestBrowserTaskResultIncludesBrowserConfigVars(t *testing.T) {
	task := &BrowserTask{
		Task: &Task{Name: "browser"},
		BrowserConfig: `name: browser
config_vars:
  - name: username
    value: admin@example.com
  - name: password
    value: secret-value
    secure: true
steps:
  - action: goto
`,
	}
	itask, err := NewTask("", task)
	require.NoError(t, err)

	_, fields := itask.GetResults()
	raw, ok := fields["browser_config_vars"].(string)
	require.True(t, ok)

	var vars []browserConfigResultVar
	require.NoError(t, json.Unmarshal([]byte(raw), &vars))
	require.Len(t, vars, 2)

	assert.Equal(t, "username", vars[0].Name)
	assert.Equal(t, "text", vars[0].Type)
	assert.Equal(t, "admin@example.com", vars[0].Value)
	assert.False(t, vars[0].Secure)

	assert.Equal(t, "password", vars[1].Name)
	assert.Equal(t, "secret", vars[1].Type)
	assert.Empty(t, vars[1].Value)
	assert.True(t, vars[1].Secure)
	assert.NotContains(t, raw, "secret-value")
}

func TestBrowserTaskCheckBrowserConfig(t *testing.T) {
	task := newBrowserTaskForTest()
	require.NoError(t, task.check())

	task.BrowserConfig = ""
	assert.EqualError(t, task.check(), "browser_config should not be empty")

	task.BrowserConfig = "name: homepage\ntarget: https://example.com\n"
	assert.EqualError(t, task.check(), "browser_config steps should not be empty")
}

func TestBrowserTaskCheckBrowserConfigTimeoutLimits(t *testing.T) {
	task := newBrowserTaskForTest()
	task.BrowserConfig = "name: homepage\ntarget: https://example.com\ntimeout_ms: 300001\nsteps:\n  - action: goto\n"
	assert.EqualError(t, task.check(), "browser_config timeout_ms should not exceed 300000")

	task.BrowserConfig = "name: homepage\ntarget: https://example.com\nsteps:\n  - action: goto\n    timeout_ms: 60001\n"
	assert.EqualError(t, task.check(), "browser_config steps 1 timeout_ms should not exceed 60000")

	task.BrowserConfig = "name: homepage\ntarget: https://example.com\nauth:\n  mode: form\n  steps:\n    - action: goto\n      timeout_ms: 60001\nsteps:\n  - action: goto\n"
	assert.EqualError(t, task.check(), "browser_config auth.steps 1 timeout_ms should not exceed 60000")
}

func TestBrowserTaskNormalizeBrowserConfigTimeouts(t *testing.T) {
	config := `name: homepage
target: https://example.com
auth:
  mode: form
  steps:
    - action: goto
steps:
  - action: goto
  - action: assert_title
    timeout_ms: 1000
`
	normalized, err := normalizeBrowserConfigTimeouts(config)
	require.NoError(t, err)
	assert.Contains(t, normalized, "timeout_ms: 300000")
	assert.Contains(t, normalized, "timeout_ms: 60000")
	assert.Contains(t, normalized, "timeout_ms: 1000")

	task := &BrowserTask{BrowserConfig: config}
	path, err := task.writeScriptFile()
	require.NoError(t, err)
	defer os.Remove(path) //nolint:errcheck
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "timeout_ms: 300000")
	assert.Contains(t, string(data), "timeout_ms: 60000")
}

func TestBrowserTaskNormalizeBrowserConfigTimeoutErrors(t *testing.T) {
	_, err := normalizeBrowserConfigTimeouts("name: [")
	assert.Error(t, err)

	normalized, err := normalizeBrowserConfigTimeouts("- item")
	require.NoError(t, err)
	assert.Equal(t, "- item", normalized)

	_, err = normalizeBrowserConfigTimeouts("name: homepage\ntimeout_ms: 300001\nsteps:\n  - action: goto\n")
	assert.EqualError(t, err, "browser_config timeout_ms should not exceed 300000")

	_, err = normalizeBrowserConfigTimeouts("name: homepage\nsteps:\n  - action: goto\n    timeout_ms: 60001\n")
	assert.EqualError(t, err, "browser_config steps 1 timeout_ms should not exceed 60000")
}

func TestBrowserTaskParseNewFields(t *testing.T) {
	taskJSON := `{
		"external_id": "bd-homepage",
		"name": "homepage",
		"frequency": "1m",
		"browser_config": "name: homepage\ntarget: https://example.com\nsteps:\n  - action: goto\n",
		"browser_window": {"viewports": [{"width": 1920, "height": 1080}]},
		"advance_options": {
			"engine": "chrome",
			"screenshot_on_failure": true,
			"headers": {"X-Test": "ok"},
			"cookies": [{"name": "sid", "value": "abc"}],
			"ignore_https_errors": true,
			"proxy_url": "http://127.0.0.1:7897"
		},
		"retry_options": {"enabled": true, "count": 2, "interval_sec": 10}
	}`
	child, err := CreateTaskChild(ClassHeadless)
	require.NoError(t, err)
	task, err := NewTask(taskJSON, child)
	require.NoError(t, err)
	browserTask := task.(*BrowserTask) //nolint:forcetypeassert
	require.NoError(t, task.Check())
	assert.Len(t, browserTask.BrowserWindow.Viewports, 1)
	assert.Equal(t, "chrome", browserTask.AdvanceOptions.Engine)
	assert.True(t, browserTask.AdvanceOptions.ScreenshotOnFailure)
	assert.Equal(t, "ok", browserTask.AdvanceOptions.Headers["X-Test"])
	assert.Equal(t, "sid", browserTask.AdvanceOptions.Cookies[0].Name)
	assert.True(t, browserTask.AdvanceOptions.IgnoreHTTPSErrors)
	assert.Equal(t, "http://127.0.0.1:7897", browserTask.AdvanceOptions.ProxyURL)
	assert.Equal(t, 2, browserTask.RetryOptions.Count)
}

func TestBrowserTaskCheckInvalidEngine(t *testing.T) {
	task := newBrowserTaskForTest()
	task.AdvanceOptions = &BrowserAdvanceOption{Engine: "firefox"}
	assert.EqualError(t, task.check(), "advance_options engine should be chrome or lightpanda")
}

func TestBrowserTaskCheckInvalidViewport(t *testing.T) {
	task := newBrowserTaskForTest()
	task.BrowserWindow = &BrowserWindowOption{Viewports: []BrowserViewport{{Width: 0, Height: 1080}}}
	assert.EqualError(t, task.check(), "browser_window viewport width and height should be greater than 0")
}

func TestBrowserTaskCheckMultipleViewports(t *testing.T) {
	task := newBrowserTaskForTest()
	task.BrowserWindow = &BrowserWindowOption{Viewports: []BrowserViewport{
		{Width: 1920, Height: 1080},
		{Width: 1366, Height: 768},
	}}
	assert.EqualError(t, task.check(), "browser_window.viewports currently supports at most one viewport")
}

func TestBrowserTaskDefaultViewport(t *testing.T) {
	task := newBrowserTaskForTest()
	task.BrowserWindow = nil
	require.NoError(t, task.check())
	require.NotNil(t, task.BrowserWindow)
	require.Len(t, task.BrowserWindow.Viewports, 1)
	assert.Equal(t, BrowserViewport{Width: 1920, Height: 1080}, task.BrowserWindow.Viewports[0])

	task.BrowserWindow = &BrowserWindowOption{}
	require.NoError(t, task.check())
	require.Len(t, task.BrowserWindow.Viewports, 1)
	assert.Equal(t, BrowserViewport{Width: 1920, Height: 1080}, task.BrowserWindow.Viewports[0])
}

func TestBrowserTaskDefaultEngine(t *testing.T) {
	task := newBrowserTaskForTest()
	assert.Equal(t, "chrome", task.effectiveEngine())

	task.AdvanceOptions = &BrowserAdvanceOption{Engine: "lightpanda"}
	assert.Equal(t, "lightpanda", task.effectiveEngine())
}

func TestBrowserTaskCheckInvalidRetry(t *testing.T) {
	task := newBrowserTaskForTest()
	task.RetryOptions = &BrowserRetryOption{Enabled: true, Count: 4, IntervalSec: 10}
	assert.EqualError(t, task.check(), "retry_options count should be between 0 and 3")

	task.RetryOptions = &BrowserRetryOption{Enabled: true, Count: 1, IntervalSec: 4}
	assert.EqualError(t, task.check(), "retry_options interval_sec should be between 5 and 300")
}

func TestBrowserTaskCheckInvalidHeaderAndCookie(t *testing.T) {
	task := newBrowserTaskForTest()
	task.AdvanceOptions = &BrowserAdvanceOption{Headers: map[string]string{"": "value"}}
	assert.EqualError(t, task.check(), "advance_options headers key should not be empty")

	task.AdvanceOptions = &BrowserAdvanceOption{Cookies: []BrowserCookie{{Value: "value"}}}
	assert.EqualError(t, task.check(), "advance_options cookie name should not be empty")
}

func TestBrowserTaskRunSingleViewport(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	browserTask.BrowserWindow = &BrowserWindowOption{Viewports: []BrowserViewport{{Width: 1366, Height: 768}}}
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	var gotOptions browserrunner.EngineOptions
	stubBrowserEngine(t, &gotOptions, nil)
	require.NoError(t, task.Run())

	assert.Equal(t, 1366, gotOptions.ViewportWidth)
	assert.Equal(t, 768, gotOptions.ViewportHeight)

	tags, fields := task.GetResults()
	assert.Equal(t, "1366x768", tags["viewport"])
	assert.Equal(t, int64(1366), fields["viewport_width"])
	assert.Equal(t, int64(768), fields["viewport_height"])
}

func TestBrowserTaskRunRejectsInvalidRetryOptionsAtRuntime(t *testing.T) {
	cases := []struct {
		name    string
		options *BrowserRetryOption
		want    string
	}{
		{
			name:    "count",
			options: &BrowserRetryOption{Enabled: true, Count: 4, IntervalSec: 5},
			want:    "retry_options count should be between 0 and 3",
		},
		{
			name:    "interval",
			options: &BrowserRetryOption{Enabled: true, Count: 1, IntervalSec: 4},
			want:    "retry_options interval_sec should be between 5 and 300",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			browserTask := newBrowserTaskForTest()
			browserTask.RetryOptions = tc.options
			task, err := NewTask("", browserTask)
			require.NoError(t, err)

			var calls int
			stubBrowserEngineWithFactory(t, func(_ context.Context, options browserrunner.EngineOptions) (browserrunner.Engine, error) {
				calls++
				return &fakeBrowserEngine{}, nil
			})
			require.NoError(t, task.Run())

			assert.Equal(t, 0, calls)

			tags, fields := task.GetResults()
			assert.Equal(t, "FAIL", tags["status"])
			assert.Equal(t, tc.want, fields["message"])
			assert.Equal(t, "config_error", fields["failure_type"])
			assert.Equal(t, int64(0), fields["retry_count"])
			assert.Equal(t, "1920x1080", tags["viewport"])
			assert.Equal(t, int64(1920), fields["viewport_width"])
			assert.Equal(t, int64(1080), fields["viewport_height"])
		})
	}
}

func TestBrowserTaskConfigErrorsReportConsistentFields(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{
			name:   "total-timeout",
			config: "name: homepage\ntarget: https://example.com\ntimeout_ms: 300001\nsteps:\n  - action: goto\n",
			want:   "browser_config timeout_ms should not exceed 300000",
		},
		{
			name:   "step-timeout",
			config: "name: homepage\ntarget: https://example.com\nsteps:\n  - action: goto\n    timeout_ms: 60001\n",
			want:   "browser_config steps 1 timeout_ms should not exceed 60000",
		},
		{
			name:   "parse",
			config: "name: [",
			want:   "parse browser_config failed",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			browserTask := newBrowserTaskForTest()
			browserTask.BrowserConfig = tc.config
			task, err := NewTask("", browserTask)
			require.NoError(t, err)

			var calls int
			stubBrowserEngineWithFactory(t, func(_ context.Context, options browserrunner.EngineOptions) (browserrunner.Engine, error) {
				calls++
				return &fakeBrowserEngine{}, nil
			})
			require.NoError(t, task.Run())

			assert.Equal(t, 0, calls)

			tags, fields := task.GetResults()
			assert.Equal(t, "FAIL", tags["status"])
			assert.Contains(t, fields["message"], tc.want)
			assert.Equal(t, "config_error", fields["failure_type"])
			assert.Equal(t, int64(-1), fields["success"])
			assert.Equal(t, int64(0), fields["retry_count"])
			assert.Equal(t, "1920x1080", tags["viewport"])
			assert.Equal(t, int64(1920), fields["viewport_width"])
			assert.Equal(t, int64(1080), fields["viewport_height"])
		})
	}
}

func TestBrowserTaskRunAdvanceOptions(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	browserTask.AdvanceOptions = &BrowserAdvanceOption{
		Engine:              "chrome",
		ScreenshotOnFailure: true,
		Headers:             map[string]string{"X-Test": "ok"},
		Cookies:             []BrowserCookie{{Name: "sid", Value: "abc"}},
		IgnoreHTTPSErrors:   true,
		ProxyURL:            "http://127.0.0.1:7897",
	}
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	var gotOptions browserrunner.EngineOptions
	var gotConfig browserrunner.BrowserConfig
	stubBrowserEngine(t, &gotOptions, &gotConfig)
	require.NoError(t, task.Run())

	assert.Equal(t, "http://127.0.0.1:7897", gotOptions.ProxyURL)
	assert.True(t, gotOptions.IgnoreHTTPSErrors)
	assert.Equal(t, "ok", gotConfig.Headers["X-Test"])
	require.Len(t, gotConfig.Cookies, 1)
	assert.Equal(t, "sid", gotConfig.Cookies[0].Name)
	assert.Equal(t, "abc", gotConfig.Cookies[0].Value)
}

func TestBrowserTaskRetryStopsAfterSuccess(t *testing.T) {
	oldSleep := browserRetrySleep
	browserRetrySleep = func(time.Duration) {}
	defer func() { browserRetrySleep = oldSleep }()

	browserTask := newBrowserTaskForTest()
	browserTask.RetryOptions = &BrowserRetryOption{Enabled: true, Count: 2, IntervalSec: 5}
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	var calls int
	stubBrowserEngineWithFactory(t, func(_ context.Context, options browserrunner.EngineOptions) (browserrunner.Engine, error) {
		calls++
		if calls == 1 {
			return &fakeBrowserEngine{title: "Wrong Title"}, nil
		}
		return &fakeBrowserEngine{}, nil
	})
	require.NoError(t, task.Run())

	tags, fields := task.GetResults()
	assert.Equal(t, "RETRY_OK", tags["status"])
	assert.Equal(t, int64(1), fields["retry_count"])
	assert.Equal(t, int64(1), fields["success"])
	rawRecords, ok := fields["retry_records"].(string)
	require.True(t, ok)
	var records []browserRetryRecord
	require.NoError(t, json.Unmarshal([]byte(rawRecords), &records))
	require.Len(t, records, 2)
	assert.Equal(t, 1, records[0].Attempt)
	assert.Equal(t, "FAIL", records[0].Status)
	assert.False(t, records[0].Success)
	assert.Equal(t, 2, records[0].FailedStep)
	assert.Equal(t, "assertion_failed", records[0].FailureType)
	assert.Equal(t, 2, records[1].Attempt)
	assert.Equal(t, "OK", records[1].Status)
	assert.True(t, records[1].Success)
	assert.Equal(t, 2, calls)
}

func TestBrowserTaskParseFailureDoesNotRetry(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	browserTask.BrowserConfig = "name: ["
	browserTask.RetryOptions = &BrowserRetryOption{Enabled: true, Count: 2, IntervalSec: 5}
	task, err := NewTask("", browserTask)
	require.NoError(t, err)

	var calls int
	stubBrowserEngineWithFactory(t, func(_ context.Context, options browserrunner.EngineOptions) (browserrunner.Engine, error) {
		calls++
		return &fakeBrowserEngine{}, nil
	})
	require.NoError(t, task.Run())

	assert.Equal(t, 0, calls)
	_, fields := task.GetResults()
	assert.Contains(t, fields["message"], "parse browser_config failed")
}

func TestBrowserTaskGetHostNameFromGotoURL(t *testing.T) {
	task := &BrowserTask{
		BrowserConfig: `name: homepage
steps:
  - action: goto
    url: https://example.com
  - action: goto
    url: https://example.com/path
  - action: goto
    url: https://docs.example.com
`,
	}
	hosts, err := task.getHostName()
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com", "docs.example.com"}, hosts)
}

func TestBrowserTaskGetHostNameFromAuthGotoURL(t *testing.T) {
	task := &BrowserTask{
		BrowserConfig: `name: homepage
auth:
  mode: form
  steps:
    - action: goto
      url: https://login.example.com
steps:
  - action: goto
    url: https://app.example.com
`,
	}
	hosts, err := task.getHostName()
	require.NoError(t, err)
	assert.Equal(t, []string{"login.example.com", "app.example.com"}, hosts)
}

func TestBrowserTaskIgnoresOuterSuccessWhen(t *testing.T) {
	taskJSON := `{
		"external_id": "bd-homepage",
		"name": "homepage",
		"frequency": "1m",
		"success_when": [{"response_time": "1ms"}],
		"success_when_logic": "and",
		"browser_config": "name: homepage\ntarget: https://example.com\nsteps:\n  - action: goto\n"
	}`
	child, err := CreateTaskChild(ClassHeadless)
	require.NoError(t, err)
	task, err := NewTask(taskJSON, child)
	require.NoError(t, err)

	stubBrowserEngine(t, nil, nil)
	require.NoError(t, task.Check())
	require.NoError(t, task.Run())
	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["success"])
}

func TestBrowserTaskSanitizeRawTask(t *testing.T) {
	task := &BrowserTask{
		Task: &Task{Name: "homepage", Frequency: "1m"},
		BrowserConfig: `name: homepage
target: https://example.com
config_vars:
  - name: LOGIN_USER
    value: user@example.com
    secure: false
  - name: LOGIN_PASSWORD
    value: secret
    secure: true
steps:
  - action: goto
`,
	}
	raw, err := task.getRawTask(mustJSON(t, task))
	require.NoError(t, err)
	assert.Contains(t, raw, "LOGIN_PASSWORD")
	assert.NotContains(t, raw, "secret")
	assert.Contains(t, raw, "user@example.com")
}

func TestBrowserTaskLightpandaPathOption(t *testing.T) {
	task := &BrowserTask{Task: &Task{}}
	task.SetOption(map[string]string{optionLightpandaPath: "/opt/lightpanda"})
	assert.Equal(t, "/opt/lightpanda", task.lightpandaPath())

	task.SetOption(map[string]string{optionLightpandaPathCamel: "/opt/lightpanda-camel"})
	assert.Equal(t, "/opt/lightpanda-camel", task.lightpandaPath())

	task.SetOption(map[string]string{})
	assert.Empty(t, task.lightpandaPath())
}

func TestBrowserTaskChromePathOption(t *testing.T) {
	task := &BrowserTask{Task: &Task{}}
	task.SetOption(map[string]string{optionChromePath: "/opt/chrome"})
	assert.Equal(t, "/opt/chrome", task.chromePath())

	task.SetOption(map[string]string{optionChromePathCamel: "/opt/chrome-camel"})
	assert.Equal(t, "/opt/chrome-camel", task.chromePath())

	task.SetOption(map[string]string{})
	assert.Empty(t, task.chromePath())
}

func TestBrowserTaskResultFallbacks(t *testing.T) {
	task := newBrowserTaskForTest()
	task.stderr = "stderr failure"
	reasons, ok := task.checkResult()
	assert.False(t, ok)
	assert.Equal(t, []string{"stderr failure"}, reasons)

	task.stderr = ""
	reasons, ok = task.checkResult()
	assert.False(t, ok)
	assert.Equal(t, []string{"browser dial failed"}, reasons)

	task.setReqError("before run failed")
	reasons, ok = task.checkResult()
	assert.False(t, ok)
	assert.Equal(t, []string{"before run failed"}, reasons)
}

func TestBrowserTaskResultUsesLastExecutedStep(t *testing.T) {
	task := newBrowserTaskForTest()
	task.exitCode = 1
	task.result = browserDialRun{
		RunID:       "run-failed",
		Name:        "homepage",
		Target:      "https://example.com",
		Status:      "FAIL",
		Success:     false,
		DurationUS:  60000000,
		FailReason:  "timeout",
		FailureType: "timeout",
		Steps: []browserDialStep{
			{
				Seq:        1,
				Name:       "open",
				Action:     "goto",
				Status:     "OK",
				DurationUS: 1000,
				URL:        "https://example.com",
				Title:      "Example Domain",
				Performance: &browserPerformanceMetrics{
					TTFBMS:        12,
					LoadingTimeMS: 123,
				},
			},
			{
				Seq:        2,
				Name:       "wait button",
				Action:     "wait_for_selector",
				Status:     "FAIL",
				DurationUS: 60000000,
				Title:      "Example Domain",
				Error: &browserDialError{
					Name:    "errorsx.TimeoutError",
					Message: "dial script timed out after 60000ms",
					Stack:   "goroutine 1 [running]",
				},
			},
			{
				Seq:        3,
				Name:       "click button",
				Action:     "click",
				Status:     "SKIP",
				DurationUS: 0,
				SkipReason: "previous_step_failed",
			},
		},
	}

	_, fields := task.getResults()
	assert.Equal(t, int64(2), fields["last_step"])
	assert.NotContains(t, fields, "page_url")
	assert.NotContains(t, fields, "page_title")

	rawSteps, ok := fields["steps"].(string)
	require.True(t, ok)
	assert.Contains(t, rawSteps, "dial script timed out after 60000ms")
	assert.NotContains(t, rawSteps, "goroutine 1")
	assert.NotContains(t, rawSteps, `"stack"`)

	var steps []browserDialStep
	require.NoError(t, json.Unmarshal([]byte(rawSteps), &steps))
	require.Len(t, steps, 3)
	require.NotNil(t, steps[0].Performance)
	assert.Equal(t, int64(12), steps[0].Performance.TTFBMS)
	require.NotNil(t, steps[1].Error)
	assert.Empty(t, steps[1].Error.Stack)
	assert.Equal(t, "SKIP", steps[2].Status)
}

func TestBrowserTaskHostNameErrors(t *testing.T) {
	task := &BrowserTask{BrowserConfig: "name: homepage\nsteps:\n  - action: click\n"}
	_, err := task.getHostName()
	assert.EqualError(t, err, "browser_config target or goto url should not be empty")

	task.BrowserConfig = "name: homepage\ntarget: http://[::1\nsteps:\n  - action: goto\n"
	_, err = task.getHostName()
	assert.Error(t, err)
}

func TestBrowserTaskRetryHelpers(t *testing.T) {
	task := &BrowserTask{RetryOptions: &BrowserRetryOption{Enabled: true, Count: 1, IntervalSec: 1}}
	assert.Equal(t, 5*time.Second, task.retryInterval())

	task.RetryOptions.IntervalSec = 301
	assert.Equal(t, 300*time.Second, task.retryInterval())

	task.RetryOptions.Enabled = false
	assert.Equal(t, time.Duration(0), task.retryInterval())

	assert.Equal(t, 0, clampBrowserRetryCount(-1))
	assert.Equal(t, 3, clampBrowserRetryCount(4))
}

func TestBrowserTaskSanitizeBrowserConfigFallbacks(t *testing.T) {
	assert.Equal(t, "", sanitizeBrowserConfig(""))

	invalid := "name: ["
	assert.Equal(t, invalid, sanitizeBrowserConfig(invalid))

	sanitized := sanitizeBrowserConfig(`name: homepage
target: https://example.com
config_vars:
  - name: LOGIN_PASSWORD
    secure: true
steps:
  - action: goto
`)
	assert.Contains(t, sanitized, "LOGIN_PASSWORD")
	assert.Contains(t, sanitized, "value: \"\"")
}

func TestBrowserTaskSmallHelpers(t *testing.T) {
	task := &BrowserTask{}
	_, err := task.getVariableValue(Variable{})
	assert.EqualError(t, err, "not support")

	task.initTask()
	require.NotNil(t, task.Task)
	assert.Equal(t, []BrowserViewport{{Width: 1920, Height: 1080}}, task.BrowserWindow.Viewports)

	assert.Equal(t, []string{"example.com"}, dedupBrowserHostNames([]string{"example.com", "", "example.com"}))
	assert.Equal(t, []string{}, dedupBrowserHostNames(nil))
	assert.Equal(t, []string{}, dedupBrowserHostNames([]string{"", ""}))
}

func TestBrowserTaskRunEmbedded(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	browserTask.BrowserWindow = &BrowserWindowOption{Viewports: []BrowserViewport{{Width: 1366, Height: 768}}}
	browserTask.AdvanceOptions = &BrowserAdvanceOption{
		Engine:              "chrome",
		ScreenshotOnFailure: true,
		Headers:             map[string]string{"X-Test": "ok"},
		Cookies:             []BrowserCookie{{Name: "sid", Value: "abc"}},
		IgnoreHTTPSErrors:   true,
		ProxyURL:            "http://127.0.0.1:7897",
	}

	var gotOptions browserrunner.EngineOptions
	var gotConfig browserrunner.BrowserConfig
	oldChromeFactory := browserEmbeddedChromeEngineFactory
	browserEmbeddedChromeEngineFactory = func(_ context.Context, options browserrunner.EngineOptions) (browserrunner.Engine, error) {
		gotOptions = options
		return &fakeBrowserEngine{config: &gotConfig}, nil
	}
	t.Cleanup(func() {
		browserEmbeddedChromeEngineFactory = oldChromeFactory
	})

	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{
		optionChromePath: "/opt/datakit-browser/chrome/chrome",
	})
	require.NoError(t, task.Run())

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.NotContains(t, tags, "runner")
	assert.Equal(t, "chrome", tags["browser_engine"])
	assert.Equal(t, "1366x768", tags["viewport"])
	assert.Equal(t, int64(1), fields["success"])
	assert.Equal(t, "success", fields["message"])
	assert.Equal(t, int64(1366), fields["viewport_width"])
	assert.Equal(t, int64(768), fields["viewport_height"])
	assert.Equal(t, "/opt/datakit-browser/chrome/chrome", gotOptions.ChromePath)
	assert.Equal(t, 1366, gotOptions.ViewportWidth)
	assert.Equal(t, 768, gotOptions.ViewportHeight)
	assert.Equal(t, "http://127.0.0.1:7897", gotOptions.ProxyURL)
	assert.True(t, gotOptions.IgnoreHTTPSErrors)
	assert.Equal(t, "ok", gotConfig.Headers["X-Test"])
	require.Len(t, gotConfig.Cookies, 1)
	assert.Equal(t, "sid", gotConfig.Cookies[0].Name)
	assert.Equal(t, "abc", gotConfig.Cookies[0].Value)
}

type fakeBrowserEngine struct {
	config  *browserrunner.BrowserConfig
	title   string
	evalErr error
}

func (e *fakeBrowserEngine) Close(context.Context) error { return nil }

func (e *fakeBrowserEngine) Navigate(context.Context, string) error { return nil }

func (e *fakeBrowserEngine) WaitForSelector(context.Context, string) error { return nil }

func (e *fakeBrowserEngine) Click(context.Context, string) error { return nil }

func (e *fakeBrowserEngine) Fill(context.Context, string, string) error { return nil }

func (e *fakeBrowserEngine) Title(context.Context) (string, error) {
	if e.title != "" {
		return e.title, nil
	}
	return "Example Domain", nil
}

func (e *fakeBrowserEngine) URL(context.Context) (string, error) { return "https://example.com", nil }

func (e *fakeBrowserEngine) Text(context.Context, string) (string, error) {
	return "Example Domain", nil
}

func (e *fakeBrowserEngine) Eval(context.Context, string) (string, error) {
	if e.evalErr != nil {
		return "", e.evalErr
	}
	return "", nil
}

func (e *fakeBrowserEngine) CaptureDOM(context.Context) (browserevidence.DomSnapshot, error) {
	return browserevidence.DomSnapshot{}, nil
}

func (e *fakeBrowserEngine) ConsoleEvents() []browserevidence.ConsoleEvent { return nil }

func (e *fakeBrowserEngine) NetworkEvents() []browserevidence.NetworkEvent { return nil }

func (e *fakeBrowserEngine) ConfigureBrowser(_ context.Context, config browserrunner.BrowserConfig) error {
	if e.config != nil {
		*e.config = config
	}
	return nil
}

func stubBrowserEngine(
	t *testing.T,
	gotOptions *browserrunner.EngineOptions,
	gotConfig *browserrunner.BrowserConfig,
	titles ...string,
) {
	t.Helper()
	var calls int
	stubBrowserEngineWithFactory(t, func(_ context.Context, options browserrunner.EngineOptions) (browserrunner.Engine, error) {
		if gotOptions != nil {
			*gotOptions = options
		}
		title := ""
		if calls < len(titles) {
			title = titles[calls]
		}
		calls++
		return &fakeBrowserEngine{config: gotConfig, title: title}, nil
	})
}

func stubBrowserEngineWithFactory(t *testing.T, factory browserrunner.EngineFactory) {
	t.Helper()
	oldChromeFactory := browserEmbeddedChromeEngineFactory
	oldLightpandaFactory := browserEmbeddedLightpandaEngineFactory
	browserEmbeddedChromeEngineFactory = factory
	browserEmbeddedLightpandaEngineFactory = factory
	t.Cleanup(func() {
		browserEmbeddedChromeEngineFactory = oldChromeFactory
		browserEmbeddedLightpandaEngineFactory = oldLightpandaFactory
	})
}

func newBrowserTaskForTest() *BrowserTask {
	task := &BrowserTask{
		Task: &Task{
			Name:      "homepage",
			Frequency: "1m",
			Tags:      map[string]string{"owner": "platform"},
		},
		BrowserConfig: "name: homepage\ntarget: https://example.com\ntimeout_ms: 1000\ntags:\n  owner: platform\nsteps:\n  - action: goto\n  - action: assert_title\n    contains: Example\n",
	}
	task.initTask()
	return task
}

func mustJSON(t *testing.T, value interface{}) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}
