// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func dnsFixtureReply(packet []byte) []byte {
	var m dnsmessage.Message
	if m.Unpack(packet) != nil || len(m.Questions) != 1 {
		return nil
	}
	m.Response = true
	m.RecursionAvailable = true
	m.AuthenticData = true
	m.Additionals = nil
	q := m.Questions[0]
	header := dnsmessage.ResourceHeader{Name: q.Name, Type: q.Type, Class: dnsmessage.ClassINET, TTL: 300}
	var body dnsmessage.ResourceBody
	//exhaustive:ignore // Fixture supports only the record types exposed by DNS tasks.
	switch q.Type {
	case dnsmessage.TypeA:
		body = &dnsmessage.AResource{A: [4]byte{203, 0, 113, 10}}
	case dnsmessage.TypeAAAA:
		body = &dnsmessage.AAAAResource{AAAA: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}}
	case dnsmessage.TypeCNAME:
		n, _ := dnsmessage.NewName("edge.example.test.")
		body = &dnsmessage.CNAMEResource{CNAME: n}
	case dnsmessage.TypeNS:
		n, _ := dnsmessage.NewName("ns.example.test.")
		body = &dnsmessage.NSResource{NS: n}
	case dnsmessage.TypeMX:
		n, _ := dnsmessage.NewName("mail.example.test.")
		body = &dnsmessage.MXResource{Pref: 10, MX: n}
	case dnsmessage.TypeTXT:
		body = &dnsmessage.TXTResource{TXT: []string{"verification=", "example"}}
	default:
		return nil
	}
	if body != nil {
		m.Answers = []dnsmessage.Resource{{Header: header, Body: body}}
	}
	p, _ := m.Pack()
	return p
}

func dnsFixtureTCP(t *testing.T, ln net.Listener, reply func([]byte) []byte) {
	t.Helper()
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close() //nolint:errcheck
				_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
				for {
					packet, err := dnsReadFrame(conn)
					if err != nil {
						return
					}
					out := reply(packet)
					if out == nil {
						continue
					}
					if dnsWriteFrame(conn, out) != nil {
						return
					}
				}
			}()
		}
	}()
}

func dnsFixturePlain(t *testing.T, reply func([]byte) []byte, truncate bool) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	dnsFixtureTCP(t, ln, reply)
	addr, err := net.ResolveUDPAddr("udp", ln.Addr().String())
	require.NoError(t, err)
	udp, err := net.ListenUDP("udp", addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = udp.Close() })
	go func() {
		b := make([]byte, 65535)
		for {
			n, remote, e := udp.ReadFromUDP(b)
			if e != nil {
				return
			}
			out := reply(b[:n])
			if out == nil {
				continue
			}
			if truncate {
				var m dnsmessage.Message
				if m.Unpack(out) == nil {
					m.Truncated = true
					m.Answers = nil
					out, _ = m.Pack()
				}
			}
			_, _ = udp.WriteToUDP(out, remote)
		}
	}()
	return strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
}

func dnsFixtureCert(t *testing.T) (tls.Certificate, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	cert := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "DNS fixture"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), DNSNames: []string{"resolver.test"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)
	return pair, string(certPEM)
}

func dnsFixtureTask(t *testing.T, protocol, port string) *DNSTask {
	t.Helper()
	task := &DNSTask{Task: &Task{Name: "DNS fixture", ExternalID: "dns-fixture", Frequency: "1m"}, Domain: "www.example.test", DNSServer: "127.0.0.1", Port: port, Protocol: protocol, Timeout: "1s", SuccessWhen: []*DNSSuccess{{ResponseTime: "1s"}, {AtLeastOneRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{{SuccessOption: SuccessOption{Is: "203.0.113.10"}}}}}}}
	_, err := NewTask("", task)
	require.NoError(t, err)
	return task
}

func dnsAssertRun(t *testing.T, task *DNSTask, want bool) map[string]any {
	t.Helper()
	require.NoError(t, task.Check())
	require.NoError(t, task.Run())
	tags, fields := task.GetResults()
	if want {
		require.Equal(t, "OK", tags["status"], "result: %v", fields)
		require.EqualValues(t, 1, fields["success"])
		require.NotContains(t, fields, "fail_reason")
	} else {
		require.Equal(t, "FAIL", tags["status"])
		require.EqualValues(t, -1, fields["success"])
		require.NotEmpty(t, fields["fail_reason"])
	}
	var queries, assertions any
	require.NoError(t, json.Unmarshal([]byte(fields["dns_queries"].(string)), &queries))
	require.NoError(t, json.Unmarshal([]byte(fields["assertion_results"].(string)), &assertions))
	_, second := task.GetResults()
	require.Equal(t, fields, second, "reading results must be idempotent")
	return fields
}

func TestDNSPlainProtocols(t *testing.T) {
	for _, protocol := range []string{"auto", "udp", "tcp"} {
		t.Run(protocol, func(t *testing.T) {
			port := dnsFixturePlain(t, dnsFixtureReply, false)
			fields := dnsAssertRun(t, dnsFixtureTask(t, protocol, port), true)
			require.EqualValues(t, 1, fields["query_count"])
		})
	}
	t.Run("fallback", func(t *testing.T) {
		port := dnsFixturePlain(t, dnsFixtureReply, true)
		task := dnsFixtureTask(t, "auto", port)
		dnsAssertRun(t, task, true)
		require.Len(t, task.result.Queries[0].Attempts, 2)
		require.Equal(t, "tcp", task.result.Queries[0].Attempts[1].Transport)
	})
	t.Run("udp does not fall back", func(t *testing.T) {
		port := dnsFixturePlain(t, dnsFixtureReply, true)
		fields := dnsAssertRun(t, dnsFixtureTask(t, "udp", port), false)
		require.Equal(t, "truncated_response", fields["error_code"])
	})
}

func TestDNSEncryptedProtocols(t *testing.T) {
	pair, ca := dnsFixtureCert(t)
	t.Run("dot", func(t *testing.T) {
		ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12})
		require.NoError(t, err)
		dnsFixtureTCP(t, ln, dnsFixtureReply)
		task := dnsFixtureTask(t, "dot", strconv.Itoa(ln.Addr().(*net.TCPAddr).Port))
		task.DNSServer, task.ServerIP = "resolver.test", "127.0.0.1"
		task.Certificate = &HTTPOptCertificate{CaCert: ca}
		fields := dnsAssertRun(t, task, true)
		require.Equal(t, true, fields["certificate_verified"])
		task.ServerName = "wrong.test"
		fields = dnsAssertRun(t, task, false)
		require.Equal(t, "tls_error", fields["error_code"])
		require.NotContains(t, fields, "certificate_verified")
	})
	for _, method := range []string{"POST", "GET"} {
		for _, version := range []string{"http/1.1", "http/2-only"} {
			t.Run("doh-"+method+"-"+version, func(t *testing.T) {
				server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var packet []byte
					var err error
					if r.Method == "GET" {
						packet, err = base64.RawURLEncoding.DecodeString(r.URL.Query().Get("dns"))
					} else {
						packet, err = io.ReadAll(r.Body)
					}
					if err != nil {
						w.WriteHeader(400)
						return
					}
					w.Header().Set("Content-Type", "application/dns-message")
					_, _ = w.Write(dnsFixtureReply(packet))
				}))
				server.EnableHTTP2 = true
				server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS12}
				server.StartTLS()
				t.Cleanup(server.Close)
				task := dnsFixtureTask(t, "doh", "")
				task.DNSServer = "resolver.test"
				task.ServerIP = "127.0.0.1"
				task.Certificate = &HTTPOptCertificate{CaCert: ca}
				task.DoH = &DNSDoHOption{URL: strings.Replace(server.URL, "127.0.0.1", "resolver.test", 1) + "/dns-query", Method: method, HTTPProtocol: version}
				fields := dnsAssertRun(t, task, true)
				require.Equal(t, true, fields["certificate_verified"])
				require.EqualValues(t, 200, fields["http_status_code"])
			})
		}
	}
	t.Run("doq", func(t *testing.T) {
		ln, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{Certificates: []tls.Certificate{pair}, NextProtos: []string{"doq"}, MinVersion: tls.VersionTLS13}, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = ln.Close() })
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		go func() {
			for {
				conn, e := ln.Accept(ctx)
				if e != nil {
					return
				}
				go func() {
					defer conn.CloseWithError(0, "") //nolint:errcheck
					for {
						stream, e := conn.AcceptStream(ctx)
						if e != nil {
							return
						}
						p, e := dnsReadFrame(stream)
						if e != nil {
							return
						}
						_ = dnsWriteFrame(stream, dnsFixtureReply(p))
						_ = stream.Close()
					}
				}()
			}
		}()
		_, port, err := net.SplitHostPort(ln.Addr().String())
		require.NoError(t, err)
		task := dnsFixtureTask(t, "doq", port)
		task.DNSServer, task.ServerIP = "resolver.test", "127.0.0.1"
		task.Certificate = &HTTPOptCertificate{CaCert: ca}
		dnsAssertRun(t, task, true)
	})
	t.Run("doh-http3", func(t *testing.T) {
		udp, err := net.ListenPacket("udp", "127.0.0.1:0")
		require.NoError(t, err)
		server := &http3.Server{TLSConfig: &tls.Config{Certificates: []tls.Certificate{pair}, MinVersion: tls.VersionTLS13}, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, e := io.ReadAll(r.Body)
			if e != nil {
				return
			}
			w.Header().Set("Content-Type", "application/dns-message")
			_, _ = w.Write(dnsFixtureReply(b))
		})}
		go func() { _ = server.Serve(udp) }()
		t.Cleanup(func() { _ = server.Close(); _ = udp.Close() })
		_, port, err := net.SplitHostPort(udp.LocalAddr().String())
		require.NoError(t, err)
		task := dnsFixtureTask(t, "doh", "")
		task.DNSServer = "resolver.test"
		task.ServerIP = "127.0.0.1"
		task.Certificate = &HTTPOptCertificate{CaCert: ca}
		task.DoH = &DNSDoHOption{URL: "https://resolver.test:" + port + "/dns-query", HTTPProtocol: "http/3"}
		fields := dnsAssertRun(t, task, true)
		require.Equal(t, "HTTP/3.0", fields["http_protocol"])
	})
}

func TestDNSConditionsAndFailure(t *testing.T) {
	port := dnsFixturePlain(t, func(p []byte) []byte {
		var m dnsmessage.Message
		_ = m.Unpack(dnsFixtureReply(p))
		if m.Questions[0].Type == dnsmessage.TypeA {
			h := m.Answers[0].Header
			m.Answers = append(m.Answers, dnsmessage.Resource{Header: h, Body: &dnsmessage.AResource{A: [4]byte{203, 0, 113, 11}}})
		}
		b, _ := m.Pack()
		return b
	}, false)
	task := dnsFixtureTask(t, "tcp", port)
	task.SuccessWhen = task.SuccessWhen[1:]
	dnsAssertRun(t, task, true)
	task.SuccessWhen[0].EveryRecord = task.SuccessWhen[0].AtLeastOneRecord
	task.SuccessWhen[0].AtLeastOneRecord = nil
	dnsAssertRun(t, task, false)
	task.SuccessWhen = append(task.SuccessWhen, &DNSSuccess{ResponseTime: "1s"})
	task.SuccessWhenLogic = "or"
	dnsAssertRun(t, task, true)
	t.Run("empty every", func(t *testing.T) {
		port := dnsFixturePlain(t, func(p []byte) []byte {
			var m dnsmessage.Message
			_ = m.Unpack(dnsFixtureReply(p))
			m.Answers = nil
			b, _ := m.Pack()
			return b
		}, false)
		task := dnsFixtureTask(t, "tcp", port)
		task.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{{Exists: true}}}}}
		dnsAssertRun(t, task, false)
	})
	t.Run("rcode cannot be masked by OR", func(t *testing.T) {
		port := dnsFixturePlain(t, func(p []byte) []byte {
			var m dnsmessage.Message
			_ = m.Unpack(dnsFixtureReply(p))
			m.RCode = 3
			m.Answers = nil
			b, _ := m.Pack()
			return b
		}, false)
		task := dnsFixtureTask(t, "udp", port)
		task.SuccessWhenLogic = "or"
		fields := dnsAssertRun(t, task, false)
		require.Equal(t, "dns_rcode_error", fields["error_code"])
		require.Contains(t, fields["assertion_results"], "skipped")
	})
	t.Run("all types and flags", func(t *testing.T) {
		task := dnsFixtureTask(t, "tcp", port)
		task.RecordTypes = dnsRecordTypes
		yes := true
		task.SuccessWhen = []*DNSSuccess{{AuthenticatedData: &yes}}
		fields := dnsAssertRun(t, task, true)
		require.EqualValues(t, 6, fields["query_count"])
		require.EqualValues(t, 7, fields["answer_count"])
	})
}

func TestDNSAddressBudgetAndCancel(t *testing.T) {
	port := dnsFixturePlain(t, dnsFixtureReply, false)
	t.Run("validate actual address once", func(t *testing.T) {
		task := dnsFixtureTask(t, "tcp", port)
		task.DNSServer = "resolver.test"
		calls := 0
		task.resolve = func(context.Context, string) ([]net.IPAddr, error) {
			calls++
			return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
		}
		task.SetResolvedIPValidator(func(_ context.Context, ip net.IP) error {
			require.True(t, ip.IsLoopback())
			return errors.New("denied")
		})
		fields := dnsAssertRun(t, task, false)
		require.Equal(t, 1, calls)
		require.Equal(t, "address_denied", fields["error_code"])
	})
	t.Run("bootstrap included in budget", func(t *testing.T) {
		task := dnsFixtureTask(t, "udp", port)
		task.Timeout = "20ms"
		task.DNSServer = "resolver.test"
		task.resolve = func(ctx context.Context, _ string) ([]net.IPAddr, error) { <-ctx.Done(); return nil, ctx.Err() }
		fields := dnsAssertRun(t, task, false)
		require.Equal(t, "timeout", fields["error_code"])
		require.GreaterOrEqual(t, fields["response_time"].(int64), int64(20000))
	})
	t.Run("stop cancels blocked read", func(t *testing.T) {
		blocked := dnsFixturePlain(t, func([]byte) []byte { return nil }, false)
		task := dnsFixtureTask(t, "tcp", blocked)
		task.Timeout = "10s"
		require.NoError(t, task.Check())
		done := make(chan error, 1)
		go func() { done <- task.Run() }()
		time.Sleep(20 * time.Millisecond)
		task.Stop()
		select {
		case e := <-done:
			require.NoError(t, e)
		case <-time.After(time.Second):
			t.Fatal("DNS stop did not cancel read")
		}
		_, fields := task.GetResults()
		require.Equal(t, "canceled", fields["error_code"])
	})
}

func TestDNSValidationAndTemplate(t *testing.T) {
	for _, raw := range []string{
		`{"domain":"x.test","dns_server":"127.0.0.1","success_when":[]}`,
		`{"domain":"x.test","dns_server":"127.0.0.1","success_when":[{}]}`,
		`{"domain":"x.test","dns_server":"127.0.0.1","success_when":[null]}`,
		`{"domain":"x.test","dns_server":"127.0.0.1","success_when":[{"at_least_one_record":{"of_type_a":[{"is":"x","contains":"y"}]}}]}`,
		`{"domain":"x.test","dns_server":"127.0.0.1","timeout":"0s","success_when":[{"response_time":"1s"}]}`,
		`{"domain":"x.test","dns_server":"127.0.0.1","dns_options":{"udp_payload_size":65535},"success_when":[{"response_time":"1s"}]}`,
	} {
		t.Run(raw, func(t *testing.T) {
			child, err := CreateTaskChild("dns")
			require.NoError(t, err)
			task, err := NewTask(raw, child)
			require.NoError(t, err)
			require.Error(t, task.CheckTask())
		})
	}
	t.Run("template escaping and repeat run clears error", func(t *testing.T) {
		port := dnsFixturePlain(t, dnsFixtureReply, false)
		task := dnsFixtureTask(t, "tcp", port)
		task.Domain = "{{domain}}"
		task.ConfigVars = []*ConfigVar{{Name: "domain", Value: "www.example.test"}}
		task.SuccessWhen = []*DNSSuccess{{AtLeastOneRecord: &DNSRecordAssertions{TXT: []*DNSRecordCondition{{SuccessOption: SuccessOption{Is: "verification=example"}}}}}}
		_, err := NewTask("", task)
		require.NoError(t, err)
		require.NoError(t, task.RenderTemplateAndInit(nil))
		dnsAssertRun(t, task, true)
		task.result.Error = "old error"
		dnsAssertRun(t, task, true)
	})
	t.Run("results are bounded valid JSON", func(t *testing.T) {
		task := dnsFixtureTask(t, "tcp", "53")
		require.NoError(t, task.Check())
		task.result.Started = true
		task.result.Cost = time.Millisecond
		records := make([]dnsRecord, 300)
		for i := range records {
			records[i] = dnsRecord{Type: "A", Value: "203.0.113.10"}
		}
		task.result.Queries = []dnsQueryResult{{Type: "A", RCode: "NOERROR", Status: "OK", Records: records, Count: 300}}
		_, fields := task.GetResults()
		require.Equal(t, true, fields["details_truncated"])
		var result any
		require.NoError(t, json.Unmarshal([]byte(fields["dns_queries"].(string)), &result))
	})
	t.Run("equal domain and IP", func(t *testing.T) {
		require.True(t, dnsEqual("NS", "Ns.Example.Test.", "ns.example.test"))
		require.True(t, dnsEqual("AAAA", "2001:db8::1", "2001:0db8:0:0:0:0:0:1"))
		require.False(t, dnsEqual("TXT", "Value", "value"))
	})
}

func TestDNSDoHRejectsInvalidResponses(t *testing.T) {
	for _, mode := range []string{"redirect", "status", "media", "body"} {
		t.Run(mode, func(t *testing.T) {
			var mu sync.Mutex
			count := 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				count++
				mu.Unlock()
				switch mode {
				case "redirect":
					http.Redirect(w, r, "https://elsewhere.test/dns-query", 302)
				case "status":
					w.WriteHeader(503)
				case "media":
					_, _ = w.Write([]byte("html"))
				case "body":
					w.Header().Set("Content-Type", "application/dns-message")
					_, _ = w.Write([]byte("bad"))
				}
			}))
			defer server.Close()
			task := dnsFixtureTask(t, "doh", "")
			task.DNSServer = ""
			task.DoH = &DNSDoHOption{URL: server.URL}
			task.Certificate = &HTTPOptCertificate{IgnoreServerCertificateError: true}
			fields := dnsAssertRun(t, task, false)
			require.Equal(t, "http_error", fields["error_code"])
			mu.Lock()
			require.Equal(t, 1, count)
			mu.Unlock()
		})
	}
}
