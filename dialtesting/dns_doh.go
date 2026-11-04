// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

func (c *dnsClient) setupDoH(runCtx context.Context) error {
	t := c.task
	cfg, err := t.dnsTLSConfig()
	if err != nil {
		return err
	}
	// A transport may detach its internal dial context from the request. Always
	// bind connection establishment to this run, including TLS and QUIC.
	ctx, cancel := context.WithCancel(runCtx)
	c.cancelHTTP = cancel
	var rt http.RoundTripper
	address := net.JoinHostPort(t.result.IP, t.Port)
	if t.DoH.HTTPProtocol == "http/3" {
		transport := &http3.Transport{TLSClientConfig: cfg, Dial: func(_ context.Context, _ string, tc *tls.Config, qc *quic.Config) (*quic.Conn, error) {
			start := time.Now()
			conn, err := quic.DialAddr(ctx, address, tc, qc)
			if err == nil {
				c.recordTLS(conn.ConnectionState().TLS, time.Since(start))
			}
			return conn, err
		}}
		rt = transport
		c.closeHTTP = func() { _ = transport.Close() }
	} else {
		transport := &http.Transport{TLSClientConfig: cfg, ForceAttemptHTTP2: true, Proxy: nil, DialTLSContext: func(_ context.Context, network, _ string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
			if err != nil {
				return nil, err
			}
			secure := tls.Client(conn, cfg)
			start := time.Now()
			if err = secure.HandshakeContext(ctx); err != nil {
				_ = conn.Close()
				return nil, err
			}
			c.recordTLS(secure.ConnectionState(), time.Since(start))
			return secure, nil
		}}
		if t.DoH.HTTPProtocol == "http/2-only" {
			protocols := new(http.Protocols)
			protocols.SetHTTP2(true)
			transport.Protocols = protocols
			cfg.NextProtos = []string{"h2"}
		}
		if t.DoH.HTTPProtocol == "http/1.1" {
			transport.ForceAttemptHTTP2 = false
			cfg.NextProtos = []string{"http/1.1"}
			transport.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		}
		rt = transport
		c.closeHTTP = transport.CloseIdleConnections
	}
	c.http = &http.Client{Transport: rt, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return nil
}

func (c *dnsClient) exchangeDoH(ctx context.Context, packet []byte) ([]byte, error) {
	t := c.task
	endpoint, err := url.Parse(t.DoH.URL)
	if err != nil {
		return nil, errors.New("invalid DoH endpoint")
	}
	var body io.Reader
	if t.DoH.Method == "GET" {
		q := endpoint.Query()
		q.Set("dns", base64.RawURLEncoding.EncodeToString(packet))
		endpoint.RawQuery = q.Encode()
	} else {
		body = bytes.NewReader(packet)
	}
	req, err := http.NewRequestWithContext(ctx, t.DoH.Method, endpoint.String(), body)
	if err != nil {
		return nil, errors.New("invalid DoH request")
	}
	for k, v := range t.DoH.Headers {
		req.Header.Set(k, v)
	}
	req.Header.Set("Accept", "application/dns-message")
	if t.DoH.Method == "POST" {
		req.Header.Set("Content-Type", "application/dns-message")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck
	t.result.HTTPProtocol, t.result.HTTPStatus = resp.Proto, resp.StatusCode
	if t.DoH.HTTPProtocol == "http/2-only" && resp.ProtoMajor != 2 {
		return nil, errors.New("DoH endpoint did not negotiate HTTP/2")
	}
	if t.DoH.HTTPProtocol == "http/3" && resp.ProtoMajor != 3 {
		return nil, errors.New("DoH endpoint did not negotiate HTTP/3")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("DoH HTTP response was not successful")
	}
	media, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil || media != "application/dns-message" {
		return nil, errors.New("DoH response has invalid Content-Type")
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 65536))
	if err != nil {
		return nil, err
	}
	if len(b) < 12 || len(b) > 65535 {
		return nil, errors.New("DoH response has invalid DNS message length")
	}
	return b, nil
}
