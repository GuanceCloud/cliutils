// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"context"
	"fmt"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/stretchr/testify/require"
)

func TestChromeNetworkPayloadBuffers(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
	if binary == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH to run the browser integration test")
	}
	const smallBytes, largeBytes, smallCount = 128 << 10, 512 << 10, 9
	expectedBytes := smallBytes*smallCount + largeBytes
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.URL.Path == "/payload" {
			cookie, err := r.Cookie("probe")
			if r.Header.Get("X-Probe") != "local" || err != nil || cookie.Value != "local" {
				http.Error(w, "missing configured header/cookie", http.StatusForbidden)
				return
			}
			size := smallBytes
			if r.URL.Query().Get("i") == "large" {
				size = largeBytes
			}
			w.Header().Set("Content-Type", "text/plain")
			if _, err := io.WriteString(w, strings.Repeat("a", size)); err != nil {
				t.Errorf("write payload: %v", err)
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := fmt.Fprintf(w, `<!doctype html><input id="probe"><button id="click"
onclick="setTimeout(()=>document.body.insertAdjacentHTML('beforeend','<span id=clicked>yes</span>'),10)">中文</button>
<script>(async()=>{let total=0;for(const i of [...Array(%d).keys(),'large']){
const response=await fetch('/payload?i='+i,{cache:'no-store'});if(!response.ok)throw Error('request failed');
const text=await response.text();const expected=i==='large'?%d:%d;
if(text.length!==expected||text[0]!=='a'||text[text.length-1]!=='a')throw Error('incomplete response');total+=text.length}
console.log('payload-ready');document.body.insertAdjacentHTML('beforeend','<span id=ready>'+total+'</span>')})()</script>`,
			smallCount, largeBytes, smallBytes)
		if err != nil {
			t.Errorf("write page: %v", err)
		}
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base, err := NewEngine(ctx, runner.EngineOptions{ChromePath: binary, ViewportWidth: 800, ViewportHeight: 600})
	require.NoError(t, err)
	engine := base.(*Engine)
	defer func() { require.NoError(t, engine.Close(context.Background())) }()
	require.NoError(t, engine.ConfigureBrowser(ctx, runner.BrowserConfig{
		Target: server.URL, Headers: map[string]string{"X-Probe": "local"},
		Cookies: []runner.BrowserCookie{{Name: "probe", Value: "local", Path: "/"}},
	}))
	require.NoError(t, engine.Navigate(ctx, server.URL))
	require.NoError(t, engine.WaitForSelector(ctx, "#ready"))
	text, err := engine.Text(ctx, "#ready")
	require.NoError(t, err)
	require.Equal(t, strconv.Itoa(expectedBytes), text)
	require.NoError(t, engine.Fill(ctx, "#probe", "still functional"))
	require.NoError(t, engine.Click(ctx, "#click"))
	require.NoError(t, engine.WaitForSelector(ctx, "#clicked"))
	responses := 0
	for _, event := range engine.NetworkEvents() {
		if event.Event == "response" && strings.Contains(event.URL, "/payload?") {
			require.EqualValues(t, http.StatusOK, event.Status)
			responses++
		}
	}
	require.Equal(t, smallCount+1, responses)
	require.NotEmpty(t, engine.ConsoleEvents())
	var payloadIDs []network.RequestID
	engine.mu.Lock()
	for id, info := range engine.requests {
		if info.URL == server.URL+"/payload?i=0" || info.URL == server.URL+"/payload?i=large" {
			payloadIDs = append(payloadIDs, id)
		}
	}
	engine.mu.Unlock()
	require.Len(t, payloadIDs, 2)
	for _, id := range payloadIDs {
		var bodyErr error
		require.NoError(t, engine.run(engine.ctx, chromedp.ActionFunc(func(actionCtx context.Context) error {
			_, bodyErr = network.GetResponseBody(id).Do(actionCtx)
			return nil
		})))
		require.Error(t, bodyErr, "old/large inspector body should be evicted while the page receives it")
	}
	screenshot, err := engine.CaptureScreenshot(ctx, filepath.Join(t.TempDir(), "page.png"), false)
	require.NoError(t, err)
	image, err := os.Open(screenshot)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, image.Close()) })
	config, err := png.DecodeConfig(image)
	require.NoError(t, err)
	require.Equal(t, 800, config.Width)
	require.Equal(t, 600, config.Height)
	_, err = engine.CapturePerformance(ctx)
	require.NoError(t, err)
}
