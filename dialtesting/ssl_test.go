package dialtesting

import (
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
