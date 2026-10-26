// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSRejectsMismatchedReplies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*dnsmessage.Message)
	}{
		{"id", func(m *dnsmessage.Message) { m.ID++ }},
		{"question", func(m *dnsmessage.Message) { m.Questions[0].Type = dnsmessage.TypeAAAA }},
		{"qr", func(m *dnsmessage.Message) { m.Response = false }},
		{"opcode", func(m *dnsmessage.Message) { m.OpCode = 1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			port := dnsFixturePlain(t, func(b []byte) []byte {
				var m dnsmessage.Message
				_ = m.Unpack(dnsFixtureReply(b))
				tc.mutate(&m)
				out, _ := m.Pack()
				return out
			}, false)
			f := dnsAssertRun(t, dnsFixtureTask(t, "udp", port), false)
			require.Equal(t, "invalid_response", f["error_code"])
		})
	}
}

func TestDNSRelatedAnswersAndAliases(t *testing.T) {
	port := dnsFixturePlain(t, func(b []byte) []byte {
		var m dnsmessage.Message
		_ = m.Unpack(dnsFixtureReply(b))
		alias, _ := dnsmessage.NewName("edge.example.test.")
		unrelated, _ := dnsmessage.NewName("unrelated.test.")
		m.Answers[0].Header.Name = alias
		m.Answers = append(m.Answers,
			dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: m.Questions[0].Name, Type: dnsmessage.TypeCNAME, Class: dnsmessage.ClassINET, TTL: 50}, Body: &dnsmessage.CNAMEResource{CNAME: alias}},
			dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: unrelated, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 50}, Body: &dnsmessage.AResource{A: [4]byte{1, 2, 3, 4}}},
		)
		out, _ := m.Pack()
		return out
	}, false)
	task := dnsFixtureTask(t, "udp", port)
	task.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{{SuccessOption: SuccessOption{Is: "203.0.113.10"}}}}}}
	f := dnsAssertRun(t, task, true)
	require.EqualValues(t, 2, f["answer_count"])
	require.NotContains(t, f["dns_queries"], "unrelated.test")
	require.True(t, dnsEqual("MX", "10 mail.EXAMPLE.test.", "010 mail.example.test"))
}

func TestDNSAutoDoesNotRetryTimeoutOverTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	var calls atomic.Int32
	go func() {
		c, e := ln.Accept()
		if e == nil {
			calls.Add(1)
			_ = c.Close()
		}
	}()
	udp, err := net.ListenPacket("udp", ln.Addr().String())
	require.NoError(t, err)
	defer udp.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	task := dnsFixtureTask(t, "auto", port)
	task.Timeout = "20ms"
	f := dnsAssertRun(t, task, false)
	require.Equal(t, "timeout", f["error_code"])
	require.Zero(t, calls.Load())
	require.NotContains(t, f["dns_queries"], `"transport":"tcp"`)
}

func TestDNSIPv6Resolver(t *testing.T) {
	udp, err := net.ListenPacket("udp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback unavailable: %v", err)
	}
	defer udp.Close()
	go func() {
		b := make([]byte, 4096)
		n, p, e := udp.ReadFrom(b)
		if e == nil {
			_, _ = udp.WriteTo(dnsFixtureReply(b[:n]), p)
		}
	}()
	_, port, _ := net.SplitHostPort(udp.LocalAddr().String())
	task := dnsFixtureTask(t, "udp", port)
	task.DNSServer = "::1"
	dnsAssertRun(t, task, true)
	require.Equal(t, "::1", task.result.IP)
}

func TestDNSMutualTLSAndInsecureReporting(t *testing.T) {
	server, ca := dnsFixtureCert(t)
	key := server.PrivateKey
	parsed, err := x509.ParseCertificate(server.Certificate[0])
	require.NoError(t, err)
	clientCert := *parsed
	clientCert.SerialNumber = big.NewInt(2)
	clientCert.IsCA = false
	clientCert.Subject = pkix.Name{CommonName: "DNS test client"}
	clientCert.RawSubject = nil
	clientCert.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	der, err := x509.CreateCertificate(rand.Reader, &clientCert, parsed, parsed.PublicKey, key)
	require.NoError(t, err)
	clientPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM([]byte(ca)))
	clientParsed, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	_, err = clientParsed.Verify(x509.VerifyOptions{Roots: pool, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}})
	require.NoError(t, err)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{server}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pool})
	require.NoError(t, err)
	dnsFixtureTCP(t, ln, dnsFixtureReply)
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	task := dnsFixtureTask(t, "dot", port)
	task.Certificate = &HTTPOptCertificate{CaCert: ca, Certificate: string(clientPEM), PrivateKey: string(keyPEM)}
	f := dnsAssertRun(t, task, true)
	require.Equal(t, true, f["certificate_verified"])
	task.Certificate.IgnoreServerCertificateError = true
	f = dnsAssertRun(t, task, true)
	require.Equal(t, false, f["certificate_verified"])
	raw, err := task.getRawTask(task.String())
	require.NoError(t, err)
	require.NotContains(t, raw, string(keyPEM))
}

func TestDNSDoHFailureDoesNotExposeURLCredentials(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	task := dnsFixtureTask(t, "doh", u.Port())
	task.DoH = &DNSDoHOption{URL: srv.URL + "/dns-query?token=query-secret", Headers: map[string]string{"Authorization": "Bearer header-secret"}}
	f := dnsAssertRun(t, task, false)
	all, err := json.Marshal(f)
	require.NoError(t, err)
	require.NotContains(t, string(all), "query-secret")
	require.NotContains(t, string(all), "header-secret")
	require.Equal(t, "[redacted] [redacted]", task.safeDNSString("query-secret Bearer header-secret"))
	require.NotContains(t, dnsErrorMessage(&url.Error{Op: "Post", URL: "https://x/?token=secret", Err: net.ErrClosed}), "secret")
}

func TestDNSDoHHandshakeStopsAtRunDeadline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	closed := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			closed <- err
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		b := make([]byte, 4096)
		for {
			if _, err = conn.Read(b); err != nil {
				closed <- err
				return
			}
		}
	}()
	_, port, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)
	task := dnsFixtureTask(t, "doh", port)
	task.Timeout = "100ms"
	task.DoH = &DNSDoHOption{URL: "https://" + ln.Addr().String() + "/dns-query"}
	f := dnsAssertRun(t, task, false)
	require.Equal(t, "timeout", f["error_code"])
	require.NotContains(t, f, "certificate_verified")
	select {
	case err := <-closed:
		var ne net.Error
		if errors.As(err, &ne) {
			require.False(t, ne.Timeout(), "TLS connection remained open after the run")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("DoH TLS connection was not closed after its deadline")
	}
}

func TestDNSDoHForcedHTTP2DoesNotSendHTTP1(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	defer srv.Close()
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	task := dnsFixtureTask(t, "doh", u.Port())
	task.Certificate = &HTTPOptCertificate{IgnoreServerCertificateError: true}
	task.DoH = &DNSDoHOption{URL: srv.URL, HTTPProtocol: "http/2-only"}
	dnsAssertRun(t, task, false)
	require.Zero(t, calls.Load())
}

func TestDNSFactoryAcceptsKodoPushPayload(t *testing.T) {
	for _, class := range []string{"dns", ClassDNS} {
		child, err := CreateTaskChild(class)
		require.NoError(t, err)
		payload := struct {
			Regions map[string]bool `json:"regions"`
			Task    TaskChild       `json:"task"`
		}{Task: child}
		require.NoError(t, json.Unmarshal([]byte(`{
			"regions":{"dns-node":true},
			"task":{"external_id":"dns-task","name":"DNS test","status":"OK","frequency":"1m",
			"domain":"example.com","dns_server":"resolver.example.com","protocol":"auto",
			"success_when":[{"at_least_one_record":{"of_type_a":[{"exists":true}]}}]}
		}`), &payload))
		task, err := NewTask("", payload.Task)
		require.NoError(t, err)
		require.NoError(t, task.Check())
		require.Equal(t, ClassDNS, task.Class())
		require.Equal(t, "dns_dial_testing", task.MetricName())
		// Kodo copies the task JSON per region; DK consumes that same JSON.
		dkChild, err := CreateTaskChild(ClassDNS)
		require.NoError(t, err)
		pulledTask, err := NewTask(task.String(), dkChild)
		require.NoError(t, err)
		require.NoError(t, pulledTask.RenderTemplateAndInit(nil))
		require.Equal(t, task.GetExternalID(), pulledTask.GetExternalID())
		require.Equal(t, []string{"A"}, pulledTask.(*DNSTask).queryTypes)
	}
}
