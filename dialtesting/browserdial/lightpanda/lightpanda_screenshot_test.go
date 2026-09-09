// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package lightpanda

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
)

type screenshotExecutor struct {
	image  []byte
	params page.CaptureScreenshotParams
}

func (e *screenshotExecutor) Execute(_ context.Context, method string, params, result any) error {
	if method != page.CommandCaptureScreenshot {
		return fmt.Errorf("unexpected CDP method %q", method)
	}
	captureParams, ok := params.(*page.CaptureScreenshotParams)
	if !ok {
		return fmt.Errorf("unexpected screenshot params type %T", params)
	}
	e.params = *captureParams
	captureResult, ok := result.(*page.CaptureScreenshotReturns)
	if !ok {
		return fmt.Errorf("unexpected screenshot result type %T", result)
	}
	captureResult.Data = base64.StdEncoding.EncodeToString(e.image)
	return nil
}

func runWithExecutor(executor cdp.Executor) func(context.Context, ...chromedp.Action) error {
	return func(ctx context.Context, actions ...chromedp.Action) error {
		ctx = cdp.WithExecutor(ctx, executor)
		for _, action := range actions {
			if err := action.Do(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestCaptureScreenshot(t *testing.T) {
	tests := []struct {
		name           string
		requestedPath  string
		fullPage       bool
		wantPath       string
		beyondViewport bool
	}{
		{
			name:          "viewport PNG",
			requestedPath: "step-1.png",
			wantPath:      "step-1.png",
		},
		{
			name:           "full page remains PNG",
			requestedPath:  "step-2.jpg",
			fullPage:       true,
			wantPath:       "step-2.png",
			beyondViewport: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			image := []byte("png screenshot evidence")
			executor := &screenshotExecutor{image: image}
			engine := &Engine{
				ctx: context.Background(),
				run: runWithExecutor(executor),
			}
			requestedPath := filepath.Join(t.TempDir(), "nested", test.requestedPath)
			gotPath, err := engine.CaptureScreenshot(context.Background(), requestedPath, test.fullPage)
			if err != nil {
				t.Fatalf("CaptureScreenshot() error = %v", err)
			}
			if want := filepath.Join(filepath.Dir(requestedPath), test.wantPath); gotPath != want {
				t.Fatalf("CaptureScreenshot() path = %q, want %q", gotPath, want)
			}
			gotImage, err := os.ReadFile(gotPath)
			if err != nil {
				t.Fatalf("read screenshot: %v", err)
			}
			if !bytes.Equal(gotImage, image) {
				t.Fatalf("screenshot data = %q, want %q", gotImage, image)
			}
			if executor.params.Format != page.CaptureScreenshotFormatPng {
				t.Fatalf("screenshot format = %q, want png", executor.params.Format)
			}
			if executor.params.CaptureBeyondViewport != test.beyondViewport {
				t.Fatalf("captureBeyondViewport = %t, want %t", executor.params.CaptureBeyondViewport, test.beyondViewport)
			}
		})
	}
}

func TestCaptureScreenshotRejectsEmptyImage(t *testing.T) {
	executor := &screenshotExecutor{}
	engine := &Engine{
		ctx: context.Background(),
		run: runWithExecutor(executor),
	}
	path := filepath.Join(t.TempDir(), "step.png")
	if _, err := engine.CaptureScreenshot(context.Background(), path, false); err == nil {
		t.Fatal("CaptureScreenshot() error = nil, want empty screenshot error")
	}
}
