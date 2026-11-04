// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/script"
	"github.com/gobwas/ws"
	"github.com/gobwas/ws/wsutil"
	"github.com/stretchr/testify/require"
)

type protocolFixture struct {
	Targets []string `json:"targets"`
	Proxies []string `json:"proxies"`
}

func TestChromeProxyProtocolMatrix(t *testing.T) {
	if os.Getenv("CHROME_TEST_EXECUTABLE_PATH") == "" {
		t.Skip("set CHROME_TEST_EXECUTABLE_PATH to run the browser integration test")
	}
	if os.Getenv("DATAKIT_CHROME_PROXY_TEST_CHILD") == "1" {
		runProtocolMatrixChild(t)
		return
	}
	var http1, http2, tls12, tls13, plainWS, secureWS, authenticated atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Proxy-Authorization"), "proxy credentials must not reach the origin")
		if r.URL.Path == "/ws" {
			conn, _, _, err := ws.UpgradeHTTP(r, w)
			if err != nil {
				return
			}
			defer conn.Close() //nolint:errcheck
			data, opcode, err := wsutil.ReadClientData(conn)
			if err == nil {
				err = wsutil.WriteServerMessage(conn, opcode, data)
			}
			if err == nil {
				if r.TLS == nil {
					plainWS.Add(1)
				} else {
					secureWS.Add(1)
				}
			}
			return
		}
		if r.URL.Path == "/" {
			if r.ProtoMajor == 1 {
				http1.Add(1)
			} else if r.ProtoMajor == 2 {
				http2.Add(1)
			}
			if r.TLS != nil {
				switch r.TLS.Version {
				case tls.VersionTLS12:
					tls12.Add(1)
				case tls.VersionTLS13:
					tls13.Add(1)
				}
			}
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, `<!doctype html><title>protocol matrix</title><script>
const socket=new WebSocket(location.origin.replace(/^http/,'ws')+'/ws');
socket.onopen=()=>socket.send('protocol echo');
socket.onmessage=e=>{document.body.insertAdjacentHTML('beforeend','<span id=ready>'+e.data+'</span>');socket.close()};
</script>`)
	})
	var targets []string
	for _, version := range []uint16{0, tls.VersionTLS12, tls.VersionTLS13} {
		origin := httptest.NewUnstartedServer(handler)
		listener, err := net.Listen("tcp4", "127.0.0.2:0")
		require.NoError(t, err)
		require.NoError(t, origin.Listener.Close())
		origin.Listener = listener
		if version == 0 {
			origin.Start()
		} else {
			origin.EnableHTTP2 = true
			origin.TLS = &tls.Config{MinVersion: version, MaxVersion: version}
			origin.StartTLS()
		}
		t.Cleanup(origin.Close)
		targets = append(targets, origin.URL)
	}
	upstream := testNetworkProxy(t, runner.EngineOptions{PrivateNetworkCIDRs: []string{"203.0.113.0/24"}})
	proxyHandler := upstream.handler()
	var proxies []string
	var rootCert []byte
	for _, secure := range []bool{false, true} {
		front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Proxy-Authorization") != "Basic dXNlcjpzZWNyZXQ=" {
				w.WriteHeader(http.StatusProxyAuthRequired)
				return
			}
			authenticated.Add(1)
			proxyHandler.ServeHTTP(w, r)
		}))
		front.Listener = &proxyListener{Listener: front.Listener, proxy: upstream}
		if secure {
			front.StartTLS()
			rootCert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: front.Certificate().Raw})
		} else {
			front.Start()
		}
		t.Cleanup(front.Close)
		address, err := url.Parse(front.URL)
		require.NoError(t, err)
		address.User = url.UserPassword("user", "secret")
		proxies = append(proxies, address.String())
	}
	certFile := filepath.Join(t.TempDir(), "proxy-root.pem")
	require.NoError(t, os.WriteFile(certFile, rootCert, 0o600))
	config, err := json.Marshal(protocolFixture{Targets: targets, Proxies: proxies})
	require.NoError(t, err)
	// System roots are cached by crypto/x509. A fresh child verifies the HTTPS
	// upstream using a fixture CA without changing the host's trust store or
	// adding a certificate-verification bypass to production proxy code.
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestChromeProxyProtocolMatrix$", "-test.v", "-test.timeout=40s")
	command.Env = append(os.Environ(), "DATAKIT_CHROME_PROXY_TEST_CHILD=1", "SSL_CERT_FILE="+certFile, "SSL_CERT_DIR="+t.TempDir())
	command.Stdin = strings.NewReader(string(config))
	output, err := command.CombinedOutput()
	t.Log(string(output))
	require.NoError(t, err)
	require.GreaterOrEqual(t, http1.Load(), int32(2))
	require.GreaterOrEqual(t, http2.Load(), int32(4), "verify actual HTTP/2 negotiation, not just HTTPS success")
	require.GreaterOrEqual(t, tls12.Load(), int32(2))
	require.GreaterOrEqual(t, tls13.Load(), int32(2))
	require.GreaterOrEqual(t, plainWS.Load(), int32(2))
	require.GreaterOrEqual(t, secureWS.Load(), int32(4))
	require.GreaterOrEqual(t, authenticated.Load(), int32(6))
	t.Logf("verified HTTP/1.1=%d HTTP/2=%d TLS1.2=%d TLS1.3=%d WS=%d WSS=%d authenticated tunnels=%d",
		http1.Load(), http2.Load(), tls12.Load(), tls13.Load(), plainWS.Load(), secureWS.Load(), authenticated.Load())
}

func runProtocolMatrixChild(t *testing.T) {
	t.Helper()
	var config protocolFixture
	require.NoError(t, json.NewDecoder(os.Stdin).Decode(&config))
	for i, proxy := range config.Proxies {
		for j, target := range config.Targets {
			t.Run(fmt.Sprintf("upstream-%d/origin-%d", i, j), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				result := runner.RunScript(ctx, script.Script{
					Target: target, IgnoreHTTPSErrors: true,
					Steps: []script.Step{{Action: "goto", URL: target}, {Action: "wait_for_selector", Selector: "#ready"},
						{Action: "assert_text", Selector: "#ready", Equals: "protocol echo"}},
				}, runner.Options{
					DefaultProxyURL: proxy, BlockPrivateNetwork: true, PrivateNetworkCIDRs: []string{"127.0.0.1/32"}, TimeoutMS: 8000,
					EngineFactory: func(ctx context.Context, options runner.EngineOptions) (runner.Engine, error) {
						options.ChromePath = os.Getenv("CHROME_TEST_EXECUTABLE_PATH")
						return NewEngine(ctx, options)
					},
				})
				require.True(t, result.Success, "%+v", result.Error)
			})
		}
	}
}
