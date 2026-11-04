// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package runner

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/script"
	"github.com/stretchr/testify/require"
)

func TestProxyConfigurationSource(t *testing.T) {
	const node = "http://node-proxy.internal:8080"
	for _, test := range []struct {
		name, task, script, node, want string
		trusted                        bool
	}{
		{name: "direct"},
		{name: "node", node: node, want: node, trusted: true},
		{name: "task overrides script and node", task: "http://task:8080", script: "http://script:8080", node: node, want: "http://task:8080"},
		{name: "script overrides node", script: "http://script:8080", node: node, want: "http://script:8080"},
		{name: "blank overrides", task: " ", script: "\t", node: " " + node + " ", want: node, trusted: true},
		{name: "task cannot inherit node trust", task: node, node: node, want: node},
		{name: "script cannot inherit node trust", script: node, node: node, want: node},
	} {
		t.Run(test.name, func(t *testing.T) {
			called := false
			result := RunScript(context.Background(), script.Script{
				Target: "https://example.com", ProxyURL: test.script,
				Steps: []script.Step{{Action: "goto", URL: "https://example.com"}},
			}, Options{
				ProxyURL: test.task, DefaultProxyURL: test.node, BlockPrivateNetwork: true,
				EngineFactory: func(_ context.Context, options EngineOptions) (Engine, error) {
					called = true
					require.Equal(t, test.want, options.ProxyURL)
					require.Equal(t, test.trusted, options.ProxyFromNode)
					require.True(t, options.BlockPrivateNetwork)
					return nil, errors.New("stop before launching a browser")
				},
			})
			require.True(t, called)
			require.False(t, result.Success)
		})
	}
}

func TestScriptCannotSetProxyTrust(t *testing.T) {
	var loaded script.Script
	require.NoError(t, json.Unmarshal([]byte(`{
		"target":"https://example.com", "proxy_url":"http://private:8080", "proxy_from_node":true,
		"steps":[{"action":"goto","url":"https://example.com"}]
	}`), &loaded))
	called := false
	RunScript(context.Background(), loaded, Options{
		DefaultProxyURL: "http://node-proxy:8080", BlockPrivateNetwork: true,
		EngineFactory: func(_ context.Context, options EngineOptions) (Engine, error) {
			called = true
			require.Equal(t, "http://private:8080", options.ProxyURL)
			require.False(t, options.ProxyFromNode)
			return nil, errors.New("stop before launching a browser")
		},
	})
	require.True(t, called)
}

func TestScriptCannotEnableNoSandbox(t *testing.T) {
	var loaded script.Script
	require.NoError(t, json.Unmarshal([]byte(`{
		"target":"https://example.com", "allow_no_sandbox":true,
		"steps":[{"action":"goto","url":"https://example.com"}]
	}`), &loaded))
	for _, allowed := range []bool{false, true} {
		called := false
		RunScript(context.Background(), loaded, Options{
			AllowNoSandbox: allowed,
			EngineFactory: func(_ context.Context, options EngineOptions) (Engine, error) {
				called = true
				require.Equal(t, allowed, options.AllowNoSandbox)
				return nil, errors.New("stop before launching a browser")
			},
		})
		require.True(t, called)
	}
}
