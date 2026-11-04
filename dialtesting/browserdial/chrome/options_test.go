// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/stretchr/testify/require"
)

func TestInvalidNetworkPolicies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "chrome-headless-shell")
	require.NoError(t, os.WriteFile(path, []byte("invalid configuration must fail before browser startup"), 0o755))
	for _, tc := range []struct {
		options runner.EngineOptions
		message string
	}{
		{runner.EngineOptions{PrivateNetworkCIDRs: []string{"invalid"}}, "invalid browser network CIDR"},
		{runner.EngineOptions{ProxyURL: "http://user:secret@proxy.example:99999"}, "invalid Chrome proxy port"},
		{runner.EngineOptions{ProxyURL: "direct://"}, "Chrome proxy_url must be an HTTP or HTTPS proxy URL"},
		{runner.EngineOptions{CACertFile: "/internal-ca.pem"}, "Chrome does not support browser ca_cert_file"},
		{runner.EngineOptions{CACertDir: "/internal-certs"}, "Chrome does not support browser ca_cert_file"},
	} {
		options := tc.options
		options.ChromePath = path
		engine, err := NewEngine(context.Background(), options)
		require.ErrorContains(t, err, tc.message)
		require.Nil(t, engine)
		pool, err := NewPool(context.Background(), 1, options)
		require.Error(t, err)
		require.Nil(t, pool)
	}
}

func TestResolveHeadlessShell(t *testing.T) {
	dir := t.TempDir()
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	shell := filepath.Join(dir, "chrome-headless-shell"+suffix)
	configured := filepath.Join(dir, "configured-shell"+suffix)
	for _, path := range []string{shell, configured, filepath.Join(dir, "google-chrome"+suffix)} {
		require.NoError(t, os.WriteFile(path, []byte("executable lookup fixture"), 0o755))
	}
	t.Setenv("PATH", dir)
	t.Setenv("CHROME_EXECUTABLE_PATH", "")
	path, err := resolveExecutable("")
	require.NoError(t, err)
	require.Equal(t, shell, path)

	t.Setenv("CHROME_EXECUTABLE_PATH", configured)
	path, err = resolveExecutable(shell)
	require.NoError(t, err)
	require.Equal(t, shell, path)
	path, err = resolveExecutable("")
	require.NoError(t, err)
	require.Equal(t, configured, path)

	missing := filepath.Join(dir, "missing-shell")
	_, err = resolveExecutable(missing)
	require.ErrorContains(t, err, "unusable Chrome Headless Shell executable")
	t.Setenv("CHROME_EXECUTABLE_PATH", missing)
	_, err = resolveExecutable("")
	require.ErrorContains(t, err, "unusable Chrome Headless Shell executable")

	t.Setenv("CHROME_EXECUTABLE_PATH", "")
	require.NoError(t, os.Remove(shell))
	_, err = resolveExecutable("")
	require.ErrorContains(t, err, "Chrome Headless Shell not found")
}

func TestPoolRejectsNetworkPolicies(t *testing.T) {
	for _, options := range []runner.EngineOptions{
		{BlockPrivateNetwork: true},
		{PrivateNetworkCIDRs: []string{"10.0.0.0/8"}},
		{ProxyURL: "http://proxy.example:8080"},
	} {
		pool, err := NewPool(context.Background(), 1, options)
		require.ErrorContains(t, err, "dedicated browser per task")
		require.Nil(t, pool)
		engine, err := (&Pool{}).Factory(context.Background(), options)
		require.ErrorContains(t, err, "dedicated browser per task")
		require.Nil(t, engine)
	}
}

func TestPoolRejectsDifferentResourcePolicy(t *testing.T) {
	pool := &Pool{}
	for _, options := range []runner.EngineOptions{{LoadImages: true}, {LoadFonts: true}} {
		engine, err := pool.Factory(context.Background(), options)
		require.ErrorContains(t, err, "must match the browser pool configuration")
		require.Nil(t, engine)
	}
}

func TestPoolRejectsDifferentSandboxPolicy(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		pool := &Pool{allowNoSandbox: allowed}
		engine, err := pool.Factory(context.Background(), runner.EngineOptions{AllowNoSandbox: !allowed})
		require.ErrorContains(t, err, "sandbox setting must match")
		require.Nil(t, engine)
	}
}
