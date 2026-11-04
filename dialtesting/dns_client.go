// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"golang.org/x/net/dns/dnsmessage"
)

type dnsResponseError struct{ error }

type dnsClient struct {
	task       *DNSTask
	conn       net.Conn
	quic       *quic.Conn
	http       *http.Client
	closeHTTP  func()
	cancelHTTP context.CancelFunc
	tlsMu      sync.Mutex
	tlsState   *tls.ConnectionState
	handshake  time.Duration
}

func (t *DNSTask) tlsServerName() string {
	if t.ServerName != "" {
		return t.ServerName
	}
	return t.host
}

func (t *DNSTask) dnsTLSConfig() (*tls.Config, error) {
	c := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: t.tlsServerName()}
	if t.Certificate == nil {
		return c, nil
	}
	c.InsecureSkipVerify = t.Certificate.IgnoreServerCertificateError //nolint:gosec // Explicit diagnostic option, reported in results.
	if t.Certificate.CaCert != "" {
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM([]byte(t.Certificate.CaCert)) {
			return nil, errors.New("invalid DNS CA certificate")
		}
		c.RootCAs = pool
	}
	if t.Certificate.Certificate != "" {
		pair, err := tls.X509KeyPair([]byte(t.Certificate.Certificate), []byte(t.Certificate.PrivateKey))
		if err != nil {
			return nil, errors.New("invalid DNS client certificate or key")
		}
		c.Certificates = []tls.Certificate{pair}
	}
	return c, nil
}

func (c *dnsClient) recordTLS(s tls.ConnectionState, d time.Duration) {
	c.tlsMu.Lock()
	defer c.tlsMu.Unlock()
	c.tlsState = &s
	c.handshake += d
}

func (c *dnsClient) publishTLS() {
	c.tlsMu.Lock()
	defer c.tlsMu.Unlock()
	if c.tlsState == nil {
		return
	}
	t, s := c.task, c.tlsState
	t.result.Handshake = c.handshake
	t.result.TLSVersion = tlsVersionString(s.Version)
	verified := len(s.VerifiedChains) > 0
	t.result.Verified = &verified
	if len(s.PeerCertificates) > 0 {
		t.result.CertNotAfter = s.PeerCertificates[0].NotAfter.UnixMicro()
	}
}

func (t *DNSTask) newDNSClient(ctx context.Context) (*dnsClient, error) {
	c := &dnsClient{task: t}
	if t.Protocol == "doh" {
		if err := c.setupDoH(ctx); err != nil {
			return nil, err
		}
	}
	if t.Protocol == "doq" {
		cfg, err := t.dnsTLSConfig()
		if err != nil {
			return nil, err
		}
		cfg.MinVersion = tls.VersionTLS13
		cfg.NextProtos = []string{"doq"}
		start := time.Now()
		c.quic, err = quic.DialAddr(ctx, net.JoinHostPort(t.result.IP, t.Port), cfg, &quic.Config{MaxIncomingStreams: -1, MaxIncomingUniStreams: -1})
		if err != nil {
			return nil, err
		}
		c.recordTLS(c.quic.ConnectionState().TLS, time.Since(start))
		if c.quic.ConnectionState().TLS.NegotiatedProtocol != "doq" {
			c.close()
			return nil, errors.New("DoQ ALPN negotiation failed")
		}
	}
	return c, nil
}

func (c *dnsClient) close() {
	if c.cancelHTTP != nil {
		c.cancelHTTP()
	}
	defer c.publishTLS()
	if c.conn != nil {
		_ = c.conn.Close()
	}
	if c.quic != nil {
		_ = c.quic.CloseWithError(0, "")
	}
	if c.closeHTTP != nil {
		c.closeHTTP()
	}
}

func (c *dnsClient) streamConn(ctx context.Context) (net.Conn, error) {
	if c.conn != nil {
		return c.conn, nil
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(c.task.result.IP, c.task.Port))
	if err != nil {
		return nil, err
	}
	if c.task.Protocol == "dot" {
		cfg, e := c.task.dnsTLSConfig()
		if e != nil {
			_ = conn.Close()
			return nil, e
		}
		tlsConn := tls.Client(conn, cfg)
		start := time.Now()
		if e = tlsConn.HandshakeContext(ctx); e != nil {
			_ = conn.Close()
			return nil, e
		}
		c.recordTLS(tlsConn.ConnectionState(), time.Since(start))
		conn = tlsConn
	}
	c.conn = conn
	return conn, nil
}

func dnsWriteFrame(w io.Writer, p []byte) error {
	if len(p) > 65535 {
		return errors.New("DNS message exceeds wire length")
	}
	b := make([]byte, len(p)+2)
	binary.BigEndian.PutUint16(b, uint16(len(p)))
	copy(b[2:], p)
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func dnsReadFrame(r io.Reader) ([]byte, error) {
	var size [2]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return nil, err
	}
	n := int(binary.BigEndian.Uint16(size[:]))
	if n < 12 {
		return nil, dnsResponseError{errors.New("invalid DNS frame length")}
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return b, err
}

func (c *dnsClient) exchange(ctx context.Context, p []byte, transport string) ([]byte, error) {
	switch transport {
	case "doh":
		return c.exchangeDoH(ctx, p)
	case "doq":
		stream, err := c.quic.OpenStreamSync(ctx)
		if err != nil {
			return nil, err
		}
		stop := context.AfterFunc(ctx, func() { stream.CancelRead(0); stream.CancelWrite(0) })
		defer stop()
		defer stream.CancelRead(0)
		if deadline, ok := ctx.Deadline(); ok {
			_ = stream.SetDeadline(deadline)
		}
		if err = dnsWriteFrame(stream, p); err != nil {
			stream.CancelWrite(0)
			return nil, err
		}
		if err = stream.Close(); err != nil {
			return nil, err
		}
		b, err := dnsReadFrame(stream)
		if err != nil {
			return nil, err
		}
		var extra [1]byte
		n, err := stream.Read(extra[:])
		if err != nil && err != io.EOF {
			return nil, err
		}
		if n != 0 || err != io.EOF {
			return nil, dnsResponseError{errors.New("DoQ response must end after one DNS message")}
		}
		return b, nil
	case "udp":
		conn, err := (&net.Dialer{}).DialContext(ctx, "udp", net.JoinHostPort(c.task.result.IP, c.task.Port))
		if err != nil {
			return nil, err
		}
		defer conn.Close() //nolint:errcheck
		stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
		defer stop()
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}
		if _, err = conn.Write(p); err != nil {
			return nil, err
		}
		b := make([]byte, 65535)
		n, err := conn.Read(b)
		return b[:n], err
	default:
		conn, err := c.streamConn(ctx)
		if err != nil {
			return nil, err
		}
		stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
		defer stop()
		if deadline, ok := ctx.Deadline(); ok {
			_ = conn.SetDeadline(deadline)
		}
		if err = dnsWriteFrame(conn, p); err != nil {
			return nil, err
		}
		return dnsReadFrame(conn)
	}
}

func dnsQType(typ string) dnsmessage.Type {
	return map[string]dnsmessage.Type{"A": dnsmessage.TypeA, "AAAA": dnsmessage.TypeAAAA, "CNAME": dnsmessage.TypeCNAME, "MX": dnsmessage.TypeMX, "NS": dnsmessage.TypeNS, "TXT": dnsmessage.TypeTXT}[typ]
}

func (t *DNSTask) dnsRequest(typ string) (*dnsmessage.Message, []byte, error) {
	name, err := dnsName(t.Domain)
	if err != nil {
		return nil, nil, err
	}
	n, err := dnsmessage.NewName(name + ".")
	if err != nil {
		return nil, nil, err
	}
	var id [2]byte
	if _, err = rand.Read(id[:]); err != nil {
		return nil, nil, err
	}
	rd := true
	if t.DNSOptions.RecursionDesired != nil {
		rd = *t.DNSOptions.RecursionDesired
	}
	m := &dnsmessage.Message{Header: dnsmessage.Header{ID: binary.BigEndian.Uint16(id[:]), RecursionDesired: rd, CheckingDisabled: t.DNSOptions.CheckingDisabled}, Questions: []dnsmessage.Question{{Name: n, Type: dnsQType(typ), Class: dnsmessage.ClassINET}}}
	if t.Protocol == "doh" || t.Protocol == "doq" {
		m.ID = 0
	}
	for _, s := range t.SuccessWhen {
		if s.AuthenticatedData != nil {
			m.AuthenticData = true
		}
	}
	root, _ := dnsmessage.NewName(".")
	h := dnsmessage.ResourceHeader{Name: root, Type: dnsmessage.TypeOPT}
	if err = h.SetEDNS0(t.DNSOptions.UDPPayloadSize, 0, t.DNSOptions.DNSSECOK); err != nil {
		return nil, nil, err
	}
	m.Additionals = []dnsmessage.Resource{{Header: h, Body: &dnsmessage.UnknownResource{Type: dnsmessage.TypeOPT}}}
	b, err := m.Pack()
	return m, b, err
}

func dnsResponseHeader(b []byte, request *dnsmessage.Message) (dnsmessage.Header, error) {
	var p dnsmessage.Parser
	h, err := p.Start(b)
	if err != nil {
		return h, err
	}
	questions, err := p.AllQuestions()
	if err != nil || !h.Response || h.OpCode != 0 || h.ID != request.ID || len(questions) != 1 {
		return h, errors.New("DNS response does not match request")
	}
	a, z := questions[0], request.Questions[0]
	if a.Type != z.Type || a.Class != z.Class || !strings.EqualFold(a.Name.String(), z.Name.String()) {
		return h, errors.New("DNS response question mismatch")
	}
	return h, nil
}

func dnsRCode(code dnsmessage.RCode) string {
	if name, ok := map[dnsmessage.RCode]string{0: "NOERROR", 1: "FORMERR", 2: "SERVFAIL", 3: "NXDOMAIN", 4: "NOTIMP", 5: "REFUSED"}[code]; ok {
		return name
	}
	return strconv.Itoa(int(code))
}

func (t *DNSTask) exchangeDNS(ctx context.Context, c *dnsClient, typ string) (q dnsQueryResult, err error) {
	q = dnsQueryResult{Type: typ, Server: t.host, Port: t.Port, Status: "FAIL", Records: []dnsRecord{}}
	t.result.QueryStarted = true
	start := time.Now()
	defer func() {
		cost := time.Since(start).Microseconds()
		q.Cost = &cost
		t.result.QueryCost += time.Since(start)
		if err != nil {
			q.Error = dnsErrorMessage(err)
			if q.Code == "" {
				q.Code = "network_error"
			}
			var wireErr dnsResponseError
			if errors.As(err, &wireErr) {
				q.Code = "invalid_response"
			}
			var networkErr net.Error
			if errors.As(err, &networkErr) && networkErr.Timeout() {
				q.Code = "timeout"
			}
			if ctx.Err() != nil {
				if errors.Is(ctx.Err(), context.Canceled) {
					q.Code = "canceled"
				} else {
					q.Code = "timeout"
				}
			}
		}
	}()
	request, packet, err := t.dnsRequest(typ)
	if err != nil {
		q.Code = "invalid_response"
		return q, err
	}
	q.Name = request.Questions[0].Name.String()
	q.RequestFlags = map[string]bool{"RD": request.RecursionDesired, "CD": request.CheckingDisabled, "AD": request.AuthenticData, "DO": t.DNSOptions.DNSSECOK}
	transport := t.Protocol
	if transport == "auto" {
		transport = "udp"
	}
	var response []byte
	var h dnsmessage.Header
	for range 2 {
		attemptStart := time.Now()
		response, err = c.exchange(ctx, packet, transport)
		if err != nil {
			q.Attempts = append(q.Attempts, dnsAttempt{Transport: transport, Cost: time.Since(attemptStart).Microseconds()})
			if transport == "dot" {
				q.Code = "tls_error"
			} else if transport == "doh" {
				q.Code = "http_error"
			} else if transport == "doq" {
				q.Code = "quic_error"
			}
			return q, err
		}
		h, err = dnsResponseHeader(response, request)
		q.Attempts = append(q.Attempts, dnsAttempt{Transport: transport, Cost: time.Since(attemptStart).Microseconds(), Truncated: h.Truncated})
		if err != nil {
			q.Code = "invalid_response"
			return q, err
		}
		if h.Truncated && t.Protocol == "auto" && transport == "udp" {
			transport = "tcp"
			continue
		}
		break
	}
	q.Flags = map[string]bool{"AA": h.Authoritative, "TC": h.Truncated, "RD": h.RecursionDesired, "RA": h.RecursionAvailable, "AD": h.AuthenticData, "CD": h.CheckingDisabled}
	if h.Truncated {
		q.Code = "truncated_response"
		return q, errors.New("DNS response is truncated")
	}
	var message dnsmessage.Message
	if err = message.Unpack(response); err != nil {
		q.Code = "invalid_response"
		return q, errors.New("invalid DNS response body")
	}
	code := h.RCode
	for _, r := range message.Additionals {
		if r.Header.Type == dnsmessage.TypeOPT {
			code = r.Header.ExtendedRCode(code)
		}
	}
	q.RCode = dnsRCode(code)
	q.Records = relatedDNSRecords(message.Answers, q.Name)
	q.Count = len(q.Records)
	for _, r := range q.Records {
		if r.Type == typ {
			q.TargetCount++
		}
	}
	if code != 0 {
		q.Code = "dns_rcode_error"
		return q, fmt.Errorf("DNS %s query returned %s", typ, q.RCode)
	}
	q.Status = "OK"
	return q, nil
}

func relatedDNSRecords(answer []dnsmessage.Resource, name string) []dnsRecord {
	related := map[string]bool{strings.ToLower(name): true}
	for range 32 {
		changed := false
		for _, r := range answer {
			if c, ok := r.Body.(*dnsmessage.CNAMEResource); ok && r.Header.Class == dnsmessage.ClassINET && related[strings.ToLower(r.Header.Name.String())] && !related[strings.ToLower(c.CNAME.String())] {
				related[strings.ToLower(c.CNAME.String())] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	result := []dnsRecord{}
	for _, r := range answer {
		if !related[strings.ToLower(r.Header.Name.String())] || r.Header.Class != dnsmessage.ClassINET {
			continue
		}
		rec := dnsRecord{Name: r.Header.Name.String(), Class: "IN", TTL: r.Header.TTL}
		switch b := r.Body.(type) {
		case *dnsmessage.AResource:
			rec.Type, rec.Value = "A", net.IP(b.A[:]).String()
		case *dnsmessage.AAAAResource:
			rec.Type, rec.Value = "AAAA", net.IP(b.AAAA[:]).String()
		case *dnsmessage.CNAMEResource:
			rec.Type, rec.Value = "CNAME", b.CNAME.String()
		case *dnsmessage.NSResource:
			rec.Type, rec.Value = "NS", b.NS.String()
		case *dnsmessage.MXResource:
			rec.Type, rec.Value = "MX", fmt.Sprintf("%d %s", b.Pref, b.MX.String())
		case *dnsmessage.TXTResource:
			rec.Type, rec.Value = "TXT", strings.Join(b.TXT, "")
		default:
			continue
		}
		result = append(result, rec)
	}
	return result
}
