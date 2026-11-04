// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"bufio"
	"context"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
	"github.com/stretchr/testify/require"
)

func TestBrowserNetworkPolicy(t *testing.T) {
	policy, err := newNetworkPolicy(runner.EngineOptions{BlockPrivateNetwork: true})
	require.NoError(t, err)
	for _, address := range []string{
		"0.1.2.3", "10.1.2.3", "100.64.1.1", "127.0.0.1", "169.254.169.254", "172.16.1.1",
		"192.168.1.1", "198.18.0.1", "224.0.0.1", "255.255.255.255", "::", "::1", "fc00::1", "fe80::1", "ff02::1",
		"::ffff:127.0.0.1", "::ffff:169.254.169.254", "fe80::1%eth0",
	} {
		require.False(t, policy.allowed(netip.MustParseAddr(address)), address)
	}
	for _, address := range []string{"1.1.1.1", "2606:4700:4700::1111"} {
		require.True(t, policy.allowed(netip.MustParseAddr(address)), address)
	}
	policy, err = newNetworkPolicy(runner.EngineOptions{
		BlockPrivateNetwork: true, PrivateNetworkCIDRs: []string{"", " ::ffff:192.168.0.0/112 ", "2001:db8::/32"},
	})
	require.NoError(t, err)
	require.True(t, policy.allowed(netip.MustParseAddr("127.0.0.1")), "explicit CIDRs replace defaults")
	require.False(t, policy.allowed(netip.MustParseAddr("192.168.1.2")))
	require.False(t, policy.allowed(netip.MustParseAddr("::ffff:192.168.1.2")))
	require.False(t, policy.allowed(netip.MustParseAddr("2001:db8::1")))
	policy, err = newNetworkPolicy(runner.EngineOptions{PrivateNetworkCIDRs: []string{"::/0"}})
	require.NoError(t, err)
	require.True(t, policy.allowed(netip.MustParseAddr("1.1.1.1")), "IPv6-only CIDRs must not also block IPv4")
	require.False(t, policy.allowed(netip.MustParseAddr("2606:4700:4700::1111")))
	_, err = newNetworkPolicy(runner.EngineOptions{PrivateNetworkCIDRs: []string{"broken"}})
	require.Error(t, err)
}

func testNetworkProxy(t *testing.T, options runner.EngineOptions) *networkProxy {
	t.Helper()
	p, err := startNetworkProxy(context.Background(), options)
	require.NoError(t, err)
	require.NotNil(t, p)
	t.Cleanup(p.close)
	return p
}

func proxyTestClient(t *testing.T, p *networkProxy) *http.Client {
	t.Helper()
	address, err := url.Parse(p.address)
	require.NoError(t, err)
	transport := &http.Transport{Proxy: http.ProxyURL(address)}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport, Timeout: 5 * time.Second}
}

func TestBrowserProxyPinsDNSAnswers(t *testing.T) {
	for _, mixed := range []bool{false, true} {
		t.Run(fmt.Sprint(mixed), func(t *testing.T) {
			var lookups, dials int
			policy, err := newNetworkPolicy(runner.EngineOptions{BlockPrivateNetwork: true})
			require.NoError(t, err)
			var peer net.Conn
			p := &networkProxy{
				policy: policy, conns: make(map[net.Conn]struct{}),
				lookup: func(context.Context, string, string) ([]netip.Addr, error) {
					lookups++
					ips := []netip.Addr{netip.MustParseAddr("93.184.216.34")}
					if mixed || lookups > 1 {
						ips = append(ips, netip.MustParseAddr("127.0.0.1"))
					}
					return ips, nil
				},
				dialIP: func(_ context.Context, network, address string) (net.Conn, error) {
					dials++
					require.Equal(t, "tcp", network)
					require.Equal(t, "93.184.216.34:443", address)
					var conn net.Conn
					conn, peer = net.Pipe()
					return conn, nil
				},
			}
			conn, err := p.dial(context.Background(), "tcp", "rebind.example:443")
			if mixed {
				require.ErrorIs(t, err, errNetworkDenied)
				require.Zero(t, dials)
				return
			}
			require.NoError(t, err)
			require.Equal(t, 1, lookups)
			require.Equal(t, 1, dials)
			require.NoError(t, conn.Close())
			require.NoError(t, peer.Close())
		})
	}
}

func TestBrowserProxyAddressTimeoutFallback(t *testing.T) {
	for _, upstream := range []bool{false, true} {
		t.Run(fmt.Sprintf("upstream=%t", upstream), func(t *testing.T) {
			options := runner.EngineOptions{BlockPrivateNetwork: true}
			if upstream {
				options.ProxyURL = "http://proxy.example:8080"
			}
			p := testNetworkProxy(t, options)
			p.lookup = func(context.Context, string, string) ([]netip.Addr, error) {
				return []netip.Addr{netip.MustParseAddr("203.0.113.1"), netip.MustParseAddr("203.0.113.2")}, nil
			}
			var attempts []string
			p.dialIP = func(ctx context.Context, _, address string) (net.Conn, error) {
				attempts = append(attempts, address)
				if strings.HasPrefix(address, "203.0.113.1:") {
					<-ctx.Done()
					return nil, ctx.Err()
				}
				require.NoError(t, ctx.Err(), "the reachable fallback must receive a live context")
				conn, peer := net.Pipe()
				t.Cleanup(func() { _ = conn.Close(); _ = peer.Close() })
				if upstream {
					go func() {
						if _, err := http.ReadRequest(bufio.NewReader(peer)); err == nil {
							_, _ = io.WriteString(peer, "HTTP/1.1 200 Connection Established\r\n\r\nok")
						}
					}()
				}
				return conn, nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var conn net.Conn
			var err error
			if upstream {
				conn, err = p.dialUpstream(ctx, "93.184.216.34:443")
			} else {
				conn, err = p.dial(ctx, "tcp", "target.example:443")
			}
			require.NoError(t, err)
			require.NoError(t, ctx.Err())
			require.Len(t, attempts, 2)
			if upstream {
				data := make([]byte, 2)
				_, err = io.ReadFull(conn, data)
				require.NoError(t, err)
				require.Equal(t, "ok", string(data))
			}
			require.NoError(t, conn.Close())
		})
	}
}

func TestBrowserAddressFallbackCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	conn, err := dialBrowserAddresses(ctx, []string{"203.0.113.1:443", "203.0.113.2:443"},
		func(context.Context, string) (net.Conn, error) {
			t.Fatal("canceled dial must not start another address")
			return nil, nil
		})
	require.Nil(t, conn)
	require.ErrorIs(t, err, context.Canceled)
}

func TestBrowserProxyHTTPAndRedirect(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Empty(t, r.Header.Get("Proxy-Authorization"))
		if r.URL.Path == "/headers" {
			require.Equal(t, "192.0.2.4", r.Header.Get("X-Forwarded-For"))
			require.Equal(t, "task=probe", r.Header.Get("Cookie"))
			require.Empty(t, r.Header.Get("Accept-Encoding"))
		}
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://127.0.0.2:1/denied", http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "forwarded")
	}))
	defer target.Close()
	p := testNetworkProxy(t, runner.EngineOptions{BlockPrivateNetwork: true, PrivateNetworkCIDRs: []string{"127.0.0.2/32"}})
	client := proxyTestClient(t, p)
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, target.URL+"/headers", nil)
	require.NoError(t, err)
	request.Header.Set("X-Forwarded-For", "192.0.2.4")
	request.Header.Set("Cookie", "task=probe")
	request.Header.Set("Proxy-Authorization", "must-not-reach-origin")
	client.Transport.(*http.Transport).DisableCompression = true
	response, err := client.Do(request)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "forwarded", string(body))
	response, err = client.Get(target.URL + "/redirect")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.EqualValues(t, 2, hits.Load())

	// A separate task's policy cannot inherit the permissive task's transport.
	blocked := testNetworkProxy(t, runner.EngineOptions{BlockPrivateNetwork: true})
	response, err = proxyTestClient(t, blocked).Get(target.URL)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.EqualValues(t, 2, hits.Load())
}

func TestBrowserProxyHTTPS(t *testing.T) {
	var hits atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		require.Empty(t, r.Header.Get("Proxy-Authorization"))
		_, _ = io.WriteString(w, "TLS unchanged")
	}))
	defer target.Close()
	p := testNetworkProxy(t, runner.EngineOptions{PrivateNetworkCIDRs: []string{"203.0.113.0/24"}})
	client := proxyTestClient(t, p)
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig = target.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	transport.TLSClientConfig.RootCAs = x509.NewCertPool()
	transport.TLSClientConfig.RootCAs.AddCert(target.Certificate())
	response, err := client.Get(target.URL)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "TLS unchanged", string(body))
	blocked := testNetworkProxy(t, runner.EngineOptions{BlockPrivateNetwork: true})
	_, err = proxyTestClient(t, blocked).Get(target.URL)
	require.Error(t, err)
	require.EqualValues(t, 1, hits.Load())
}

func TestBrowserProxyUpstream(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Empty(t, r.Header.Get("Proxy-Authorization"))
		_, _ = io.WriteString(w, "upstream forwarded")
	}))
	defer target.Close()
	upstream := testNetworkProxy(t, runner.EngineOptions{PrivateNetworkCIDRs: []string{"203.0.113.0/24"}})
	var tunnels atomic.Int32
	handler := upstream.handler()
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodConnect, r.Method)
		require.Equal(t, strings.TrimPrefix(target.URL, "http://"), r.Host)
		require.Equal(t, "Basic dXNlcjpzZWNyZXQ=", r.Header.Get("Proxy-Authorization"))
		tunnels.Add(1)
		handler.ServeHTTP(w, r)
	}))
	front.Listener = &proxyListener{Listener: front.Listener, proxy: upstream}
	front.Start()
	defer front.Close()
	proxyURL, err := url.Parse(front.URL)
	require.NoError(t, err)
	proxyURL.User = url.UserPassword("user", "secret")
	p := testNetworkProxy(t, runner.EngineOptions{ProxyURL: proxyURL.String()})
	response, err := proxyTestClient(t, p).Get(target.URL)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "upstream forwarded", string(body))
	require.EqualValues(t, 1, tunnels.Load())

	// Even a task-supplied upstream must pass the destination policy.
	blocked := testNetworkProxy(t, runner.EngineOptions{BlockPrivateNetwork: true, ProxyURL: proxyURL.String()})
	_, err = blocked.dialUpstream(context.Background(), "93.184.216.34:443")
	require.ErrorIs(t, err, errNetworkDenied)
	require.EqualValues(t, 1, tunnels.Load())
}

func TestBrowserProxyTrustedNodeEndpoint(t *testing.T) {
	var originHits, tunnels atomic.Int32
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		originHits.Add(1)
		require.Empty(t, r.Header.Get("Proxy-Authorization"))
		_, _ = io.WriteString(w, "trusted upstream forwarded")
	}))
	defer origin.Close()
	_, port, err := net.SplitHostPort(origin.Listener.Addr().String())
	require.NoError(t, err)
	upstream := testNetworkProxy(t, runner.EngineOptions{PrivateNetworkCIDRs: []string{"203.0.113.0/24"}})
	handler := upstream.handler()
	front := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tunnels.Add(1)
		require.Equal(t, http.MethodConnect, r.Method)
		require.Equal(t, "93.184.216.34:"+port, r.Host, "the upstream receives the checked origin IP")
		// Only the test upstream maps the public test IP to the local fixture.
		r.Host = origin.Listener.Addr().String()
		handler.ServeHTTP(w, r)
	}))
	front.Listener = &proxyListener{Listener: front.Listener, proxy: upstream}
	front.Start()
	defer front.Close()
	trusted := testNetworkProxy(t, runner.EngineOptions{
		BlockPrivateNetwork: true, ProxyURL: front.URL, ProxyFromNode: true,
	})
	response, err := proxyTestClient(t, trusted).Get("http://93.184.216.34:" + port)
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, "trusted upstream forwarded", string(body))

	// Trust applies only to the configured upstream endpoint, never the origin.
	response, err = proxyTestClient(t, trusted).Get(origin.URL)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.EqualValues(t, 1, originHits.Load())
	require.EqualValues(t, 1, tunnels.Load())

	untrusted := testNetworkProxy(t, runner.EngineOptions{BlockPrivateNetwork: true, ProxyURL: front.URL})
	response, err = proxyTestClient(t, untrusted).Get("http://93.184.216.34:" + port)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusForbidden, response.StatusCode)
	require.EqualValues(t, 1, tunnels.Load(), "a task cannot gain node proxy trust")
}

func TestBrowserProxyUpgradeAndShutdown(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, buffered, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer conn.Close() //nolint:errcheck
		_, _ = buffered.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: probe\r\n\r\n")
		_ = buffered.Flush()
		_, _ = io.Copy(conn, buffered)
	}))
	defer target.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := startNetworkProxy(ctx, runner.EngineOptions{PrivateNetworkCIDRs: []string{"203.0.113.0/24"}})
	require.NoError(t, err)
	defer p.close()
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(p.address, "http://"), time.Second)
	require.NoError(t, err)
	defer conn.Close() //nolint:errcheck
	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = fmt.Fprintf(conn, "GET %s HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: probe\r\n\r\n",
		target.URL, strings.TrimPrefix(target.URL, "http://"))
	require.NoError(t, err)
	reader := bufio.NewReader(conn)
	response, err := http.ReadResponse(reader, &http.Request{Method: http.MethodGet})
	require.NoError(t, err)
	require.Equal(t, http.StatusSwitchingProtocols, response.StatusCode)
	_, err = io.WriteString(conn, "hello")
	require.NoError(t, err)
	data := make([]byte, 5)
	_, err = io.ReadFull(reader, data)
	require.NoError(t, err)
	require.Equal(t, "hello", string(data))
	cancel()
	_, err = reader.ReadByte()
	require.Error(t, err)
	p.close()
	p.mu.Lock()
	require.Empty(t, p.conns)
	p.mu.Unlock()
	_, err = net.DialTimeout("tcp", strings.TrimPrefix(p.address, "http://"), time.Second)
	require.Error(t, err)
}

func TestBrowserProxyUpstreamFailures(t *testing.T) {
	var originHits atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		originHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	for _, secure := range []bool{false, true} {
		t.Run(fmt.Sprint(secure), func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusProxyAuthRequired)
			})
			var upstream *httptest.Server
			if secure {
				upstream = httptest.NewTLSServer(handler)
			} else {
				upstream = httptest.NewServer(handler)
			}
			defer upstream.Close()
			p := testNetworkProxy(t, runner.EngineOptions{ProxyURL: upstream.URL, IgnoreHTTPSErrors: true})
			response, err := proxyTestClient(t, p).Get(target.URL)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, http.StatusBadGateway, response.StatusCode)
			require.Zero(t, originHits.Load(), "proxy failure or an untrusted TLS proxy must never fall back to direct")
		})
	}
}

func TestBrowserProxyURLValidation(t *testing.T) {
	for _, value := range []string{"direct://", "socks5://proxy:1080", "http://user:secret@proxy:99999", "http://proxy/path"} {
		_, err := browserUpstreamProxy(value)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret")
	}
	for _, value := range []string{"http://proxy:8080", "https://user:secret@proxy", "http://[::1]:8080"} {
		_, err := browserUpstreamProxy(value)
		require.NoError(t, err)
	}
	p, err := startNetworkProxy(context.Background(), runner.EngineOptions{})
	require.NoError(t, err)
	require.Nil(t, p, "unrestricted direct tasks need no proxy")
}
