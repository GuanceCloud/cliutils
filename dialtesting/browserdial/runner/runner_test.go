package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/evidence"
	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/script"
)

func TestRunSuccess(t *testing.T) {
	path := writeScript(t, `
name: homepage
target: https://example.com
post_url: https://openway.example.com?token=tkn_x
tags:
  owner: platform
metadata:
  suite: smoke
steps:
  - name: open
    action: goto
  - name: title
    action: assert_title
    contains: Example
  - name: body
    action: assert_text
    selector: body
    contains: Example Domain
`)
	result := Run(context.Background(), Options{
		ScriptPath: path,
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &performanceEngine{fakeEngine: fakeEngine{}}, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}
	if result.Tags["owner"] != "platform" {
		t.Fatalf("unexpected tags: %#v", result.Tags)
	}
	if result.Metadata["suite"] != "smoke" {
		t.Fatalf("unexpected metadata: %#v", result.Metadata)
	}
	if result.PostURL != "https://openway.example.com?token=tkn_x" {
		t.Fatalf("unexpected post_url: %q", result.PostURL)
	}
	if result.Performance == nil || result.Performance.TTFBMS != 12 || result.Performance.CLS != 0.03 {
		t.Fatalf("unexpected performance metrics: %#v", result.Performance)
	}
	if result.Steps[0].Performance == nil || result.Steps[0].Performance.LoadingTimeMS != 123 {
		t.Fatalf("expected goto step performance metrics: %#v", result.Steps[0].Performance)
	}
	if result.Steps[1].Performance != nil || result.Steps[2].Performance != nil {
		t.Fatalf("non-navigation steps should not include performance metrics: %#v %#v", result.Steps[1].Performance, result.Steps[2].Performance)
	}
}

func TestRunFailureCapturesDOM(t *testing.T) {
	path := writeScript(t, `
name: homepage
target: https://example.com
steps:
  - action: goto
  - name: wrong title
    action: assert_title
    contains: Nope
`)
	result := Run(context.Background(), Options{
		ScriptPath:    path,
		TimeoutMS:     1_000,
		EngineFactory: newFakeEngine,
	})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.FailReason != "step_error" {
		t.Fatalf("FailReason = %s", result.FailReason)
	}
	if result.FailureType != "assertion_failed" {
		t.Fatalf("FailureType = %s", result.FailureType)
	}
	if result.DomSnapshot == nil || result.DomSnapshot.Text != "Example Domain" {
		t.Fatalf("expected DOM snapshot, got %#v", result.DomSnapshot)
	}
}

func TestRunStepTimeoutDoesNotMaskAssertionFailure(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "homepage",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "assert_title", Contains: "Nope", TimeoutMS: 1_000},
		},
	}, Options{
		ScriptPath:    "task:assertion-timeout",
		TimeoutMS:     30_000,
		EngineFactory: newFakeEngine,
	})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.FailReason != "step_error" || result.FailureType != "assertion_failed" {
		t.Fatalf("unexpected failure classification: %#v", result)
	}
}

func TestRunStepTimeoutDoesNotMaskConfigVariableFailure(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "missing-var",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "fill", Selector: "input", ValueFrom: "MISSING", TimeoutMS: 1_000},
		},
	}, Options{
		ScriptPath:    "task:missing-var-timeout",
		TimeoutMS:     30_000,
		EngineFactory: newFakeEngine,
	})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.FailReason != "step_error" || result.FailureType != "config_error" {
		t.Fatalf("unexpected failure classification: %#v", result)
	}
}

func TestRunStepTimeoutClassifiesDeadlineExceeded(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "step-timeout",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "wait_for_selector", Selector: "#never", TimeoutMS: 5},
		},
	}, Options{
		ScriptPath: "task:step-timeout",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &fakeEngine{blockWait: true}, nil
		},
	})
	if result.Success {
		t.Fatal("expected timeout failure")
	}
	if result.FailReason != "timeout" || result.FailureType != "timeout" {
		t.Fatalf("unexpected failure classification: %#v", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Message, "5ms") {
		t.Fatalf("expected step timeout in error, got %#v", result.Error)
	}
}

func TestRunTotalTimeoutClassifiesDirectDeadlineExceeded(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "total-timeout-direct",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "goto", TimeoutMS: 60_000},
		},
	}, Options{
		ScriptPath: "task:total-timeout-direct",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &deadlineEngine{}, nil
		},
	})
	if result.Success {
		t.Fatal("expected timeout failure")
	}
	if result.FailReason != "timeout" || result.FailureType != "timeout" {
		t.Fatalf("unexpected failure classification: %#v", result)
	}
	if result.Error == nil || !strings.Contains(result.Error.Message, "1000ms") {
		t.Fatalf("expected total timeout in error, got %#v", result.Error)
	}
}

func TestRunEngineFactoryDeadlineClassifiesTimeout(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "factory-timeout",
		Target: "https://example.com",
		Steps:  []script.Step{{Action: "goto"}},
	}, Options{
		ScriptPath: "task:factory-timeout",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return nil, context.DeadlineExceeded
		},
	})
	if result.Success {
		t.Fatal("expected timeout failure")
	}
	if result.FailReason != "timeout" || result.FailureType != "timeout" {
		t.Fatalf("unexpected failure classification: %#v", result)
	}
}

func TestRunFailureAddsStepFieldsAndSkipsRemainingSteps(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "homepage",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "goto"},
			{Action: "fill", Selector: `input[name="password"]`, ValueFrom: "LOGIN_PASSWORD", Sensitive: true},
			{Action: "assert_title", Contains: "Nope"},
			{Action: "click", Selector: "#next"},
		},
		ConfigVars: []script.ConfigVar{
			{Name: "LOGIN_PASSWORD", Value: "secret", Secure: true},
		},
	}, Options{
		ScriptPath:    "task:fields",
		TimeoutMS:     1_000,
		EngineFactory: newFakeEngine,
	})
	if result.Success {
		t.Fatal("expected failure")
	}
	if len(result.Steps) != 4 {
		t.Fatalf("steps = %d, want 4: %#v", len(result.Steps), result.Steps)
	}
	fill := result.Steps[1]
	if fill.Action != "fill" || fill.Selector == "" || fill.InputDisplay != "***" || fill.ValueFrom != "LOGIN_PASSWORD" {
		t.Fatalf("unexpected fill evidence: %#v", fill)
	}
	failed := result.Steps[2]
	if failed.Status != evidence.StatusFail || failed.Expected != "contains:Nope" {
		t.Fatalf("unexpected failed step: %#v", failed)
	}
	skipped := result.Steps[3]
	if skipped.Status != evidence.StatusSkip || skipped.SkipReason != "previous_step_failed" || skipped.Action != "click" {
		t.Fatalf("unexpected skipped step: %#v", skipped)
	}
}

func TestRunRetriesUntilSuccess(t *testing.T) {
	calls := 0
	result := RunScript(context.Background(), scriptForTest(), Options{
		ScriptPath: "task:retry",
		TimeoutMS:  1_000,
		RetryCount: 1,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			calls++
			if calls == 1 {
				return &fakeEngine{title: "Wrong"}, nil
			}
			return &fakeEngine{}, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected retry success: %#v", result.Error)
	}
	if calls != 2 || result.Attempt != 2 || result.MaxAttempts != 2 {
		t.Fatalf("calls=%d attempt=%d max=%d", calls, result.Attempt, result.MaxAttempts)
	}
	if len(result.RetryRecords) != 2 {
		t.Fatalf("expected 2 retry records, got %#v", result.RetryRecords)
	}
	if result.RetryRecords[0].Attempt != 1 || result.RetryRecords[0].Status != evidence.StatusFail || result.RetryRecords[0].FailedStep != 2 {
		t.Fatalf("unexpected first retry record: %#v", result.RetryRecords[0])
	}
	if result.RetryRecords[1].Attempt != 2 || result.RetryRecords[1].Status != evidence.StatusOK || !result.RetryRecords[1].Success {
		t.Fatalf("unexpected second retry record: %#v", result.RetryRecords[1])
	}
}

func TestRunFailureCapturesScreenshot(t *testing.T) {
	dir := t.TempDir()
	result := RunScript(context.Background(), script.Script{
		Name:   "homepage",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "assert_title", Contains: "Nope"},
		},
	}, Options{
		ScriptPath:          "task:screenshot",
		TimeoutMS:           1_000,
		EngineName:          "fake",
		ScreenshotOnFailure: true,
		ScreenshotDir:       dir,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &screenshotEngine{fakeEngine: fakeEngine{}}, nil
		},
	})
	if result.Success {
		t.Fatal("expected failure")
	}
	if result.Steps[0].Screenshot == "" {
		t.Fatalf("expected screenshot path: %#v", result.Steps[0])
	}
	if _, err := os.Stat(result.Steps[0].Screenshot); err != nil {
		t.Fatalf("expected screenshot file: %v", err)
	}
}

func TestRunCapturesScreenshotPerStep(t *testing.T) {
	dir := t.TempDir()
	result := RunScript(context.Background(), scriptForTest(), Options{
		ScriptPath:        "task:screenshot-per-step",
		TimeoutMS:         1_000,
		EngineName:        "fake",
		ScreenshotPerStep: true,
		ScreenshotDir:     dir,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &screenshotEngine{fakeEngine: fakeEngine{}}, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("steps = %d", len(result.Steps))
	}
	for _, step := range result.Steps {
		if step.Screenshot == "" {
			t.Fatalf("missing screenshot on step %#v", step)
		}
		if _, err := os.Stat(step.Screenshot); err != nil {
			t.Fatalf("expected screenshot file: %v", err)
		}
	}
}

func TestRunConfiguresBrowserHeadersCookiesAndTLS(t *testing.T) {
	engine := &fakeEngine{}
	result := RunScript(context.Background(), script.Script{
		Name:              "configured",
		Target:            "https://example.com",
		Headers:           map[string]string{"X-Env": "prod"},
		IgnoreHTTPSErrors: true,
		ConfigVars: []script.ConfigVar{
			{Name: "SESSION_ID", Value: "session-secret", Secure: true},
		},
		Cookies: []script.Cookie{
			{Name: "sid", ValueFrom: "SESSION_ID", Domain: "example.com", Path: "/", Secure: true, HTTPOnly: true, SameSite: "Lax"},
		},
		Steps: []script.Step{{Action: "goto"}},
	}, Options{
		ScriptPath:     "task:configured",
		TimeoutMS:      1_000,
		ViewportWidth:  1366,
		ViewportHeight: 768,
		Headers:        map[string]string{"X-Node": "node-a"},
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if !engine.config.IgnoreHTTPSErrors || engine.config.Headers["X-Env"] != "prod" || engine.config.Headers["X-Node"] != "node-a" {
		t.Fatalf("unexpected browser config: %#v", engine.config)
	}
	if len(engine.config.Cookies) != 1 || engine.config.Cookies[0].Value != "session-secret" {
		t.Fatalf("unexpected cookies: %#v", engine.config.Cookies)
	}
	if result.ViewportWidth != 1366 || result.ViewportHeight != 768 {
		t.Fatalf("unexpected viewport on result: %#v", result)
	}
}

func TestRunInfersCookieTargetFromFirstGotoURL(t *testing.T) {
	engine := &fakeEngine{}
	result := RunScript(context.Background(), script.Script{
		Name: "step-url-cookie",
		Cookies: []script.Cookie{
			{Name: "sid", Value: "session"},
		},
		Auth: script.Auth{
			Mode: "form",
			Steps: []script.Step{
				{Action: "goto", URL: "https://login.example.com"},
			},
		},
		Steps: []script.Step{
			{Action: "goto", URL: "https://app.example.com"},
		},
	}, Options{
		ScriptPath: "task:step-url-cookie",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if engine.config.Target != "https://login.example.com" {
		t.Fatalf("cookie target = %q", engine.config.Target)
	}

	engine = &fakeEngine{}
	result = RunScript(context.Background(), script.Script{
		Name: "normal-step-url-cookie",
		Cookies: []script.Cookie{
			{Name: "sid", Value: "session"},
		},
		Steps: []script.Step{
			{Action: "goto", URL: "https://app.example.com"},
		},
	}, Options{
		ScriptPath: "task:normal-step-url-cookie",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if engine.config.Target != "https://app.example.com" {
		t.Fatalf("cookie target = %q", engine.config.Target)
	}
}

func TestRunMissingCookieVariableFailsBeforeExecution(t *testing.T) {
	called := false
	result := RunScript(context.Background(), script.Script{
		Name:    "configured",
		Target:  "https://example.com",
		Cookies: []script.Cookie{{Name: "sid", ValueFrom: "SESSION_ID"}},
		Steps:   []script.Step{{Action: "goto"}},
	}, Options{
		ScriptPath: "task:configured",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			called = true
			return &fakeEngine{}, nil
		},
	})
	if result.Success || result.FailReason != "script_load_error" {
		t.Fatalf("expected script load failure: %#v", result)
	}
	if called {
		t.Fatal("engine factory should not be called")
	}
}

func TestRunTimeout(t *testing.T) {
	path := writeScript(t, `
name: homepage
steps:
  - action: wait_for_selector
    selector: "#never"
`)
	result := Run(context.Background(), Options{
		ScriptPath: path,
		TimeoutMS:  5,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &fakeEngine{blockWait: true}, nil
		},
	})
	if result.Success {
		t.Fatal("expected timeout failure")
	}
	if result.FailReason != "timeout" {
		t.Fatalf("FailReason = %s", result.FailReason)
	}
	if result.FailureType != "timeout" {
		t.Fatalf("FailureType = %s", result.FailureType)
	}
}

func TestRunAuthUsesConfigVariables(t *testing.T) {
	engine := &fakeEngine{}
	result := RunScript(context.Background(), script.Script{
		Name:   "dashboard",
		Target: "https://example.com/dashboard",
		Auth: script.Auth{
			Mode: "form",
			Steps: []script.Step{
				{Action: "goto", URL: "https://example.com/login"},
				{Action: "fill", Selector: `input[name="email"]`, ValueFrom: "LOGIN_USER"},
				{Action: "fill", Selector: `input[name="password"]`, ValueFrom: "LOGIN_PASSWORD", Sensitive: true},
				{Action: "assert_url", Contains: "/login"},
			},
		},
		ConfigVars: []script.ConfigVar{
			{Name: "LOGIN_USER", Value: "monitor@example.com"},
			{Name: "LOGIN_PASSWORD", Value: "secret", Secure: true},
		},
		Steps: []script.Step{
			{Action: "goto"},
			{Action: "assert_title", Contains: "Example"},
		},
	}, Options{
		ScriptPath: "task:auth",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if got := engine.fills[`input[name="password"]`]; got != "secret" {
		t.Fatalf("password fill = %q", got)
	}
	if len(result.Steps) != 6 || result.Steps[0].Seq != 1 || result.Steps[5].Seq != 6 {
		t.Fatalf("unexpected steps: %#v", result.Steps)
	}
}

func TestRunAuthMissingVariableIsAuthError(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:   "dashboard",
		Target: "https://example.com/dashboard",
		Auth: script.Auth{
			Mode: "form",
			Steps: []script.Step{
				{Action: "fill", Selector: `input[name="password"]`, ValueFrom: "LOGIN_PASSWORD", Sensitive: true},
			},
		},
		Steps: []script.Step{
			{Action: "goto"},
		},
	}, Options{
		ScriptPath:    "task:auth",
		TimeoutMS:     1_000,
		EngineFactory: newFakeEngine,
	})
	if result.Success {
		t.Fatal("expected auth failure")
	}
	if result.FailReason != "auth_error" {
		t.Fatalf("FailReason = %s", result.FailReason)
	}
}

func TestRunScriptUsesInMemoryScript(t *testing.T) {
	result := RunScript(context.Background(), scriptForTest(), Options{
		ScriptPath:    "task:bd-homepage",
		TimeoutMS:     1_000,
		EngineFactory: newFakeEngine,
	})
	if !result.Success {
		t.Fatalf("expected success: %#v", result.Error)
	}
	if result.ScriptPath != "task:bd-homepage" {
		t.Fatalf("ScriptPath = %q", result.ScriptPath)
	}
	if result.Name != "homepage" {
		t.Fatalf("Name = %q", result.Name)
	}
}

func TestCollectTraceIDs(t *testing.T) {
	got := collectTraceIDs([]evidence.NetworkEvent{
		{Seq: 1, Event: "request", TraceID: "trace-a"},
		{Seq: 2, Event: "response", TraceID: "trace-a"},
		{Seq: 3, Event: "request", TraceID: "trace-b"},
		{Seq: 4, Event: "request"},
	})
	want := []string{"trace-a", "trace-b"}
	if len(got) != len(want) {
		t.Fatalf("trace IDs = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trace IDs = %#v, want %#v", got, want)
		}
	}
}

func TestRunAdditionalFailureAndStepBranches(t *testing.T) {
	if result := RunScript(context.Background(), scriptForTest(), Options{ScriptPath: "task:no-factory"}); result.Success || result.FailReason != "runner_error" {
		t.Fatalf("expected missing factory runner error: %#v", result)
	}
	if result := RunScript(context.Background(), scriptForTest(), Options{
		ScriptPath: "task:factory-error",
		TimeoutMS:  1_000,
		EngineName: "chrome",
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return nil, errors.New("factory boom")
		},
	}); result.Success || result.Engine != "chrome" || result.FailReason != "runner_error" {
		t.Fatalf("expected factory runner error: %#v", result)
	}
	if result := RunScript(context.Background(), scriptForTest(), Options{
		ScriptPath:          "task:lightpanda-shot",
		TimeoutMS:           1_000,
		EngineName:          "lightpanda",
		ScreenshotPerStep:   true,
		ScreenshotOnFailure: true,
		EngineFactory:       newFakeEngine,
	}); !result.Success || result.Steps[0].Screenshot != "" {
		t.Fatalf("expected lightpanda screenshots to be ignored: %#v", result)
	}
	var gotOptions EngineOptions
	if result := RunScript(context.Background(), scriptForTest(), Options{
		ScriptPath: "task:lightpanda-proxy",
		TimeoutMS:  1_000,
		EngineName: "lightpanda",
		ProxyURL:   "http://127.0.0.1:7897",
		EngineFactory: func(_ context.Context, options EngineOptions) (Engine, error) {
			gotOptions = options
			return &fakeEngine{}, nil
		},
	}); !result.Success || gotOptions.ProxyURL != "" {
		t.Fatalf("expected lightpanda proxy to be ignored, result=%#v options=%#v", result, gotOptions)
	}
	scriptWithProxy := scriptForTest()
	scriptWithProxy.ProxyURL = "http://127.0.0.1:7897"
	var ignored []string
	if result := RunScript(context.Background(), scriptWithProxy, Options{
		ScriptPath: "task:lightpanda-script-proxy",
		TimeoutMS:  1_000,
		EngineName: "lightpanda",
		EngineFactory: func(_ context.Context, options EngineOptions) (Engine, error) {
			gotOptions = options
			return &fakeEngine{}, nil
		},
		IgnoredOptionLogger: func(option string, reason string) {
			ignored = append(ignored, option+" "+reason)
		},
	}); !result.Success || gotOptions.ProxyURL != "" || len(ignored) != 1 || !strings.Contains(ignored[0], "proxy_url") {
		t.Fatalf("expected script proxy ignore warning, result=%#v options=%#v ignored=%#v", result, gotOptions, ignored)
	}

	result := RunScript(context.Background(), script.Script{
		Name:   "branches",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "goto"},
			{Action: "wait_for_selector", Selector: "body"},
			{Action: "click", Selector: "a"},
			{Action: "fill", Selector: "input", Value: "value"},
			{Action: "assert_title", Equals: "Example Domain"},
			{Action: "assert_url", Contains: "example.com"},
			{Action: "assert_text", Selector: "body", Equals: "Example Domain"},
			{Action: "eval", Text: "true"},
		},
	}, Options{
		ScriptPath:    "task:branches",
		TimeoutMS:     1_000,
		EngineFactory: newFakeEngine,
	})
	if !result.Success {
		t.Fatalf("expected branch run success: %#v", result.Error)
	}

	result = RunScript(context.Background(), script.Script{
		Name:   "missing-var",
		Target: "https://example.com",
		Steps:  []script.Step{{Action: "fill", Selector: "input", ValueFrom: "MISSING"}},
	}, Options{ScriptPath: "task:missing-var", TimeoutMS: 1_000, EngineFactory: newFakeEngine})
	if result.Success || result.FailReason != "step_error" {
		t.Fatalf("expected missing variable step error: %#v", result)
	}
}

func TestRunConfigureAndScreenshotUnavailableBranches(t *testing.T) {
	result := RunScript(context.Background(), script.Script{
		Name:    "configure-fails",
		Target:  "https://example.com",
		Headers: map[string]string{"X-Test": "1"},
		Steps:   []script.Step{{Action: "goto"}},
	}, Options{
		ScriptPath: "task:configure-fails",
		TimeoutMS:  1_000,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return &configureFailEngine{fakeEngine: fakeEngine{}}, nil
		},
	})
	if result.Success || result.FailReason != "runner_error" {
		t.Fatalf("expected configure failure: %#v", result)
	}

	result = RunScript(context.Background(), script.Script{
		Name:   "no-shot",
		Target: "https://example.com",
		Steps:  []script.Step{{Action: "assert_title", Contains: "Nope"}},
	}, Options{
		ScriptPath:          "task:no-shot",
		TimeoutMS:           1_000,
		EngineName:          "fake",
		ScreenshotOnFailure: true,
		EngineFactory:       newFakeEngine,
	})
	if result.Success || !strings.Contains(result.Steps[0].Error.Message, "screenshot capture unavailable") {
		t.Fatalf("expected screenshot unavailable note: %#v", result.Steps)
	}
}

func writeScript(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "script.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newFakeEngine(context.Context, EngineOptions) (Engine, error) {
	return &fakeEngine{}, nil
}

func scriptForTest() script.Script {
	return script.Script{
		Name:   "homepage",
		Target: "https://example.com",
		Steps: []script.Step{
			{Action: "goto"},
			{Action: "assert_title", Contains: "Example"},
		},
	}
}

type fakeEngine struct {
	url       string
	title     string
	blockWait bool
	fills     map[string]string
	config    BrowserConfig
}

func (f *fakeEngine) Close(context.Context) error {
	return nil
}

func (f *fakeEngine) ConfigureBrowser(_ context.Context, config BrowserConfig) error {
	f.config = config
	return nil
}

func (f *fakeEngine) Navigate(_ context.Context, target string) error {
	f.url = target
	return nil
}

func (f *fakeEngine) WaitForSelector(ctx context.Context, _ string) error {
	if !f.blockWait {
		return nil
	}
	<-ctx.Done()
	return ctx.Err()
}

func (f *fakeEngine) Click(context.Context, string) error {
	return nil
}

func (f *fakeEngine) Fill(_ context.Context, selector string, value string) error {
	if f.fills == nil {
		f.fills = map[string]string{}
	}
	f.fills[selector] = value
	return nil
}

func (f *fakeEngine) Title(context.Context) (string, error) {
	if f.title != "" {
		return f.title, nil
	}
	return "Example Domain", nil
}

func (f *fakeEngine) URL(context.Context) (string, error) {
	if f.url == "" {
		return "about:blank", nil
	}
	return f.url, nil
}

func (f *fakeEngine) Text(context.Context, string) (string, error) {
	return "Example Domain", nil
}

func (f *fakeEngine) Eval(context.Context, string) (string, error) {
	return "true", nil
}

func (f *fakeEngine) CaptureDOM(context.Context) (evidence.DomSnapshot, error) {
	return evidence.DomSnapshot{
		CapturedAt: time.Now().UTC().Format(time.RFC3339Nano),
		Text:       "Example Domain",
		HTML:       "<html><body>Example Domain</body></html>",
	}, nil
}

type deadlineEngine struct {
	fakeEngine
}

func (d *deadlineEngine) Navigate(context.Context, string) error {
	return context.DeadlineExceeded
}

func (f *fakeEngine) ConsoleEvents() []evidence.ConsoleEvent {
	return nil
}

func (f *fakeEngine) NetworkEvents() []evidence.NetworkEvent {
	return nil
}

type screenshotEngine struct {
	fakeEngine
}

type performanceEngine struct {
	fakeEngine
}

func (f *performanceEngine) CapturePerformance(context.Context) (evidence.PerformanceMetrics, error) {
	return evidence.PerformanceMetrics{
		TTFBMS:             12,
		LoadingTimeMS:      123,
		LCPMS:              98,
		CLS:                0.03,
		DOMContentLoadedMS: 87,
		LoadEventEndMS:     123,
	}, nil
}

func (f *screenshotEngine) CaptureScreenshot(_ context.Context, path string, _ bool) (string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte("png"), 0o644); err != nil {
		return "", fmt.Errorf("write screenshot: %w", err)
	}
	return path, nil
}

type configureFailEngine struct {
	fakeEngine
}

func (f *configureFailEngine) ConfigureBrowser(context.Context, BrowserConfig) error {
	return errors.New("configure boom")
}
