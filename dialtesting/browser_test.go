// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateBrowserTaskChild(t *testing.T) {
	ct, err := CreateTaskChild(ClassHeadless)
	require.NoError(t, err)
	require.IsType(t, &BrowserTask{}, ct)
}

func TestBrowserTaskRunExternalProcess(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "success")
	err = task.Run()
	require.NoError(t, err)

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, ClassHeadless, task.Class())
	assert.Equal(t, "browser_dial_testing", task.MetricName())
	assert.Equal(t, int64(1), fields["success"])
	assert.Equal(t, int64(12345), fields["response_time"])
	assert.Equal(t, int64(1), fields["last_step"])
	assert.Equal(t, "trace-1", fields["trace_id"])
	assert.Contains(t, fields["steps"], "open")
	assert.Contains(t, fields["steps"], "/tmp/browser-dial/run-1-step-1.png")
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
		optionBrowserDialPath: os.Args[0],
		optionLightpandaPath:  "/opt/datakit/lightpanda",
	})

	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "check-lightpanda")
	err = task.Run()
	require.NoError(t, err)

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["success"])
}

func TestBrowserTaskRunExternalProcessFailure(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "failure")
	err = task.Run()
	require.NoError(t, err)

	tags, fields := task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Equal(t, int64(-1), fields["success"])
	assert.Contains(t, fields["fail_reason"], "step_error")
	assert.Contains(t, fields["message"], "title mismatch")
	assert.Equal(t, int64(2), fields["last_step"])
	assert.Contains(t, fields["steps"], "title")
}

func TestBrowserTaskStopCancelsExternalProcess(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	startedPath := t.TempDir() + "/started"
	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "sleep")
	t.Setenv("BROWSER_DIAL_HELPER_STARTED", startedPath)

	done := make(chan error, 1)
	go func() {
		done <- task.Run()
	}()

	require.Eventually(t, func() bool {
		_, err := os.Stat(startedPath)
		return err == nil
	}, 2*time.Second, 10*time.Millisecond)

	task.Stop()
	task.Stop()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("browser task did not stop external process")
	}

	_, fields := task.GetResults()
	assert.Equal(t, int64(-1), fields["success"])
	assert.NotEmpty(t, fields["message"])
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
		BrowserConfig: "name: browser\ntarget: https://{{host}}\nsteps:\n  - action: goto\n  - action: assert_title\n    contains: {{title}}\n",
	}
	itask, err := NewTask("", task)
	require.NoError(t, err)
	require.NoError(t, itask.RenderTemplateAndInit(nil))

	assert.Contains(t, task.BrowserConfig, "target: https://example.com")
	assert.Contains(t, task.BrowserConfig, "contains: Example")
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
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	argsPath := t.TempDir() + "/args.log"
	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "success")
	t.Setenv("BROWSER_DIAL_HELPER_ARGS", argsPath)
	require.NoError(t, task.Run())

	lines := readHelperArgs(t, argsPath)
	require.Len(t, lines, 1)
	assert.Contains(t, lines[0], "--viewport-width 1366 --viewport-height 768")

	tags, fields := task.GetResults()
	assert.Equal(t, "1366x768", tags["viewport"])
	assert.Equal(t, int64(1366), fields["viewport_width"])
	assert.Equal(t, int64(768), fields["viewport_height"])
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
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	argsPath := t.TempDir() + "/args.log"
	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "success")
	t.Setenv("BROWSER_DIAL_HELPER_ARGS", argsPath)
	require.NoError(t, task.Run())

	line := readHelperArgs(t, argsPath)[0]
	assert.Contains(t, line, "--engine chrome")
	assert.Contains(t, line, "--screenshot-on-failure")
	assert.Contains(t, line, "--header X-Test=ok")
	assert.Contains(t, line, "--cookie sid=abc")
	assert.Contains(t, line, "--ignore-https-errors")
	assert.Contains(t, line, "--proxy-url http://127.0.0.1:7897")
}

func TestBrowserTaskRetryStopsAfterSuccess(t *testing.T) {
	oldSleep := browserRetrySleep
	browserRetrySleep = func(time.Duration) {}
	defer func() { browserRetrySleep = oldSleep }()

	browserTask := newBrowserTaskForTest()
	browserTask.RetryOptions = &BrowserRetryOption{Enabled: true, Count: 2, IntervalSec: 5}
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	dir := t.TempDir()
	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "fail-once")
	t.Setenv("BROWSER_DIAL_HELPER_COUNT", dir+"/count")
	t.Setenv("BROWSER_DIAL_HELPER_ARGS", dir+"/args.log")
	require.NoError(t, task.Run())

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["retry_count"])
	assert.Len(t, readHelperArgs(t, dir+"/args.log"), 2)
}

func TestBrowserTaskParseFailureDoesNotRetry(t *testing.T) {
	browserTask := newBrowserTaskForTest()
	browserTask.BrowserConfig = "name: ["
	browserTask.RetryOptions = &BrowserRetryOption{Enabled: true, Count: 2, IntervalSec: 5}
	task, err := NewTask("", browserTask)
	require.NoError(t, err)
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	argsPath := t.TempDir() + "/args.log"
	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "success")
	t.Setenv("BROWSER_DIAL_HELPER_ARGS", argsPath)
	require.NoError(t, task.Run())

	_, err = os.Stat(argsPath)
	assert.True(t, os.IsNotExist(err))
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
	task.SetOption(map[string]string{optionBrowserDialPath: os.Args[0]})

	t.Setenv("GO_WANT_BROWSER_DIAL_HELPER", "success")
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

func TestBrowserTaskExecutablePathOption(t *testing.T) {
	task := &BrowserTask{Task: &Task{}}
	task.SetOption(map[string]string{optionBrowserDialPath: "/opt/browser-dial"})
	assert.Equal(t, "/opt/browser-dial", task.executablePath())

	task.SetOption(map[string]string{optionBrowserDialPathCamel: "/opt/browser-dial-camel"})
	assert.Equal(t, "/opt/browser-dial-camel", task.executablePath())

	task.SetOption(map[string]string{})
	t.Setenv("BROWSER_DIAL_PATH", "/env/browser-dial")
	assert.Equal(t, "/env/browser-dial", task.executablePath())

	t.Setenv("BROWSER_DIAL_PATH", "")
	assert.Equal(t, defaultBrowserDialPath, task.executablePath())
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

func TestBrowserDialHelperProcess(t *testing.T) {
	mode := os.Getenv("GO_WANT_BROWSER_DIAL_HELPER")
	if mode == "" {
		return
	}
	if len(os.Args) == 0 || !strings.Contains(strings.Join(os.Args, " "), "run") {
		os.Exit(2)
	}
	if argsPath := os.Getenv("BROWSER_DIAL_HELPER_ARGS"); argsPath != "" {
		file, err := os.OpenFile(argsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			os.Exit(4)
		}
		_, _ = file.WriteString(strings.Join(os.Args, " ") + "\n")
		_ = file.Close()
	}
	if mode == "check-lightpanda" && os.Getenv("LIGHTPANDA_EXECUTABLE_PATH") != "/opt/datakit/lightpanda" {
		os.Exit(3)
	}
	if mode == "sleep" {
		if startedPath := os.Getenv("BROWSER_DIAL_HELPER_STARTED"); startedPath != "" {
			_ = os.WriteFile(startedPath, []byte("started"), 0o600)
		}
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}

	output := map[string]interface{}{
		"exit_code": 0,
		"run": map[string]interface{}{
			"run_id":      "run-1",
			"name":        "homepage",
			"target":      "https://example.com",
			"status":      "OK",
			"success":     true,
			"duration_us": 12345,
			"steps": []map[string]interface{}{
				{
					"seq": 1, "name": "open", "status": "OK", "duration_us": 1000,
					"url": "https://example.com", "title": "Example", "screenshot": "/tmp/browser-dial/run-1-step-1.png",
				},
			},
			"trace_ids": []string{"trace-1"},
		},
	}
	exitCode := 0
	if mode == "fail-once" && incrementHelperCount(os.Getenv("BROWSER_DIAL_HELPER_COUNT")) == 1 {
		mode = "failure"
	}
	if mode == "failure" {
		output["exit_code"] = 1
		output["run"] = map[string]interface{}{
			"run_id":      "run-2",
			"name":        "homepage",
			"target":      "https://example.com",
			"status":      "FAIL",
			"success":     false,
			"duration_us": 54321,
			"fail_reason": "step_error",
			"error": map[string]interface{}{
				"name":    "assertion",
				"message": "title mismatch",
			},
			"steps": []map[string]interface{}{
				{"seq": 1, "name": "open", "status": "OK", "duration_us": 1000},
				{"seq": 2, "name": "title", "status": "FAIL", "duration_us": 1000},
			},
		}
		exitCode = 1
	}

	_ = json.NewEncoder(os.Stdout).Encode(output)
	os.Exit(exitCode)
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

func readHelperArgs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func incrementHelperCount(path string) int {
	if path == "" {
		return 1
	}
	count := 0
	if data, err := os.ReadFile(path); err == nil {
		count, _ = strconv.Atoi(strings.TrimSpace(string(data)))
	}
	count++
	_ = os.WriteFile(path, []byte(strconv.Itoa(count)), 0o600)
	return count
}
