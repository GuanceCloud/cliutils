// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPConnectionEmptyArrayValidation(t *testing.T) {
	for _, tc := range []struct {
		name, rules string
		invalid     bool
	}{
		{"empty", `[{"connection":[]}]`, true},
		{"empty with response time", `[{"connection":[],"response_time":[{"target":"1s"}]}]`, true},
		{"empty after valid connection", `[{"connection":[{"op":"eq","target":"refused"}]},{"connection":[]}]`, true},
		{"legacy omitted", `[{"response_time":[{"target":"1s"}]}]`, false},
		{"optional null", `[{"connection":null,"response_time":[{"target":"1s"}]}]`, false},
		{"valid", `[{"connection":[{"op":"eq","target":"refused"}]}]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := `{"external_id":"tcp-review","frequency":"1m","host":"127.0.0.1","port":"1","success_when":` + tc.rules + `}`
			task, err := NewTask(raw, &TCPTask{})
			require.NoError(t, err)
			for _, err := range []error{task.Check(), task.RenderTemplateAndInit(nil)} {
				if tc.invalid {
					require.ErrorContains(t, err, "TCP connection rules must not be empty")
				} else {
					require.NoError(t, err)
				}
			}
		})
	}
}

func TestTCPConnectionUnavailableResponse(t *testing.T) {
	for _, state := range []string{"refused", "timeout"} {
		for _, logic := range []string{"and", "or"} {
			for _, option := range []*SuccessOption{
				{IsNot: "ERROR"}, {NotContains: "ERROR"}, {NotMatchRegex: "ERROR"}, {MatchRegex: "^$"},
			} {
				t.Run(state+"/"+logic+"/"+option.IsNot+option.NotContains+option.NotMatchRegex+option.MatchRegex, func(t *testing.T) {
					task := connectionTask(t, "eq", state)
					task.Message, task.SuccessWhenLogic = "ping", logic
					task.SuccessWhen[0].ResponseMessage = []*SuccessOption{option}
					// AND must reject a missing message; OR must not pass solely on it.
					if logic == "or" {
						task.SuccessWhen[0].Connection[0].Target = "established"
					}
					require.NoError(t, task.CheckTask())
					require.NoError(t, task.runWithNetwork(context.Background(), nil,
						func(context.Context, string, string) (net.Conn, error) { return nil, connectionError(state) }))
					tags, fields := task.GetResults()
					assert.Equal(t, "FAIL", tags["status"])
					assert.Contains(t, fields["fail_reason"], "response message unavailable")
					assert.NotContains(t, fields, "message_error")
					if logic == "or" {
						task.SuccessWhen[0].Connection[0].Target = state
						tags, _ = task.GetResults()
						assert.Equal(t, "OK", tags["status"], "a matching connection assertion can still satisfy OR")
					}
				})
			}
		}
	}
}

func TestTCPConnectionResponseAvailabilityResets(t *testing.T) {
	task := connectionTask(t, "eq", "established")
	task.Message = "ping"
	task.SuccessWhen[0].ResponseMessage = []*SuccessOption{{NotContains: "ERROR"}}
	require.NoError(t, task.CheckTask())
	client, server := net.Pipe()
	done := make(chan error, 1)
	go func() {
		defer server.Close() //nolint:errcheck
		if _, err := io.ReadFull(server, make([]byte, len(task.Message))); err != nil {
			done <- err
			return
		}
		_, err := server.Write([]byte("pong"))
		done <- err
	}()
	require.NoError(t, task.runWithNetwork(context.Background(), nil,
		func(context.Context, string, string) (net.Conn, error) { return client, nil }))
	require.NoError(t, <-done)
	tags, _ := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	task.clear()
	task.SuccessWhen[0].Connection[0].Target = "refused"
	require.NoError(t, task.runWithNetwork(context.Background(), nil,
		func(context.Context, string, string) (net.Conn, error) { return nil, connectionError("refused") }))
	tags, fields := task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Contains(t, fields["fail_reason"], "response message unavailable")

	// A task without connection assertions retains its existing message semantics.
	task.SuccessWhen = []*TCPSuccess{{ResponseMessage: []*SuccessOption{{NotContains: "ERROR"}}}}
	require.NoError(t, task.CheckTask())
	task.clear()
	tags, _ = task.GetResults()
	assert.Equal(t, "OK", tags["status"])
}
