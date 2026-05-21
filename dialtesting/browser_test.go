// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"encoding/json"
	"os"
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
	assert.Equal(t, "https://example.com", tags["url"])
	assert.Equal(t, "platform", tags["owner"])
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

func TestBrowserDialHelperProcess(t *testing.T) {
	mode := os.Getenv("GO_WANT_BROWSER_DIAL_HELPER")
	if mode == "" {
		return
	}
	if len(os.Args) == 0 || !strings.Contains(strings.Join(os.Args, " "), "run") {
		os.Exit(2)
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
				{"seq": 1, "name": "open", "status": "OK", "duration_us": 1000, "url": "https://example.com", "title": "Example"},
			},
			"trace_ids": []string{"trace-1"},
		},
	}
	exitCode := 0
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
