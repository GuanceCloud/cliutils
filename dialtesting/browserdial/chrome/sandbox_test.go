// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
)

func TestChromeSandboxUnavailable(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "no error"},
		{name: "unavailable", err: errors.New("chrome failed to start:\n[FATAL] No usable sandbox!"), want: true},
		{name: "namespace blocked", err: errors.New("Failed to move to new namespace: errno = Operation not permitted"), want: true},
		{name: "unrelated permission", err: errors.New("open profile: Operation not permitted")},
		{name: "missing binary", err: errors.New("fork/exec chrome: no such file or directory")},
		{name: "canceled", err: fmt.Errorf("No usable sandbox: %w", context.Canceled)},
		{name: "timeout", err: fmt.Errorf("No usable sandbox: %w", context.DeadlineExceeded)},
	} {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.want, chromeSandboxUnavailable(test.err))
		})
	}
}

func TestChromeSandboxPermission(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses a POSIX shell")
	}
	for _, allowed := range []bool{false, true} {
		t.Run(fmt.Sprint(allowed), func(t *testing.T) {
			dir := t.TempDir()
			attempts := filepath.Join(dir, "attempts")
			t.Setenv("CHROME_SANDBOX_TEST_ATTEMPTS", attempts)
			binary := filepath.Join(dir, "chrome-headless-shell")
			fixture := `#!/bin/sh
printf '%s\n' "$*" >> "$CHROME_SANDBOX_TEST_ATTEMPTS"
echo 'No usable sandbox!' >&2
exit 1
`
			require.NoError(t, os.WriteFile(binary, []byte(fixture), 0o755))
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _, err := startChrome(ctx, binary, 800, 600, runner.EngineOptions{AllowNoSandbox: allowed})
			require.Error(t, err)
			if !allowed && os.Getuid() == 0 {
				require.ErrorContains(t, err, "browser.allow_no_sandbox is false")
				_, err = os.Stat(attempts)
				require.True(t, os.IsNotExist(err))
				return
			}
			data, err := os.ReadFile(attempts)
			require.NoError(t, err)
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			want := 1
			if allowed && os.Getuid() != 0 {
				want = 2
			}
			require.Len(t, lines, want)
			require.Equal(t, allowed, slices.Contains(strings.Fields(lines[len(lines)-1]), "--no-sandbox"))
			if len(lines) == 2 {
				require.NotContains(t, strings.Fields(lines[0]), "--no-sandbox")
			}
		})
	}
}

func TestChromeSandboxStartup(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
	expected := os.Getenv("CHROME_TEST_EXPECT_NO_SANDBOX")
	if binary == "" || expected == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH and CHROME_TEST_EXPECT_NO_SANDBOX to check browser startup")
	}
	wantNoSandbox, err := strconv.ParseBool(expected)
	require.NoError(t, err)
	allowNoSandbox := os.Getenv("CHROME_TEST_ALLOW_NO_SANDBOX") == "true"
	for _, pooled := range []bool{false, true} {
		t.Run(fmt.Sprintf("pooled=%t", pooled), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			options := runner.EngineOptions{ChromePath: binary, AllowNoSandbox: allowNoSandbox}
			factory := NewEngine
			if pooled {
				pool, err := NewPool(ctx, 1, options)
				require.NoError(t, err)
				defer pool.Close()
				factory = pool.Factory
			}
			base, err := factory(ctx, options)
			require.NoError(t, err)
			defer func() { require.NoError(t, base.Close(context.Background())) }()
			engine := base.(*Engine)
			var arguments []string
			require.NoError(t, engine.run(engine.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
				var err error
				arguments, err = cdpbrowser.GetBrowserCommandLine().Do(ctx)
				return err
			})))
			require.Equal(t, wantNoSandbox, slices.Contains(arguments, "--no-sandbox"))
			require.Contains(t, arguments, "--site-per-process")
			require.NoError(t, engine.ConfigureBrowser(ctx, runner.BrowserConfig{
				Synthetics: runner.SyntheticsContext{TestID: "sandbox-test", ResultID: "sandbox-result"},
			}))
			require.NoError(t, engine.Navigate(ctx, "about:blank"))
			var correlation []any
			require.NoError(t, engine.run(engine.ctx, chromedp.Evaluate(`[
window._DATAFLUX_SYNTHETICS_BROWSER,
window._GUANCE_SYNTHETICS_PUBLIC_ID,
window._GUANCE_SYNTHETICS_RESULT_ID
]`, &correlation)))
			require.Equal(t, []any{true, "sandbox-test", "sandbox-result"}, correlation)
			var value int
			require.NoError(t, engine.run(engine.ctx, chromedp.Evaluate("1 + 1", &value)))
			require.Equal(t, 2, value)
		})
	}
}
