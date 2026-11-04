// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/GuanceCloud/cliutils/logger"
	"github.com/chromedp/chromedp"
)

func startChrome(
	ctx context.Context, executable string, width, height int, options runner.EngineOptions,
) (context.Context, context.CancelFunc, error) {
	// Only node configuration may permit unsandboxed execution. Non-root
	// browsers still try the sandbox first when compatibility mode is allowed.
	noSandbox := options.AllowNoSandbox && os.Getuid() == 0
	if os.Getuid() == 0 && !options.AllowNoSandbox {
		return nil, nil, errors.New("Chrome sandbox requires a non-root user; browser.allow_no_sandbox is false")
	}
	if noSandbox {
		logger.SLogger("dialtesting").Warn("Chrome sandbox disabled for root by browser.allow_no_sandbox")
	}
	for {
		allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, chromeAllocatorOptions(executable, width, height, options, noSandbox)...)
		browserCtx, browserCancel := chromedp.NewContext(allocCtx)
		cancel := func() {
			browserCancel()
			allocCancel()
		}
		err := chromedp.Run(browserCtx)
		if err == nil {
			return browserCtx, cancel, nil
		}
		cancel()
		if !options.AllowNoSandbox || noSandbox || ctx.Err() != nil || !chromeSandboxUnavailable(err) {
			return nil, nil, err
		}
		// An allocator cannot be reused after startup fails. The next attempt
		// gets a fresh process and retries only this specific sandbox failure.
		logger.SLogger("dialtesting").Warn("Chrome sandbox is unavailable; retrying browser startup with --no-sandbox")
		noSandbox = true
	}
}

func chromeSandboxUnavailable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no usable sandbox") ||
		(strings.Contains(message, "failed to move to new namespace") && strings.Contains(message, "operation not permitted"))
}
