// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package runner

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/evidence"
	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/script"
)

type screenshotTestEngine struct {
	captures      int
	text          string
	screenshotErr error
}

func (e *screenshotTestEngine) Close(context.Context) error                   { return nil }
func (e *screenshotTestEngine) Navigate(context.Context, string) error        { return nil }
func (e *screenshotTestEngine) WaitForSelector(context.Context, string) error { return nil }
func (e *screenshotTestEngine) Click(context.Context, string) error           { return nil }
func (e *screenshotTestEngine) Fill(context.Context, string, string) error    { return nil }
func (e *screenshotTestEngine) Title(context.Context) (string, error)         { return "title", nil }
func (e *screenshotTestEngine) URL(context.Context) (string, error) {
	return "https://example.com", nil
}
func (e *screenshotTestEngine) Text(context.Context, string) (string, error) {
	if e.text == "" {
		return "actual", nil
	}
	return e.text, nil
}
func (e *screenshotTestEngine) Eval(context.Context, string) (string, error) { return "", nil }
func (e *screenshotTestEngine) CaptureDOM(context.Context) (evidence.DomSnapshot, error) {
	return evidence.DomSnapshot{}, nil
}
func (e *screenshotTestEngine) ConsoleEvents() []evidence.ConsoleEvent { return nil }
func (e *screenshotTestEngine) NetworkEvents() []evidence.NetworkEvent { return nil }

func (e *screenshotTestEngine) CaptureScreenshot(_ context.Context, path string, _ bool) (string, error) {
	e.captures++
	if e.screenshotErr != nil {
		return "", e.screenshotErr
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte("screenshot"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func TestLightpandaFailureScreenshotIsEnabled(t *testing.T) {
	engine := &screenshotTestEngine{}
	result := RunScript(context.Background(), script.Script{
		Name:      "failure screenshot",
		Target:    "https://example.com",
		TimeoutMS: 1_000,
		Steps: []script.Step{
			{Name: "open", Action: "goto"},
			{Name: "fail", Action: "assert_text", Selector: "#status", Contains: "expected", TimeoutMS: 1},
		},
	}, Options{
		EngineName:          "lightpanda",
		ScreenshotOnFailure: true,
		ScreenshotDir:       t.TempDir(),
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})

	if result.Success {
		t.Fatal("RunScript() success = true, want failure")
	}
	if engine.captures != 1 {
		t.Fatalf("screenshot captures = %d, want 1", engine.captures)
	}
	if len(result.Steps) != 2 || result.Steps[1].Screenshot == "" {
		t.Fatalf("failed step screenshot missing: %#v", result.Steps)
	}
	if _, err := os.Stat(result.Steps[1].Screenshot); err != nil {
		t.Fatalf("stat failed step screenshot: %v", err)
	}
}

func TestScreenshotRemainsOptIn(t *testing.T) {
	engine := &screenshotTestEngine{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := RunScript(ctx, script.Script{
		Name:      "failure without screenshot",
		Target:    "https://example.com",
		TimeoutMS: 500,
		Steps: []script.Step{
			{Name: "fail", Action: "assert_text", Selector: "#status", Contains: "expected", TimeoutMS: 1},
		},
	}, Options{
		EngineName: "lightpanda",
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})

	if result.Success {
		t.Fatal("RunScript() success = true, want failure")
	}
	if engine.captures != 0 {
		t.Fatalf("screenshot captures = %d, want 0", engine.captures)
	}
}

func TestFailureReportsScreenshotCaptureError(t *testing.T) {
	engine := &screenshotTestEngine{screenshotErr: errors.New("capture denied")}
	result := RunScript(context.Background(), script.Script{
		Name:      "screenshot error",
		Target:    "https://example.com",
		TimeoutMS: 500,
		Steps: []script.Step{
			{Name: "fail", Action: "assert_text", Selector: "#status", Contains: "expected", TimeoutMS: 1},
		},
	}, Options{
		EngineName:          "lightpanda",
		ScreenshotOnFailure: true,
		ScreenshotDir:       t.TempDir(),
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			return engine, nil
		},
	})

	if len(result.Steps) != 1 || result.Steps[0].Error == nil {
		t.Fatalf("failed step error missing: %#v", result.Steps)
	}
	if !strings.Contains(result.Steps[0].Error.Message, "screenshot capture unavailable: capture denied") {
		t.Fatalf("failed step error = %q", result.Steps[0].Error.Message)
	}
}

func TestRetryCleansScreenshotWhenLaterAttemptSucceeds(t *testing.T) {
	screenshotDir := t.TempDir()
	attempt := 0
	result := RunScript(context.Background(), script.Script{
		Name:      "retry screenshot cleanup",
		Target:    "https://example.com",
		TimeoutMS: 1_000,
		Steps: []script.Step{
			{Name: "assert", Action: "assert_text", Selector: "#status", Contains: "expected", TimeoutMS: 1},
		},
	}, Options{
		EngineName:          "lightpanda",
		RetryCount:          1,
		ScreenshotOnFailure: true,
		ScreenshotDir:       screenshotDir,
		EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
			attempt++
			if attempt == 1 {
				return &screenshotTestEngine{text: "actual"}, nil
			}
			return &screenshotTestEngine{text: "expected"}, nil
		},
	})

	if !result.Success {
		t.Fatalf("RunScript() success = false, want success: %#v", result.Error)
	}
	entries, err := os.ReadDir(screenshotDir)
	if err != nil {
		t.Fatalf("read screenshot directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("screenshot directory contains %d leaked entries", len(entries))
	}
}
