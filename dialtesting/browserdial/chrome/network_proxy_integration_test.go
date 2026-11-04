// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/script"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/require"
)

func TestChromeNetworkProxy(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
	if binary == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH to run the browser integration test")
	}
	var forbiddenHits atomic.Int32
	blocked := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		forbiddenHits.Add(1)
		_, _ = io.WriteString(w, "must not arrive")
	}))
	listener, err := net.Listen("tcp4", "127.0.0.2:0")
	require.NoError(t, err)
	require.NoError(t, blocked.Listener.Close())
	blocked.Listener = listener
	blocked.Start()
	defer blocked.Close()
	page := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ws" {
			conn, _, _, err := ws.UpgradeHTTP(r, w)
			if err != nil {
				return
			}
			defer conn.Close() //nolint:errcheck
			data, opcode, err := wsutil.ReadClientData(conn)
			if err == nil {
				_ = wsutil.WriteServerMessage(conn, opcode, data)
			}
			return
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, blocked.URL+"/redirected", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprintf(w, `<!doctype html><title>proxy ready</title><input id="input"><button id="button"
onclick="document.querySelector('#result').textContent=document.querySelector('#input').value">click</button>
<span id="result"></span><script>(async()=>{
let denied=0;
for(const url of [%q, '/redirect']) {
try { const r=await fetch(url); if(!r.ok)denied++ } catch(e){ denied++ }
}
document.body.insertAdjacentHTML('beforeend','<span id="denied">'+denied+'</span>');
const echo=await new Promise((resolve,reject)=>{
const ws=new WebSocket(location.origin.replace(/^http/,'ws')+'/ws');
ws.onopen=()=>ws.send('proxy echo');ws.onmessage=e=>{resolve(e.data);ws.close()};ws.onerror=reject;
});document.body.insertAdjacentHTML('beforeend','<span id="echo">'+echo+'</span>');
})()</script>`, blocked.URL+"/fetch")
	}))
	defer page.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	base, err := NewEngine(ctx, runner.EngineOptions{
		ChromePath: binary, BlockPrivateNetwork: true, PrivateNetworkCIDRs: []string{"127.0.0.2/32"},
	})
	require.NoError(t, err)
	engine := base.(*Engine)
	defer engine.Close(context.Background()) //nolint:errcheck
	require.NoError(t, engine.Navigate(ctx, page.URL))
	require.NoError(t, engine.WaitForSelector(ctx, "#denied"))
	text, err := engine.Text(ctx, "#denied")
	require.NoError(t, err)
	require.Equal(t, "2", text)
	require.NoError(t, engine.Fill(ctx, "#input", "works through proxy"))
	require.NoError(t, engine.Click(ctx, "#button"))
	text, err = engine.Text(ctx, "#result")
	require.NoError(t, err)
	require.Equal(t, "works through proxy", text)
	require.NoError(t, engine.WaitForSelector(ctx, "#echo"))
	text, err = engine.Text(ctx, "#echo")
	require.NoError(t, err)
	require.Equal(t, "proxy echo", text)
	require.Error(t, engine.Navigate(ctx, strings.Replace(blocked.URL, "http://", "https://", 1)))
	require.Zero(t, forbiddenHits.Load())

	// Chrome's localhost bypass must also be disabled for the default policy.
	guarded, err := NewEngine(ctx, runner.EngineOptions{ChromePath: binary, BlockPrivateNetwork: true})
	require.NoError(t, err)
	defer guarded.Close(context.Background()) //nolint:errcheck
	require.NoError(t, guarded.Navigate(ctx, page.URL))
	text, err = guarded.Text(ctx, "body")
	require.NoError(t, err)
	require.Contains(t, text, "blocked by network policy")
}

func TestChromeTrustedNodeProxy(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
	if binary == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH to run the browser integration test")
	}
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<title>trusted upstream</title>")
	}))
	defer origin.Close()
	upstream := testNetworkProxy(t, runner.EngineOptions{PrivateNetworkCIDRs: []string{"203.0.113.0/24"}})
	front := httptest.NewUnstartedServer(upstream.handler())
	listener, err := net.Listen("tcp4", "127.0.0.2:0")
	require.NoError(t, err)
	require.NoError(t, front.Listener.Close())
	front.Listener = &proxyListener{Listener: listener, proxy: upstream}
	front.Start()
	defer front.Close()
	var blockedHits atomic.Int32
	blocked := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		blockedHits.Add(1)
		_, _ = io.WriteString(w, "forbidden origin")
	}))
	listener, err = net.Listen("tcp4", "127.0.0.2:0")
	require.NoError(t, err)
	require.NoError(t, blocked.Listener.Close())
	blocked.Listener = listener
	blocked.StartTLS()
	defer blocked.Close()
	for _, test := range []struct {
		name, target                          string
		taskOverride, scriptOverride, success bool
	}{
		{name: "node internal upstream", target: origin.URL, success: true},
		{name: "task override remains restricted", target: origin.URL, taskOverride: true},
		{name: "script override remains restricted", target: origin.URL, scriptOverride: true},
		{name: "origin remains restricted", target: blocked.URL},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			loaded := script.Script{
				Target: test.target, IgnoreHTTPSErrors: true,
				Steps: []script.Step{{Action: "goto", URL: test.target}, {Action: "assert_title", Equals: "trusted upstream"}},
			}
			options := runner.Options{
				DefaultProxyURL: front.URL, BlockPrivateNetwork: true,
				PrivateNetworkCIDRs: []string{"127.0.0.2/32"}, TimeoutMS: 8000,
				EngineFactory: func(ctx context.Context, options runner.EngineOptions) (runner.Engine, error) {
					options.ChromePath = binary
					return NewEngine(ctx, options)
				},
			}
			if test.taskOverride {
				options.ProxyURL = front.URL
			}
			if test.scriptOverride {
				loaded.ProxyURL = front.URL
			}
			result := runner.RunScript(ctx, loaded, options)
			require.Equal(t, test.success, result.Success, "%+v", result.Error)
		})
	}
	require.Zero(t, blockedHits.Load(), "node proxy trust must not allow forbidden origins")
}

func TestChromeProxyWebRTCUDP(t *testing.T) {
	binary := os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
	if binary == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH to run the browser integration test")
	}
	for _, guarded := range []bool{false, true} {
		t.Run(fmt.Sprint(guarded), func(t *testing.T) {
			udp, err := net.ListenPacket("udp4", "0.0.0.0:0")
			require.NoError(t, err)
			defer udp.Close() //nolint:errcheck
			_, port, err := net.SplitHostPort(udp.LocalAddr().String())
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			options := runner.EngineOptions{ChromePath: binary}
			if guarded {
				options.PrivateNetworkCIDRs = []string{"203.0.113.0/24"}
			}
			base, err := NewEngine(ctx, options)
			require.NoError(t, err)
			defer base.Close(context.Background()) //nolint:errcheck
			engine := base.(*Engine)
			var result string
			script := fmt.Sprintf(`(async()=>{
const pc=new RTCPeerConnection({iceServers:[{urls:%q}]});
pc.createDataChannel('probe');await pc.setLocalDescription(await pc.createOffer());
await new Promise(r=>setTimeout(r,1200));pc.close();return 'complete';})()`, "stun:127.0.0.1:"+port)
			require.NoError(t, engine.run(engine.ctx, chromedp.Evaluate(script, &result, chromedp.EvalAsValue,
				func(p *runtime.EvaluateParams) *runtime.EvaluateParams { return p.WithAwaitPromise(true) })))
			require.Equal(t, "complete", result)
			require.NoError(t, udp.SetReadDeadline(time.Now().Add(50*time.Millisecond)))
			data := make([]byte, 2048)
			_, _, err = udp.ReadFrom(data)
			if guarded {
				require.Error(t, err, "managed browser must not emit direct STUN UDP")
			} else {
				require.NoError(t, err, "control browser must exercise the UDP path")
			}
		})
	}
}
