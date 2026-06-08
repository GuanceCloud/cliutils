// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBrowserTaskDisplayRunnerError(t *testing.T) {
	raw := "lightpanda exited before CDP was ready: exit status 1\n" +
		"/root/.local/bin/lightpanda: /lib64/libm.so.6: version GLIBC_2.27 not found\n" +
		"/root/.local/bin/lightpanda: /lib64/libc.so.6: version GLIBC_2.34 not found"
	task := &BrowserTask{
		result: browserDialRun{
			FailReason:  "runner_error",
			FailureType: "browser_error",
			Error:       &browserDialError{Name: "Error", Message: raw, Stack: "/root/.local/bin/lightpanda"},
			Steps: []browserDialStep{
				{
					Seq:    1,
					Status: "FAIL",
					Error:  &browserDialError{Name: "Error", Message: raw, Stack: "/root/.local/bin/lightpanda"},
				},
			},
			RetryRecords: []browserRetryRecord{
				{Attempt: 1, Status: "FAIL", FailureType: "runner_error", Message: raw},
			},
		},
	}

	display := task.displayRunnerError()
	assert.Equal(t, browserSystemErrorMessage, display)
	assert.Equal(t, []string{display}, task.displayReasons([]string{"runner_error", raw}))

	steps := sanitizeBrowserSteps(task.result.Steps, display)
	assert.Len(t, steps, 1)
	assert.Equal(t, display, steps[0].Error.Message)
	assert.Empty(t, steps[0].Error.Stack)

	records := sanitizeBrowserRetryRecords(task.result.RetryRecords, display)
	assert.Len(t, records, 1)
	assert.Equal(t, display, records[0].Message)

	assertNoLeak(t, steps[0].Error.Message, "/root/.local/bin/lightpanda", "GLIBC_")
	assertNoLeak(t, records[0].Message, "/root/.local/bin/lightpanda", "GLIBC_")
}

func TestBrowserTaskDisplayRunnerErrorUsesUnifiedMessage(t *testing.T) {
	rawMessages := []string{
		"no lightpanda executable found; set LIGHTPANDA_EXECUTABLE_PATH or install lightpanda in PATH",
		"fork/exec /opt/lightpanda: permission denied",
		"/lib64/libc.so.6: version GLIBC_2.34 not found",
		"lightpanda CDP server did not become ready at http://127.0.0.1:9222",
	}

	for _, raw := range rawMessages {
		t.Run(raw, func(t *testing.T) {
			task := &BrowserTask{
				result: browserDialRun{
					FailReason: "runner_error",
					Error:      &browserDialError{Message: raw},
				},
			}

			assert.Equal(t, browserSystemErrorMessage, task.displayRunnerError())
		})
	}
}

func TestBrowserTaskDisplayRunnerErrorKeepsNonRunnerError(t *testing.T) {
	task := &BrowserTask{
		result: browserDialRun{
			FailReason:  "selector_not_found",
			FailureType: "selector_not_found",
			Error:       &browserDialError{Message: "selector not found: #login"},
		},
	}

	assert.Empty(t, task.displayRunnerError())
	assert.Equal(t, []string{"selector not found: #login"}, task.displayReasons([]string{"selector not found: #login"}))
}

func assertNoLeak(t *testing.T, value string, leaks ...string) {
	t.Helper()
	for _, leak := range leaks {
		assert.False(t, strings.Contains(value, leak))
	}
}
