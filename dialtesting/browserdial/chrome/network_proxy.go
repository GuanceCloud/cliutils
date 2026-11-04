// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package chrome

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
)

var errNetworkDenied = errors.New("browser destination blocked by network policy")

type networkPolicy struct {
	deny []netip.Prefix
}

func newNetworkPolicy(options runner.EngineOptions) (networkPolicy, error) {
	policy := networkPolicy{}
	for _, value := range options.PrivateNetworkCIDRs {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return policy, fmt.Errorf("invalid browser network CIDR %q: %w", value, err)
		}
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		policy.deny = append(policy.deny, prefix.Masked())
	}
	// Match Lightpanda's configuration semantics: explicit CIDRs replace the
	// default private ranges rather than adding an implicit private deny list.
	if len(policy.deny) == 0 && options.BlockPrivateNetwork {
		for _, value := range []string{
			"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8", "169.254.0.0/16",
			"172.16.0.0/12", "192.168.0.0/16", "198.18.0.0/15", "224.0.0.0/4", "240.0.0.0/4",
			"::/128", "::1/128", "fc00::/7", "fe80::/10", "ff00::/8",
		} {
			policy.deny = append(policy.deny, netip.MustParsePrefix(value))
		}
	}
	return policy, nil
}

func (p networkPolicy) allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.Zone() != "" || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	for _, prefix := range p.deny {
		if prefix.Contains(ip) {
			return false
		}
	}
	return true
}

func browserUpstreamProxy(value string) (*url.URL, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		// Do not include a potentially credential-bearing URL in errors.
		return nil, fmt.Errorf("Chrome proxy_url must be an HTTP or HTTPS proxy URL with a host")
	}
	if port := u.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return nil, fmt.Errorf("invalid Chrome proxy port")
		}
	}
	return u, nil
}

// networkProxy is task-local. It never decrypts origin TLS. Browser flags route
// URL requests here; deployment egress rules remain the boundary for other traffic.
type networkProxy struct {
	policy        networkPolicy
	upstream      *url.URL
	proxyFromNode bool
	lookup        func(context.Context, string, string) ([]netip.Addr, error)
	dialIP        func(context.Context, string, string) (net.Conn, error)
	server        *http.Server
	listener      net.Listener
	transport     *http.Transport
	address       string
	cancel        context.CancelFunc
	closeOnce     sync.Once
	mu            sync.Mutex
	conns         map[net.Conn]struct{}
	closed        bool
}

func startNetworkProxy(ctx context.Context, options runner.EngineOptions) (*networkProxy, error) {
	policy, err := newNetworkPolicy(options)
	if err != nil {
		return nil, err
	}
	upstream, err := browserUpstreamProxy(options.ProxyURL)
	if err != nil {
		return nil, err
	}
	if len(policy.deny) == 0 && upstream == nil {
		return nil, nil
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("start browser network proxy: %w", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	p := &networkProxy{
		policy: policy, upstream: upstream, lookup: net.DefaultResolver.LookupNetIP,
		proxyFromNode: options.ProxyFromNode,
		dialIP:        dialer.DialContext, cancel: cancel, conns: make(map[net.Conn]struct{}),
		address:  "http://" + listener.Addr().String(),
		listener: listener,
	}
	p.transport = &http.Transport{
		DialContext: p.dial, MaxIdleConns: 16, MaxIdleConnsPerHost: 4,
		IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression: true,
	}
	p.server = &http.Server{
		Handler: p.handler(), ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout: 30 * time.Second, MaxHeaderBytes: 64 << 10,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		if err := p.server.Serve(&proxyListener{Listener: listener, proxy: p}); err != nil {
			p.close()
		}
	}()
	go func() { <-ctx.Done(); p.close() }()
	return p, nil
}

func (p *networkProxy) destinations(ctx context.Context, address string, policy networkPolicy) ([]string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid browser destination")
	}
	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return nil, fmt.Errorf("invalid browser destination port")
	}
	ips, err := p.lookup(ctx, "ip", host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("browser destination has no addresses")
	}
	addresses := make([]string, 0, len(ips))
	for _, ip := range ips {
		// Check every answer before dialing any, including mixed IPv4/IPv6
		// results. Dial these exact IPs so a second lookup cannot rebind them.
		if !policy.allowed(ip) {
			return nil, errNetworkDenied
		}
		addresses = append(addresses, net.JoinHostPort(ip.Unmap().String(), port))
	}
	return addresses, nil
}

func (p *networkProxy) dial(ctx context.Context, _, address string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	addresses, err := p.destinations(ctx, address, p.policy)
	if err != nil {
		return nil, err
	}
	conn, err := dialBrowserAddresses(ctx, addresses, func(attempt context.Context, destination string) (net.Conn, error) {
		if p.upstream != nil {
			return p.dialUpstream(attempt, destination)
		}
		return p.dialIP(attempt, "tcp", destination)
	})
	if err != nil {
		return nil, err
	}
	return p.track(conn)
}

func dialBrowserAddresses(ctx context.Context, addresses []string, dial func(context.Context, string) (net.Conn, error)) (net.Conn, error) {
	if len(addresses) == 0 {
		return nil, fmt.Errorf("browser destination has no addresses")
	}
	var lastErr error
	for i, address := range addresses {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		attempt := ctx
		cancel := func() {}
		// Reserve time for the remaining checked IPs without another DNS lookup.
		if deadline, ok := ctx.Deadline(); ok && len(addresses)-i > 1 {
			attempt, cancel = context.WithTimeout(ctx, time.Until(deadline)/time.Duration(len(addresses)-i))
		}
		conn, err := dial(attempt, address)
		cancel()
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (p *networkProxy) dialUpstream(ctx context.Context, destination string) (net.Conn, error) {
	port := p.upstream.Port()
	if port == "" {
		port = "80"
		if p.upstream.Scheme == "https" {
			port = "443"
		}
	}
	// Only the administrator's node proxy may use an internal endpoint. Task
	// overrides retain the destination policy, and origin checks are unchanged.
	policy := p.policy
	if p.proxyFromNode {
		policy = networkPolicy{}
	}
	addresses, err := p.destinations(ctx, net.JoinHostPort(p.upstream.Hostname(), port), policy)
	if err != nil {
		return nil, err
	}
	conn, err := dialBrowserAddresses(ctx, addresses, func(attempt context.Context, address string) (net.Conn, error) {
		return p.dialIP(attempt, "tcp", address)
	})
	if err != nil {
		return nil, err
	}
	raw := conn
	stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
	defer stop()
	if err = conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if p.upstream.Scheme == "https" {
		secure := tls.Client(conn, &tls.Config{ServerName: p.upstream.Hostname(), MinVersion: tls.VersionTLS12})
		if err = secure.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		conn = secure
	}
	conn, err = connectUpstream(conn, destination, p.upstream.User)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func connectUpstream(conn net.Conn, destination string, credentials *url.Userinfo) (net.Conn, error) {
	request := &http.Request{
		Method: http.MethodConnect, URL: &url.URL{Host: destination}, Host: destination,
		Header: make(http.Header),
	}
	if credentials != nil {
		password, _ := credentials.Password()
		encoded := base64.StdEncoding.EncodeToString([]byte(credentials.Username() + ":" + password))
		request.Header.Set("Proxy-Authorization", "Basic "+encoded)
	}
	if err := request.Write(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	reader := bufio.NewReader(io.LimitReader(conn, 64<<10))
	response, err := http.ReadResponse(reader, request)
	if err != nil || response.StatusCode != http.StatusOK {
		_ = conn.Close()
		return nil, fmt.Errorf("upstream browser proxy CONNECT failed")
	}
	if err = conn.SetDeadline(time.Time{}); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &proxyBufferedConn{Conn: conn, reader: reader}, nil
}

type proxyBufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *proxyBufferedConn) Read(data []byte) (int, error) {
	if c.reader.Buffered() > 0 {
		return c.reader.Read(data)
	}
	return c.Conn.Read(data)
}

func (p *networkProxy) handler() http.Handler {
	forward := &httputil.ReverseProxy{
		Rewrite: func(request *httputil.ProxyRequest) {
			request.Out.URL = request.In.URL
			request.Out.Host = request.In.URL.Host
			// ReverseProxy strips these by default. A forward proxy must retain
			// explicitly configured task headers without adding its own values.
			for _, header := range []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"} {
				if values, ok := request.In.Header[header]; ok && !proxyHopHeader(request.In.Header, header) {
					request.Out.Header[header] = append([]string(nil), values...)
				}
			}
			request.Out.Header.Del("Proxy-Authorization")
			request.Out.Header.Del("Proxy-Connection")
		},
		Transport:    p.transport,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) { proxyError(w, err) },
	}
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodConnect {
			p.connect(w, request)
			return
		}
		if request.URL.Scheme != "http" || request.URL.Host == "" || request.URL.User != nil {
			http.Error(w, "absolute HTTP URL required", http.StatusBadRequest)
			return
		}
		forward.ServeHTTP(w, request)
	})
}

func proxyHopHeader(headers http.Header, name string) bool {
	for _, value := range headers.Values("Connection") {
		for token := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), name) {
				return true
			}
		}
	}
	return false
}

func proxyError(w http.ResponseWriter, err error) {
	if errors.Is(err, errNetworkDenied) {
		http.Error(w, errNetworkDenied.Error(), http.StatusForbidden)
		return
	}
	http.Error(w, "browser proxy connection failed", http.StatusBadGateway)
}

func (p *networkProxy) connect(w http.ResponseWriter, request *http.Request) {
	upstream, err := p.dial(request.Context(), "tcp", request.Host)
	if err != nil {
		proxyError(w, err)
		return
	}
	defer upstream.Close() //nolint:errcheck
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "browser proxy tunnel unavailable", http.StatusInternalServerError)
		return
	}
	client, buffered, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer client.Close() //nolint:errcheck
	if _, err = buffered.WriteString("HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if err = buffered.Flush(); err != nil {
		return
	}
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstream, buffered); done <- struct{}{} }()
	go func() { _, _ = io.Copy(client, upstream); done <- struct{}{} }()
	<-done
}

type proxyConn struct {
	net.Conn
	proxy *networkProxy
}

func (c *proxyConn) Close() error {
	err := c.Conn.Close()
	c.proxy.mu.Lock()
	delete(c.proxy.conns, c)
	c.proxy.mu.Unlock()
	return err
}

func (p *networkProxy) track(conn net.Conn) (net.Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		_ = conn.Close()
		return nil, net.ErrClosed
	}
	tracked := &proxyConn{Conn: conn, proxy: p}
	p.conns[tracked] = struct{}{}
	return tracked, nil
}

type proxyListener struct {
	net.Listener
	proxy *networkProxy
}

func (l *proxyListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return l.proxy.track(conn)
}

func (p *networkProxy) close() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		p.cancel()
		_ = p.listener.Close()
		_ = p.server.Close()
		p.transport.CloseIdleConnections()
		p.mu.Lock()
		p.closed = true
		connections := make([]net.Conn, 0, len(p.conns))
		for conn := range p.conns {
			connections = append(connections, conn)
		}
		p.mu.Unlock()
		for _, conn := range connections {
			_ = conn.Close()
		}
	})
}
