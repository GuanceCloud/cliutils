// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateSSLTaskChild(t *testing.T) {
	child, err := CreateTaskChild("ssl")
	require.NoError(t, err)
	assert.IsType(t, &SSLTask{}, child)

	child, err = CreateTaskChild(ClassSSL)
	require.NoError(t, err)
	assert.IsType(t, &SSLTask{}, child)
}

func TestSSLTaskCheckAndResult(t *testing.T) {
	task, err := NewTask("", &SSLTask{
		Task: &Task{
			ExternalID: "ssl-task",
			Name:       "ssl-task",
			Frequency:  "1m",
		},
		Host: "example.com",
		Port: "443",
		SuccessWhen: []*SSLSuccess{
			{
				ResponseTime: "1s",
				CertificateExpiresInDays: []*ValueSuccess{
					{Op: "gt", Target: 7},
				},
				TLSVersion: []*SuccessOption{
					{Is: "TLS1.3"},
				},
				Subject: []*SuccessOption{
					{Contains: "example.com"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, task.Check())

	sslTask := task.(*SSLTask)
	sslTask.reqCost = 100 * time.Millisecond
	sslTask.tlsHandshakeCost = 80 * time.Millisecond
	sslTask.sslCertExpiresInDays = 30
	sslTask.sslCertNotAfter = time.Now().Add(30 * 24 * time.Hour).UnixMicro()
	sslTask.tlsVersion = "TLS1.3"
	sslTask.certSubject = "CN=example.com"
	sslTask.certIssuer = "CN=Example CA"

	reasons, ok := sslTask.CheckResult()
	assert.True(t, ok)
	assert.Empty(t, reasons)

	tags, fields := sslTask.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, int64(1), fields["success"])
	assert.Equal(t, "TLS1.3", fields["tls_version"])
	assert.Equal(t, int64(80000), fields["tls_handshake_time"])
	assert.Equal(t, int64(30), fields["ssl_cert_expires_in_days"])
}

func TestSSLTaskCheckResultCertificateExpiresSoon(t *testing.T) {
	task := &SSLTask{
		Task: &Task{Name: "ssl-task"},
		SuccessWhen: []*SSLSuccess{
			{
				CertificateExpiresInDays: []*ValueSuccess{
					{Op: "gt", Target: 7},
				},
			},
		},
		sslCertExpiresInDays: 3,
	}

	reasons, ok := task.checkResult()
	assert.False(t, ok)
	require.Len(t, reasons, 1)
	assert.Contains(t, reasons[0], "SSL certificate expires in days check failed")
}

func TestSSLTaskRunSuccess(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	require.NoError(t, err)

	task, err := NewTask("", &SSLTask{
		Task: &Task{
			ExternalID: "ssl-run-success",
			Name:       "ssl-run-success",
			Frequency:  "1m",
		},
		Host:                         host,
		Port:                         port,
		ServerName:                   "example.test",
		Timeout:                      "3s",
		IgnoreServerCertificateError: true,
		SuccessWhen: []*SSLSuccess{
			{
				ResponseTime: "3s",
				CertificateExpiresInDays: []*ValueSuccess{
					{Op: "gt", Target: 0},
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, task.Check())
	require.NoError(t, task.Run())

	sslTask := task.(*SSLTask)
	assert.Empty(t, sslTask.reqError)
	assert.NotZero(t, sslTask.reqCost)
	assert.NotZero(t, sslTask.tlsHandshakeCost)
	assert.LessOrEqual(t, sslTask.tlsHandshakeCost, sslTask.reqCost)
	assert.Equal(t, host, sslTask.destIP)
	assert.NotEmpty(t, sslTask.tlsVersion)

	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, "example.test", tags["server_name"])
	assert.Equal(t, int64(1), fields["success"])
	assert.NotEmpty(t, fields["task"])
	assert.NotEmpty(t, fields["config_vars"])
}

func TestSSLTaskClearResetsTimingFields(t *testing.T) {
	task := &SSLTask{
		reqCost:          time.Second,
		tlsHandshakeCost: 500 * time.Millisecond,
		reqError:         "failed",
		destIP:           "127.0.0.1",
		tlsVersion:       "TLS1.3",
	}

	task.clear()

	assert.Zero(t, task.reqCost)
	assert.Zero(t, task.tlsHandshakeCost)
	assert.Empty(t, task.reqError)
	assert.Empty(t, task.destIP)
	assert.Empty(t, task.tlsVersion)
}

func TestSSLTaskRunFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	require.NoError(t, listener.Close())

	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	task := &SSLTask{
		Task:    &Task{Name: "ssl-run-failure"},
		Host:    host,
		Port:    port,
		timeout: time.Second,
		SuccessWhen: []*SSLSuccess{
			{
				ResponseTime: "1s",
				CertificateExpiresInDays: []*ValueSuccess{
					{Op: "gt", Target: 7},
				},
			},
		},
	}

	require.NoError(t, task.run())
	assert.NotEmpty(t, task.reqError)

	tags, fields := task.getResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Equal(t, int64(-1), fields["success"])
	assert.Contains(t, fields["fail_reason"], "connect")
	assert.NotContains(t, fields["fail_reason"], "SSL certificate expires in days")
}

func TestSSLTaskCheckErrors(t *testing.T) {
	assert.EqualError(t, (&SSLTask{}).check(), "host should not be empty")
	assert.EqualError(t, (&SSLTask{Host: "example.com"}).check(), "port should not be empty")
}

func TestSSLTaskInitErrors(t *testing.T) {
	err := (&SSLTask{Timeout: "bad", SuccessWhen: []*SSLSuccess{{}}}).init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")

	err = (&SSLTask{}).init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no any check rule")

	err = (&SSLTask{SuccessWhen: []*SSLSuccess{{ResponseTime: "bad"}}}).init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid duration")

	err = (&SSLTask{SuccessWhen: []*SSLSuccess{{Subject: []*SuccessOption{{MatchRegex: "["}}}}}).init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing")

	err = (&SSLTask{SuccessWhen: []*SSLSuccess{{Issuer: []*SuccessOption{{MatchRegex: "["}}}}}).init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing")

	err = (&SSLTask{SuccessWhen: []*SSLSuccess{{TLSVersion: []*SuccessOption{{MatchRegex: "["}}}}}).init()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing closing")
}

func TestSSLTaskCheckResultFailures(t *testing.T) {
	task := &SSLTask{
		reqCost:              2 * time.Second,
		sslCertExpiresInDays: 3,
		sslCertNotAfter:      100,
		certSubject:          "CN=unexpected",
		certIssuer:           "CN=Unexpected CA",
		tlsVersion:           "TLS1.2",
		SuccessWhen: []*SSLSuccess{
			{
				respTime:                 time.Second,
				CertificateExpiresInDays: []*ValueSuccess{{Op: "gt", Target: 7}},
				CertificateNotAfter:      []*ValueSuccess{{Op: "gt", Target: 200}},
				Subject:                  []*SuccessOption{{Contains: "example.com"}},
				Issuer:                   []*SuccessOption{{Contains: "Expected CA"}},
				TLSVersion:               []*SuccessOption{{Is: "TLS1.3"}},
			},
		},
	}

	reasons, ok := task.checkResult()
	assert.False(t, ok)
	require.Len(t, reasons, 6)
	assert.Contains(t, reasons[0], "SSL response time")
	assert.Contains(t, reasons[1], "SSL certificate expires in days")
	assert.Contains(t, reasons[2], "SSL certificate not after")
	assert.Contains(t, reasons[3], "SSL certificate subject")
	assert.Contains(t, reasons[4], "SSL certificate issuer")
	assert.Contains(t, reasons[5], "TLS version")
}

func TestSSLTaskGetResultsOrLogicFailure(t *testing.T) {
	task := &SSLTask{
		Task:             &Task{Name: "ssl-task", Tags: map[string]string{"env": "test"}},
		Host:             "example.com",
		Port:             "443",
		ServerName:       "sni.example.com",
		SuccessWhenLogic: "or",
		SuccessWhen:      []*SSLSuccess{{respTime: time.Second}},
		reqCost:          2 * time.Second,
		reqError:         "handshake failed",
	}

	tags, fields := task.getResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Equal(t, "test", tags["env"])
	assert.Equal(t, "sni.example.com", tags["server_name"])
	assert.Equal(t, int64(-1), fields["success"])
	assert.Contains(t, fields["fail_reason"], "handshake failed")
	assert.Contains(t, fields["message"], "FAIL")
}

func TestSSLTaskHelpers(t *testing.T) {
	task := &SSLTask{
		Task:                 &Task{Name: "ssl-task"},
		Host:                 "example.com",
		Port:                 "443",
		reqCost:              time.Second,
		reqError:             "err",
		destIP:               "127.0.0.1",
		tlsVersion:           "TLS1.3",
		sslCertNotBefore:     1,
		sslCertNotAfter:      2,
		sslCertExpiresInDays: 3,
		certSubject:          "subject",
		certIssuer:           "issuer",
	}

	assert.Equal(t, "ssl_dial_testing", task.metricName())
	assert.Equal(t, ClassSSL, task.class())
	hosts, err := task.getHostName()
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com"}, hosts)
	assert.EqualError(t, func() error {
		_, err := task.getVariableValue(Variable{})
		return err
	}(), "not support")
	task.setReqError("new error")
	assert.Equal(t, "new error", task.reqError)
	task.stop()

	task.clear()
	assert.Zero(t, task.reqCost)
	assert.Empty(t, task.reqError)
	assert.Empty(t, task.destIP)
	assert.Empty(t, task.tlsVersion)
	assert.Zero(t, task.sslCertNotBefore)
	assert.Zero(t, task.sslCertNotAfter)
	assert.Zero(t, task.sslCertExpiresInDays)
	assert.Empty(t, task.certSubject)
	assert.Empty(t, task.certIssuer)

	task.Task = nil
	task.initTask()
	assert.NotNil(t, task.Task)

	raw, err := task.getRawTask(`{"host":"example.com","port":"443"}`)
	require.NoError(t, err)
	assert.Contains(t, raw, `"host":"example.com"`)

	_, err = task.getRawTask(`{`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal ssl task failed")
}

func TestSSLTaskRenderTemplate(t *testing.T) {
	task, err := NewTask("", &SSLTask{
		Task:       &Task{ConfigVars: []*ConfigVar{{Name: "host", Value: "example.com"}}},
		Host:       "{{ host }}",
		Port:       "{{ port }}",
		ServerName: "{{ server }}",
		SuccessWhen: []*SSLSuccess{
			{
				ResponseTime: "{{ timeout }}",
				Subject: []*SuccessOption{
					{Contains: "{{ subject }}"},
					{IsNot: "{{ subject_is_not }}"},
				},
				Issuer: []*SuccessOption{
					{MatchRegex: "{{ issuer_regex }}"},
					{NotContains: "{{ issuer_not_contains }}"},
				},
				TLSVersion: []*SuccessOption{
					{Is: "{{ tls_version }}"},
					{NotMatchRegex: "{{ tls_not_regex }}"},
				},
			},
		},
	})
	require.NoError(t, err)

	sslTask := task.(*SSLTask)
	err = sslTask.renderTemplate(map[string]any{
		"host":   func() string { return "example.org" },
		"port":   func() string { return "443" },
		"server": func() string { return "sni.example.org" },
		"timeout": func() string {
			return "2s"
		},
		"subject": func() string {
			return "example.org"
		},
		"subject_is_not": func() string {
			return "bad.example.org"
		},
		"issuer_regex": func() string {
			return "Example.*CA"
		},
		"issuer_not_contains": func() string {
			return "Bad CA"
		},
		"tls_version": func() string {
			return "TLS1.3"
		},
		"tls_not_regex": func() string {
			return "TLS1\\.0"
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "example.org", sslTask.Host)
	assert.Equal(t, "443", sslTask.Port)
	assert.Equal(t, "sni.example.org", sslTask.ServerName)
	require.NoError(t, sslTask.init())
	assert.Equal(t, 2*time.Second, sslTask.SuccessWhen[0].respTime)
	assert.Equal(t, "example.org", sslTask.SuccessWhen[0].Subject[0].Contains)
	assert.Equal(t, "bad.example.org", sslTask.SuccessWhen[0].Subject[1].IsNot)
	assert.Equal(t, "Example.*CA", sslTask.SuccessWhen[0].Issuer[0].MatchRegex)
	assert.Equal(t, "Bad CA", sslTask.SuccessWhen[0].Issuer[1].NotContains)
	assert.Equal(t, "TLS1.3", sslTask.SuccessWhen[0].TLSVersion[0].Is)
	assert.Equal(t, "TLS1\\.0", sslTask.SuccessWhen[0].TLSVersion[1].NotMatchRegex)
	require.NoError(t, sslTask.renderSuccessWhen(nil, map[string]any{}))

	sslTask.rawTask = nil
	sslTask.SetTaskJSONString("")
	err = sslTask.renderTemplate(map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new raw task failed")

	err = (&SSLTask{Task: &Task{}, rawTask: nil}).renderTemplate(map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "new raw task failed")

	badTask, err := NewTask("", &SSLTask{
		Task: &Task{},
		Host: "example.com",
		Port: "443",
		SuccessWhen: []*SSLSuccess{
			{ResponseTime: "{{"},
		},
	})
	require.NoError(t, err)
	err = badTask.(*SSLTask).renderTemplate(map[string]any{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "render response time failed")
}

func TestSSLTaskExtractCertificateInfoEmpty(t *testing.T) {
	task := &SSLTask{}
	task.extractCertificateInfo(tls.ConnectionState{})
	assert.Zero(t, task.sslCertNotAfter)
}

func TestSSLTaskExtractCertificateInfo(t *testing.T) {
	notBefore := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	notAfter := time.Now().Add(48 * time.Hour).Truncate(time.Microsecond)
	task := &SSLTask{}

	task.extractCertificateInfo(tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{
			{
				Subject:   pkix.Name{CommonName: "example.com"},
				Issuer:    pkix.Name{CommonName: "Example CA"},
				NotBefore: notBefore,
				NotAfter:  notAfter,
			},
		},
	})

	assert.Equal(t, notBefore.UnixMicro(), task.sslCertNotBefore)
	assert.Equal(t, notAfter.UnixMicro(), task.sslCertNotAfter)
	assert.GreaterOrEqual(t, task.sslCertExpiresInDays, int64(1))
	assert.Contains(t, task.certSubject, "example.com")
	assert.Contains(t, task.certIssuer, "Example CA")
}

func TestTLSVersionString(t *testing.T) {
	assert.Equal(t, "TLS1.0", tlsVersionString(tls.VersionTLS10))
	assert.Equal(t, "TLS1.1", tlsVersionString(tls.VersionTLS11))
	assert.Equal(t, "TLS1.2", tlsVersionString(tls.VersionTLS12))
	assert.Equal(t, "TLS1.3", tlsVersionString(tls.VersionTLS13))
	assert.Equal(t, "0x1234", tlsVersionString(0x1234))
}
