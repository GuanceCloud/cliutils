// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNetPathExecutor struct {
	config        NetPathProbeConfig
	tags          map[string]string
	fields        map[string]any
	reasons       []string
	success       bool
	started       chan struct{}
	waitForCancel bool
	clearCount    int
}

func (e *testNetPathExecutor) Run(ctx context.Context, config NetPathProbeConfig) error {
	e.config = config
	if e.started != nil {
		close(e.started)
	}
	if e.waitForCancel {
		<-ctx.Done()
		return ctx.Err()
	}
	return nil
}

func (e *testNetPathExecutor) Clear() {
	e.clearCount++
}

func (e *testNetPathExecutor) CheckResult() ([]string, bool) {
	return e.reasons, e.success
}

func (e *testNetPathExecutor) GetResults() (map[string]string, map[string]any) {
	return e.tags, e.fields
}

func (e *testNetPathExecutor) SetError(message string) {
	e.reasons = []string{message}
	e.success = false
}

func TestNetPathTaskFactoryAndExecutor(t *testing.T) {
	child, err := CreateTaskChild("netpath")
	require.NoError(t, err)

	task, err := NewTask(validNetPathTaskJSON(), child)
	require.NoError(t, err)
	require.NoError(t, task.Check())
	assert.Equal(t, ClassNetPath, task.Class())
	assert.Equal(t, netPathMetricName, task.MetricName())

	netPathTask, ok := task.(*NetPathTask)
	require.True(t, ok)
	executor := &testNetPathExecutor{
		tags:    map[string]string{"status": "OK"},
		fields:  map[string]any{"success": int64(1)},
		success: true,
	}
	netPathTask.SetExecutor(executor)

	require.NoError(t, task.Run())
	assert.Equal(t, 1, executor.clearCount)
	assert.Equal(t, "task_1", executor.config.ID)
	assert.Equal(t, "example.com", executor.config.Host)
	assert.Equal(t, uint16(443), executor.config.Port)
	assert.Equal(t, "tcp", executor.config.Protocol)
	assert.Equal(t, 30, executor.config.MaxTTL)
	assert.Equal(t, 10, executor.config.E2EQueries)
	assert.Equal(t, 3, executor.config.TracerouteQueries)

	reasons, success := task.CheckResult()
	assert.True(t, success)
	assert.Empty(t, reasons)

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["success"])
	assert.NotContains(t, fields["task"], "access_key")
	assert.NotContains(t, fields["task"], "post_url")
}

func TestNetPathTaskValidation(t *testing.T) {
	tests := []struct {
		name    string
		replace func(map[string]any)
		want    string
	}{
		{
			name: "unsupported protocol",
			replace: func(task map[string]any) {
				task["protocol"] = "http"
			},
			want: "unsupported protocol",
		},
		{
			name: "tcp requires port",
			replace: func(task map[string]any) {
				delete(task, "port")
			},
			want: "port must be between",
		},
		{
			name: "icmp rejects port",
			replace: func(task map[string]any) {
				task["protocol"] = "icmp"
			},
			want: "port must be omitted",
		},
		{
			name: "IPv6 is unsupported",
			replace: func(task map[string]any) {
				task["host"] = "2001:db8::1"
			},
			want: "IPv6 host is not supported",
		},
		{
			name: "timeout is bounded",
			replace: func(task map[string]any) {
				task["advance_options"].(map[string]any)["timeout"] = "31s"
			},
			want: "timeout must be between",
		},
		{
			name: "assertions are required",
			replace: func(task map[string]any) {
				task["success_when"] = []any{}
			},
			want: "success_when is required",
		},
		{
			name: "numeric assertion rejects null",
			replace: func(task map[string]any) {
				success := task["success_when"].([]any)[0].(map[string]any)
				condition := success["e2e_probe_loss_percent"].([]any)[0].(map[string]any)
				condition["target"] = nil
			},
			want: "target must be numeric",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var raw map[string]any
			require.NoError(t, json.Unmarshal([]byte(validNetPathTaskJSON()), &raw))
			test.replace(raw)
			data, err := json.Marshal(raw)
			require.NoError(t, err)

			child, err := CreateTaskChild(ClassNetPath)
			require.NoError(t, err)
			task, err := NewTask(string(data), child)
			require.NoError(t, err)
			require.ErrorContains(t, task.Check(), test.want)
		})
	}
}

func TestNetPathTaskRendersAndValidatesTemplates(t *testing.T) {
	var raw map[string]any
	require.NoError(t, json.Unmarshal([]byte(validNetPathTaskJSON()), &raw))
	raw["protocol"] = "{{protocol}}"
	raw["host"] = "{{host}}"
	raw["port"] = "{{port}}"
	raw["success_when_logic"] = "{{logic}}"
	advanceOptions := raw["advance_options"].(map[string]any)
	advanceOptions["timeout"] = "{{timeout}}"
	advanceOptions["source_name"] = "{{source}}"
	success := raw["success_when"].([]any)[0].(map[string]any)
	loss := success["e2e_probe_loss_percent"].([]any)[0].(map[string]any)
	loss["target"] = "{{loss}}"
	status := success["e2e_status"].([]any)[0].(map[string]any)
	status["target"] = "{{status}}"
	raw["config_vars"] = []any{
		map[string]any{"name": "protocol", "value": " TCP "},
		map[string]any{"name": "host", "value": "rendered.example.com"},
		map[string]any{"name": "port", "value": "8443"},
		map[string]any{"name": "logic", "value": "or"},
		map[string]any{"name": "timeout", "value": "5s"},
		map[string]any{"name": "source", "value": `source "quoted"`},
		map[string]any{"name": "loss", "value": "2.5"},
		map[string]any{"name": "status", "value": "reached"},
	}
	data, err := json.Marshal(raw)
	require.NoError(t, err)

	child, err := CreateTaskChild(ClassNetPath)
	require.NoError(t, err)
	task, err := NewTask(string(data), child)
	require.NoError(t, err)

	// The debug path validates before rendering. Template placeholders must
	// not be parsed as concrete NetPath values at this stage.
	require.NoError(t, task.CheckTask())
	require.NoError(t, task.RenderTemplateAndInit(nil))

	netPathTask := task.(*NetPathTask)
	assert.Equal(t, " TCP ", netPathTask.Protocol)
	assert.Equal(t, "rendered.example.com", netPathTask.Host)
	assert.Equal(t, "8443", netPathTask.Port)
	assert.Equal(t, "5s", netPathTask.AdvanceOptions.Timeout)
	assert.Equal(t, `source "quoted"`, netPathTask.AdvanceOptions.SourceName)
	assert.Equal(t, "or", netPathTask.SuccessWhenLogic)

	var number float64
	require.NoError(t, json.Unmarshal(
		netPathTask.SuccessWhen[0].E2EProbeLossPercent[0].Target,
		&number,
	))
	assert.Equal(t, 2.5, number)
	assert.JSONEq(t, `"reached"`, string(netPathTask.SuccessWhen[0].E2EStatus[0].Target))
}

func TestNetPathTaskRejectsInvalidRenderedEndpoint(t *testing.T) {
	raw := strings.Replace(validNetPathTaskJSON(), `"tcp"`, `"{{protocol}}"`, 1)
	raw = strings.Replace(raw, `"config_vars": [`, `"config_vars": [
			{"name": "protocol", "value": "http"},`, 1)

	child, err := CreateTaskChild(ClassNetPath)
	require.NoError(t, err)
	task, err := NewTask(raw, child)
	require.NoError(t, err)
	require.NoError(t, task.CheckTask())
	require.ErrorContains(t, task.RenderTemplateAndInit(nil), "unsupported protocol")
}

func TestNetPathTaskTemplateAndCancellation(t *testing.T) {
	raw := strings.ReplaceAll(validNetPathTaskJSON(), "example.com", "{{host}}")
	raw = strings.ReplaceAll(raw, `"443"`, `"{{port}}"`)

	child, err := CreateTaskChild(ClassNetPath)
	require.NoError(t, err)
	task, err := NewTask(raw, child)
	require.NoError(t, err)

	netPathTask, ok := task.(*NetPathTask)
	require.True(t, ok)
	executor := &testNetPathExecutor{
		started:       make(chan struct{}),
		waitForCancel: true,
	}
	netPathTask.SetExecutor(executor)

	require.NoError(t, task.RenderTemplateAndInit(map[string]Variable{
		"host-id": {UUID: "host-id", Value: "rendered.example.com"},
		"port-id": {UUID: "port-id", Value: "8443"},
	}))
	assert.Equal(t, "rendered.example.com", netPathTask.Host)
	assert.Equal(t, "8443", netPathTask.Port)

	done := make(chan error, 1)
	go func() {
		done <- task.Run()
	}()
	<-executor.started
	task.Stop()
	require.NoError(t, <-done)

	reasons, success := task.CheckResult()
	assert.False(t, success)
	require.NotEmpty(t, reasons)
	assert.Contains(t, reasons[0], "canceled")
}

func validNetPathTaskJSON() string {
	return `{
		"external_id": "task_1",
		"name": "netpath task",
		"access_key": "ak_test",
		"post_url": "https://example.com/v1/write",
		"status": "OK",
		"frequency": "1m",
		"schedule_type": "frequency",
		"protocol": "tcp",
		"host": "example.com",
		"port": "443",
		"advance_options": {
			"timeout": "3s",
			"source_name": "source-a"
		},
		"config_vars": [
			{"id": "host-id", "type": "global", "name": "host", "value": "example.com", "secure": false},
			{"id": "port-id", "type": "global", "name": "port", "value": "443", "secure": false}
		],
		"success_when": [{
			"e2e_rtt_avg": [{"op": "lt", "target": "2ms"}],
			"e2e_probe_loss_percent": [{"op": "leq", "target": 1}],
			"e2e_status": [{"op": "eq", "target": "OK"}],
			"traceroute_status": [{"op": "eq", "target": "OK"}]
		}],
		"success_when_logic": "and"
	}`
}
