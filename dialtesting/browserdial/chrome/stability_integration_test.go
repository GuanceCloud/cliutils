// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/script"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
)

type stabilityTracker struct {
	mu              sync.Mutex
	processes       map[int]bool
	profiles        map[string]bool
	proxies         map[string]bool
	created, closed int
	binary          string
}

func (s *stabilityTracker) trackTree(root int) {
	entries, _ := os.ReadDir("/proc")
	children := make(map[int][]int)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", entry.Name(), "stat"))
		if err != nil {
			continue
		}
		fields := strings.Fields(string(data[bytes.LastIndexByte(data, ')')+1:]))
		if len(fields) < 2 {
			continue
		}
		parent, err := strconv.Atoi(fields[1])
		if err == nil {
			children[parent] = append(children[parent], pid)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var visit func(int)
	visit = func(pid int) {
		s.processes[pid] = true
		for _, child := range children[pid] {
			visit(child)
		}
	}
	visit(root)
}

func (s *stabilityTracker) factory(ctx context.Context, options runner.EngineOptions) (runner.Engine, error) {
	options.ChromePath = s.binary
	base, err := NewEngine(ctx, options)
	if err != nil {
		return nil, err
	}
	engine := base.(*Engine)
	process := chromedp.FromContext(engine.ctx).Browser.Process()
	command, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", process.Pid))
	if err != nil {
		_ = engine.Close(ctx)
		return nil, err
	}
	s.mu.Lock()
	s.created++
	for value := range strings.SplitSeq(string(command), "\x00") {
		if path, ok := strings.CutPrefix(value, "--user-data-dir="); ok {
			s.profiles[path] = true
		}
		if address, ok := strings.CutPrefix(value, "--proxy-server=http://"); ok {
			s.proxies[address] = true
		}
	}
	s.mu.Unlock()
	s.trackTree(process.Pid)
	closeEngine := engine.cancel
	engine.cancel = func() {
		s.trackTree(process.Pid)
		closeEngine()
		s.mu.Lock()
		s.closed++
		s.mu.Unlock()
	}
	return engine, nil
}

func (s *stabilityTracker) remaining() (processes, profiles, listeners int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for pid := range s.processes {
		if _, err := os.Stat(fmt.Sprintf("/proc/%d", pid)); err == nil {
			processes++
		}
	}
	for path := range s.profiles {
		if _, err := os.Stat(path); err == nil {
			profiles++
		}
	}
	for address := range s.proxies {
		conn, err := net.DialTimeout("tcp", address, 50*time.Millisecond)
		if err == nil {
			listeners++
			_ = conn.Close()
		}
	}
	return
}

func stabilityFootprint(t *testing.T) (fds, goroutines int, heap uint64) {
	t.Helper()
	runtime.GC()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	return len(entries), runtime.NumGoroutine(), memory.HeapAlloc
}

func stabilityScript(target, token string) script.Script {
	return script.Script{Target: target, Steps: []script.Step{
		{Action: "goto", URL: target},
		{Action: "eval", Value: fmt.Sprintf(`if(localStorage.getItem('tenant')!==null||document.cookie.includes('tenant='))throw Error('previous task state');localStorage.setItem('tenant',%q);document.cookie='tenant='+%q`, token, token)},
		{Action: "goto", URL: target},
		{Action: "eval", Value: fmt.Sprintf(`if(localStorage.getItem('tenant')!==%q||!document.cookie.includes('tenant='+%q))throw Error('task state changed')`, token, token)},
		{Action: "fill", Selector: "#input", Value: token},
		{Action: "click", Selector: "#submit"},
		{Action: "wait_for_selector", Selector: "#done"},
		{Action: "assert_text", Selector: "#done", Equals: token},
	}}
}

func TestChromeTaskStability(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
	if binary == "" || runtime.GOOS != "linux" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH on Linux for the task stability regression")
	}
	profiles := t.TempDir()
	t.Setenv("TMPDIR", profiles)
	tracker := &stabilityTracker{binary: binary, processes: make(map[int]bool), profiles: make(map[string]bool), proxies: make(map[string]bool)}
	cancelEntered, crashEntered := make(chan struct{}), make(chan struct{})
	var crashVisits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/hang" {
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/cancel" {
			close(cancelEntered)
			<-r.Context().Done()
			return
		}
		if r.URL.Path == "/crash" && crashVisits.Add(1) == 1 {
			close(crashEntered)
			<-r.Context().Done()
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<!doctype html><title>stable</title><input id=input><button id=submit onclick="setTimeout(()=>document.body.insertAdjacentHTML('beforeend','<span id=done>'+document.querySelector('#input').value+'</span>'),10)">Continue</button>`)
	}))
	defer server.Close()
	options := runner.Options{EngineName: "chrome", TimeoutMS: 8000, EngineFactory: tracker.factory,
		PrivateNetworkCIDRs: []string{"127.0.0.2/32"}, ScreenshotDir: t.TempDir(), SyntheticsTestID: "stability-task"}
	var count int
	runGood := func(t *testing.T) {
		t.Helper()
		count++
		result := runner.RunScript(context.Background(), stabilityScript(server.URL, fmt.Sprint(count)), options)
		require.True(t, result.Success, "%+v", result.Error)
	}
	runGood(t)
	time.Sleep(100 * time.Millisecond)
	fdsBefore, goroutinesBefore, heapBefore := stabilityFootprint(t)
	t.Run("repeated tasks", func(t *testing.T) {
		for range 12 {
			runGood(t)
		}
	})
	t.Run("four concurrent isolated tasks", func(t *testing.T) {
		for round := range 6 {
			var group sync.WaitGroup
			results := make([]runner.Result, 4)
			for i := range results {
				group.Add(1)
				go func(i int) {
					defer group.Done()
					results[i] = runner.RunScript(context.Background(), stabilityScript(server.URL, fmt.Sprintf("%d-%d", round, i)), options)
				}(i)
			}
			group.Wait()
			for _, result := range results {
				require.True(t, result.Success, "%+v", result.Error)
			}
		}
	})
	t.Run("timeout then recovery", func(t *testing.T) {
		short := options
		short.TimeoutMS = 1500
		result := runner.RunScript(context.Background(), script.Script{Target: server.URL + "/hang", Steps: []script.Step{{Action: "goto", URL: server.URL + "/hang"}}}, short)
		require.False(t, result.Success)
		require.Less(t, result.DurationUS, int64((5*time.Second)/time.Microsecond))
		runGood(t)
	})
	t.Run("cancel then recovery", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan runner.Result, 1)
		go func() {
			done <- runner.RunScript(ctx, script.Script{Target: server.URL + "/cancel", Steps: []script.Step{{Action: "goto", URL: server.URL + "/cancel"}}}, options)
		}()
		select {
		case <-cancelEntered:
		case <-time.After(5 * time.Second):
			t.Fatal("task did not enter navigation")
		}
		cancel()
		select {
		case result := <-done:
			require.False(t, result.Success)
		case <-time.After(5 * time.Second):
			t.Fatal("canceled task did not finish")
		}
		runGood(t)
	})
	t.Run("assertion failure retains screenshot then recovery", func(t *testing.T) {
		failure := options
		failure.ScreenshotOnFailure = true
		result := runner.RunScript(context.Background(), script.Script{Target: server.URL, Steps: []script.Step{
			{Action: "goto", URL: server.URL}, {Action: "assert_title", Equals: "wrong title", TimeoutMS: 100},
		}}, failure)
		require.False(t, result.Success)
		var screenshot string
		for _, step := range result.Steps {
			if step.Screenshot != "" {
				screenshot = step.Screenshot
			}
		}
		require.NotEmpty(t, screenshot)
		info, err := os.Stat(screenshot)
		require.NoError(t, err)
		require.Positive(t, info.Size())
		runGood(t)
	})
	t.Run("browser crash retries in fresh process", func(t *testing.T) {
		retry := options
		retry.RetryCount = 1
		process := make(chan *os.Process, 1)
		var launched atomic.Int32
		retry.EngineFactory = func(ctx context.Context, options runner.EngineOptions) (runner.Engine, error) {
			engine, err := tracker.factory(ctx, options)
			if err == nil && launched.Add(1) == 1 {
				process <- chromedp.FromContext(engine.(*Engine).ctx).Browser.Process()
			}
			return engine, err
		}
		done := make(chan runner.Result, 1)
		go func() {
			done <- runner.RunScript(context.Background(), script.Script{Target: server.URL + "/crash", Steps: []script.Step{
				{Action: "goto", URL: server.URL + "/crash"}, {Action: "assert_title", Equals: "stable"},
			}}, retry)
		}()
		browser := <-process
		select {
		case <-crashEntered:
		case <-time.After(5 * time.Second):
			t.Fatal("crash fixture was not reached")
		}
		tracker.trackTree(browser.Pid)
		require.NoError(t, browser.Kill())
		select {
		case result := <-done:
			require.True(t, result.Success, "%+v", result.Error)
			require.Equal(t, 2, result.Attempt)
			require.Len(t, result.RetryRecords, 2)
			require.False(t, result.RetryRecords[0].Success)
			require.True(t, result.RetryRecords[1].Success)
			require.NotEqual(t, result.RetryRecords[0].BrowserRunID, result.RetryRecords[1].BrowserRunID)
		case <-time.After(12 * time.Second):
			t.Fatal("crashed task did not recover")
		}
		runGood(t)
	})
	t.Run("startup failure then recovery", func(t *testing.T) {
		invalid := options
		invalid.EngineFactory = func(ctx context.Context, options runner.EngineOptions) (runner.Engine, error) {
			options.ChromePath = filepath.Join(profiles, "missing-browser")
			return NewEngine(ctx, options)
		}
		for range 4 {
			result := runner.RunScript(context.Background(), stabilityScript(server.URL, "invalid"), invalid)
			require.False(t, result.Success)
		}
		runGood(t)
	})
	require.Eventually(t, func() bool {
		processes, profiles, listeners := tracker.remaining()
		return processes == 0 && profiles == 0 && listeners == 0
	}, 3*time.Second, 100*time.Millisecond)
	fdsAfter, goroutinesAfter, heapAfter := stabilityFootprint(t)
	require.LessOrEqual(t, fdsAfter, fdsBefore+4)
	require.LessOrEqual(t, goroutinesAfter, goroutinesBefore+8)
	require.Equal(t, tracker.created, tracker.closed)
	left, err := os.ReadDir(profiles)
	require.NoError(t, err)
	require.Empty(t, left, "temporary browser profiles must be removed")
	t.Logf("created=%d closed=%d tracked_processes=%d profiles=%d proxies=%d fds=%d->%d goroutines=%d->%d live_heap_bytes=%d->%d",
		tracker.created, tracker.closed, len(tracker.processes), len(tracker.profiles), len(tracker.proxies), fdsBefore, fdsAfter, goroutinesBefore, goroutinesAfter, heapBefore, heapAfter)
}
