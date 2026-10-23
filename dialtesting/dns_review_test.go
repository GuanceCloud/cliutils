// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

func TestDNSDeadlineBeforeQueryIsFailure(t *testing.T) {
	task := dnsFixtureTask(t, "udp", "53")
	task.Timeout = "1ms"
	task.SuccessWhen = []*DNSSuccess{{ResponseTime: "1s"}}
	task.SetResolvedIPValidator(func(ctx context.Context, _ net.IP) error {
		<-ctx.Done()
		return nil
	})
	fields := dnsAssertRun(t, task, false)
	require.Equal(t, "timeout", fields["error_code"])
	require.Contains(t, fields["assertion_results"], `"status":"skipped"`)
}

func TestDNSRejectsScopedServerIP(t *testing.T) {
	task := dnsFixtureTask(t, "udp", "53")
	task.ServerIP = "fe80::1%eth0"
	require.Error(t, task.CheckTask())
}

func TestDNSTemplatesDoNotBypassStaticValidation(t *testing.T) {
	task := dnsFixtureTask(t, "invalid-protocol", "53")
	task.Name = "{{task_name}}"
	require.Error(t, task.CheckTask())
	task.Domain = "{{domain}}"
	require.Error(t, task.CheckTask())
}

func TestDNSNullPayloadDoesNotPanic(t *testing.T) {
	require.NotPanics(t, func() {
		task, err := NewTask("null", &DNSTask{})
		require.Error(t, err)
		require.Nil(t, task)
	})
	require.NotPanics(t, func() {
		task, err := NewTask("", nil)
		require.Error(t, err)
		require.Nil(t, task)
	})
}

func TestDNSValidationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update func(*DNSTask)
	}{
		{"empty domain", func(d *DNSTask) { d.Domain = "" }},
		{"empty label", func(d *DNSTask) { d.Domain = "bad..example" }},
		{"long label", func(d *DNSTask) { d.Domain = strings.Repeat("a", 64) + ".example" }},
		{"long name", func(d *DNSTask) { d.Domain = strings.Repeat(strings.Repeat("a", 63)+".", 4) + "a" }},
		{"invalid character", func(d *DNSTask) { d.Domain = "https://example.com" }},
		{"timeout maximum", func(d *DNSTask) { d.Timeout = "61s" }},
		{"logic", func(d *DNSTask) { d.SuccessWhenLogic = "xor" }},
		{"port zero", func(d *DNSTask) { d.Port = "0" }},
		{"port overflow", func(d *DNSTask) { d.Port = "65536" }},
		{"dot port 53", func(d *DNSTask) { d.Protocol = "dot" }},
		{"doq port 53", func(d *DNSTask) { d.Protocol = "doq" }},
		{"server ip", func(d *DNSTask) { d.ServerIP = "resolver.test" }},
		{"scoped resolver", func(d *DNSTask) { d.DNSServer = "fe80::1%eth0" }},
		{"server name", func(d *DNSTask) { d.ServerName = "https://bad" }},
		{"client certificate pair", func(d *DNSTask) { d.Certificate = &HTTPOptCertificate{Certificate: "only-cert"} }},
		{"payload minimum", func(d *DNSTask) { d.DNSOptions = &DNSQueryOptions{UDPPayloadSize: 511} }},
		{"unsupported type", func(d *DNSTask) { d.RecordTypes = []string{"PTR"} }},
		{"missing asserted type", func(d *DNSTask) { d.RecordTypes = []string{"TXT"} }},
		{"empty condition", func(d *DNSTask) {
			d.SuccessWhen = []*DNSSuccess{{AtLeastOneRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{{}}}}}
		}},
		{"null condition", func(d *DNSTask) {
			d.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{nil}}}}
		}},
		{"empty record scope", func(d *DNSTask) { d.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{}}} }},
		{"invalid regex", func(d *DNSTask) {
			d.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{{SuccessOption: SuccessOption{MatchRegex: "["}}}}}}
		}},
		{"long regex", func(d *DNSTask) {
			d.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{A: []*DNSRecordCondition{{SuccessOption: SuccessOption{MatchRegex: strings.Repeat("a", 4097)}}}}}}
		}},
		{"maximum conditions", func(d *DNSTask) {
			d.SuccessWhen = make([]*DNSSuccess, 21)
			for i := range d.SuccessWhen {
				d.SuccessWhen[i] = &DNSSuccess{ResponseTime: "1s"}
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := dnsFixtureTask(t, "udp", "53")
			tc.update(task)
			require.Error(t, task.CheckTask())
			task.Name = "{{name}}"
			require.Error(t, task.CheckTask(), "templates must not hide static errors")
		})
	}
}

func TestDNSDoHValidationBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update func(*DNSTask)
	}{
		{"missing options", func(d *DNSTask) { d.DoH = nil }},
		{"insecure url", func(d *DNSTask) { d.DoH.URL = "http://resolver.test/dns-query" }},
		{"userinfo", func(d *DNSTask) { d.DoH.URL = "https://user:password@resolver.test/dns-query" }},
		{"fragment", func(d *DNSTask) { d.DoH.URL += "#fragment" }},
		{"host mismatch", func(d *DNSTask) { d.DNSServer = "other.test" }},
		{"port mismatch", func(d *DNSTask) { d.Port = "8443" }},
		{"method", func(d *DNSTask) { d.DoH.Method = "PUT" }},
		{"duplicate dns parameter", func(d *DNSTask) { d.DoH.Method = "GET"; d.DoH.URL += "?dns=encoded" }},
		{"http version", func(d *DNSTask) { d.DoH.HTTPProtocol = "h2c" }},
		{"reserved header", func(d *DNSTask) { d.DoH.Headers = map[string]string{"Host": "other.test"} }},
		{"invalid header name", func(d *DNSTask) { d.DoH.Headers = map[string]string{"bad key": "value"} }},
		{"invalid header value", func(d *DNSTask) { d.DoH.Headers = map[string]string{"X-Test": "bad\x00value"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := dnsFixtureTask(t, "doh", "")
			d.DNSServer = "resolver.test"
			d.DoH = &DNSDoHOption{URL: "https://resolver.test/dns-query"}
			tc.update(d)
			require.Error(t, d.CheckTask())
		})
	}
}

func TestDNSValidTemplatesAndSecureResults(t *testing.T) {
	port := dnsFixturePlain(t, dnsFixtureReply, false)
	task := dnsFixtureTask(t, "tcp", port)
	task.Domain = "{{domain}}"
	task.Timeout = "{{timeout}}"
	task.SuccessWhen = []*DNSSuccess{{ResponseTime: "{{threshold}}"}}
	task.ConfigVars = []*ConfigVar{{Name: "domain", Value: "sensitive.example.test", Secure: true}, {Name: "timeout", Value: "1s"}, {Name: "threshold", Value: "1s"}}
	_, err := NewTask("", task)
	require.NoError(t, err)
	require.NoError(t, task.CheckTask())
	require.NoError(t, task.RenderTemplateAndInit(nil))
	fields := dnsAssertRun(t, task, true)
	tags, _ := task.GetResults()
	data, err := json.Marshal([]any{tags, fields})
	require.NoError(t, err)
	require.NotContains(t, string(data), "sensitive.example.test")
	task.result.Error = "bootstrap sensitive.example.test failed"
	reasons, _ := task.CheckResult()
	require.NotContains(t, strings.Join(reasons, ","), "sensitive.example.test")
}

func TestDNSRecordStringConditions(t *testing.T) {
	port := dnsFixturePlain(t, dnsFixtureReply, false)
	for _, tc := range []struct {
		name      string
		condition SuccessOption
		want      bool
	}{
		{"contains", SuccessOption{Contains: "verification="}, true},
		{"contains mismatch", SuccessOption{Contains: "wrong"}, false},
		{"regex", SuccessOption{MatchRegex: `^verification=\w+$`}, true},
		{"regex mismatch", SuccessOption{MatchRegex: `^wrong`}, false},
		{"negative regex", SuccessOption{NotMatchRegex: `^wrong`}, true},
		{"negative regex mismatch", SuccessOption{NotMatchRegex: `verification`}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := dnsFixtureTask(t, "tcp", port)
			task.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{TXT: []*DNSRecordCondition{{SuccessOption: tc.condition}}}}}
			dnsAssertRun(t, task, tc.want)
		})
	}
}

func TestDNSQueriesShareOneDeadline(t *testing.T) {
	port := dnsFixturePlain(t, func(p []byte) []byte { time.Sleep(25 * time.Millisecond); return dnsFixtureReply(p) }, false)
	task := dnsFixtureTask(t, "tcp", port)
	task.Timeout = "40ms"
	task.RecordTypes = []string{"A", "AAAA", "TXT"}
	task.SuccessWhen = []*DNSSuccess{{ResponseTime: "1s"}}
	task.SuccessWhenLogic = "or"
	fields := dnsAssertRun(t, task, false)
	require.Equal(t, "timeout", fields["error_code"])
	require.Less(t, fields["response_time"].(int64), int64(150000))
	require.Contains(t, fields["dns_queries"], `"status":"SKIPPED"`)
	require.Contains(t, fields["assertion_results"], `"status":"skipped"`)
}

func TestDNSInvalidStreamFrame(t *testing.T) {
	port := dnsFixturePlain(t, func([]byte) []byte { return []byte{1} }, false)
	fields := dnsAssertRun(t, dnsFixtureTask(t, "tcp", port), false)
	require.Equal(t, "invalid_response", fields["error_code"])
	_, err := dnsReadFrame(bytes.NewReader([]byte{0, 11}))
	require.Error(t, err)
	_, err = dnsReadFrame(bytes.NewReader([]byte{0, 12, 1}))
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	require.Error(t, dnsWriteFrame(io.Discard, make([]byte, 65536)))
	require.ErrorIs(t, dnsWriteFrame(dnsZeroWriter{}, []byte{1}), io.ErrShortWrite)
}

type dnsZeroWriter struct{}

func (dnsZeroWriter) Write([]byte) (int, error) { return 0, nil }

func TestDNSBeforeRunFailureAndStoppedTask(t *testing.T) {
	task := dnsFixtureTask(t, "udp", "53")
	task.SetBeforeRun(func(*Task) error { return errors.New("blocked before execution") })
	fields := dnsAssertRun(t, task, false)
	require.NotContains(t, fields, "response_time")
	task.SetBeforeRun(nil)
	task.Stop()
	fields = dnsAssertRun(t, task, false)
	require.Equal(t, "canceled", fields["error_code"])
	_, err := task.GetVariableValue(Variable{})
	require.Error(t, err)
	hosts, err := task.GetHostName()
	require.NoError(t, err)
	require.Equal(t, []string{"127.0.0.1"}, hosts)
}

func TestDNSDoQRejectsMalformedStreams(t *testing.T) {
	cert, ca := dnsFixtureCert(t)
	for _, mode := range []string{"nonzero id", "extra message", "short frame"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			listener, err := quic.ListenAddr("127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, NextProtos: []string{"doq"}}, nil)
			require.NoError(t, err)
			defer listener.Close()
			request := make(chan []byte, 1)
			go func() {
				conn, e := listener.Accept(ctx)
				if e != nil {
					return
				}
				defer conn.CloseWithError(0, "")
				stream, e := conn.AcceptStream(ctx)
				if e != nil {
					return
				}
				packet, e := dnsReadFrame(stream)
				if e != nil {
					return
				}
				request <- packet
				reply := dnsFixtureReply(packet)
				switch mode {
				case "nonzero id":
					reply[0], reply[1] = 0, 1
					_ = dnsWriteFrame(stream, reply)
				case "extra message":
					_ = dnsWriteFrame(stream, reply)
					_ = dnsWriteFrame(stream, reply)
				case "short frame":
					_, _ = stream.Write([]byte{0, 1, 0})
				}
				_ = stream.Close()
				select {
				case <-conn.Context().Done():
				case <-ctx.Done():
				}
			}()
			_, port, _ := net.SplitHostPort(listener.Addr().String())
			task := dnsFixtureTask(t, "doq", port)
			task.Certificate = &HTTPOptCertificate{CaCert: ca}
			fields := dnsAssertRun(t, task, false)
			require.Equal(t, "invalid_response", fields["error_code"])
			select {
			case packet := <-request:
				var m dnsmessage.Message
				require.NoError(t, m.Unpack(packet))
				require.Zero(t, m.ID)
			default:
				t.Fatal("DoQ request did not reach server")
			}
		})
	}
}

func TestDNSRequestFlagsAndExtendedRCode(t *testing.T) {
	received := make(chan dnsmessage.Message, 1)
	port := dnsFixturePlain(t, func(packet []byte) []byte {
		var request dnsmessage.Message
		_ = request.Unpack(packet)
		received <- request
		var response dnsmessage.Message
		_ = response.Unpack(dnsFixtureReply(packet))
		root, _ := dnsmessage.NewName(".")
		header := dnsmessage.ResourceHeader{Name: root, Type: dnsmessage.TypeOPT}
		_ = header.SetEDNS0(1232, 16, false)
		response.Additionals = []dnsmessage.Resource{{Header: header, Body: &dnsmessage.UnknownResource{Type: dnsmessage.TypeOPT}}}
		out, _ := response.Pack()
		return out
	}, false)
	task := dnsFixtureTask(t, "udp", port)
	no, yes := false, true
	task.DNSOptions = &DNSQueryOptions{RecursionDesired: &no, DNSSECOK: true, CheckingDisabled: true, UDPPayloadSize: 1232}
	task.SuccessWhen = []*DNSSuccess{{AuthenticatedData: &yes}}
	fields := dnsAssertRun(t, task, false)
	require.Equal(t, "dns_rcode_error", fields["error_code"])
	require.Contains(t, fields["dns_queries"], `"rcode":"16"`)
	message := <-received
	require.False(t, message.RecursionDesired)
	require.True(t, message.AuthenticData)
	require.True(t, message.CheckingDisabled)
	require.Len(t, message.Additionals, 1)
	require.Equal(t, uint16(1232), uint16(message.Additionals[0].Header.Class))
	require.NotZero(t, message.Additionals[0].Header.TTL&(1<<15))
}

func TestDNSTruncationCannotHideFailedRecord(t *testing.T) {
	task := dnsFixtureTask(t, "tcp", "53")
	task.SuccessWhen = []*DNSSuccess{{EveryRecord: &DNSRecordAssertions{TXT: []*DNSRecordCondition{{SuccessOption: SuccessOption{Contains: "ok"}}}}}}
	require.NoError(t, task.CheckTask())
	records := make([]dnsRecord, 300)
	for i := range records {
		records[i] = dnsRecord{Type: "TXT", Value: strings.Repeat("ok", 200)}
	}
	records[299].Value = "invalid-last-record"
	task.result.Queries = []dnsQueryResult{{Type: "TXT", Status: "OK", RCode: "NOERROR", Records: records, Count: len(records)}}
	task.result.Started = true
	tags, fields := task.GetResults()
	require.Equal(t, "FAIL", tags["status"])
	require.Equal(t, true, fields["details_truncated"])
	require.EqualValues(t, 300, fields["answer_count"])
	require.LessOrEqual(t, len(fields["dns_queries"].(string)), 32768)
	require.LessOrEqual(t, len(fields["assertion_results"].(string)), 32768)
	require.Contains(t, fields["assertion_results"], `"status":"failed"`)
	require.Contains(t, fields["assertion_results"], `"actual_truncated":true`)
	require.NotContains(t, fields["dns_queries"], "invalid-last-record")
	var queries, assertions any
	require.NoError(t, json.Unmarshal([]byte(fields["dns_queries"].(string)), &queries))
	require.NoError(t, json.Unmarshal([]byte(fields["assertion_results"].(string)), &assertions))
	againTags, againFields := task.GetResults()
	require.Equal(t, tags, againTags)
	require.Equal(t, fields, againFields)
}

func TestDNSSecretValuesCannotChangeStatusEnums(t *testing.T) {
	port := dnsFixturePlain(t, dnsFixtureReply, false)
	task := dnsFixtureTask(t, "udp", port)
	task.ConfigVars = []*ConfigVar{{Name: "secret", Value: "OK", Secure: true}}
	dnsAssertRun(t, task, true)
	task.Tags = map[string]string{"status": "FAIL", "dest_ip": "fake", "proto": "http"}
	dnsAssertRun(t, task, true)
	tags, _ := task.GetResults()
	require.Equal(t, "127.0.0.1", tags["dest_ip"])
	require.Equal(t, "dns", tags["proto"])
}
