// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// Global certificate and private key content, shared by all tests
const (
	realCert = `-----BEGIN CERTIFICATE-----
MIIDCTCCAfGgAwIBAgIUVoZx6x36Br9gQJehrYSQ0YGlgwgwDQYJKoZIhvcNAQEL
BQAwFDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTI2MDMzMDA3MjgyNloXDTI3MDMz
MDA3MjgyNlowFDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEF
AAOCAQ8AMIIBCgKCAQEA21ZdW6O3xmWiJ+0JcCYZhwx05X9ZzL/OuHs3DMye8nXO
JEMsJx6EcQzpAuVarhmdCZPFT3GI3NpN0yhcGH0TLnInurq6T3FIBUukaKvbCQGe
MqLv72tR+GdVsaGzDwxPbqzrwtsuOohT+XEmghOGSA7AlGKKLYVg4JRimFGj7h+0
Tn24pL1Xn8gdqC8yQrY+KrQalNwhMmcdbTF3jhxDrMJF7ESbSi3xKS1y6QOGVYx/
w7uLXwqp8BAWVVKMogGEtB/ZnCG4wULt2h9ufrLxM2+jsREdnooDhcNEFiz1ChAE
6Y0ttdxKi9KlNunzR42CeXMo+BLv48kYyCP+6UQiHwIDAQABo1MwUTAdBgNVHQ4E
FgQUUZgmhhMfiqr/Th58lUnrZnNARIkwHwYDVR0jBBgwFoAUUZgmhhMfiqr/Th58
lUnrZnNARIkwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAwghT
q1ngiTK+fdyfvUgoqdpnJGK+3PzfAdfU4FhYRjaKx+04Ag7bGYyMbOoGjINFXupp
MsmBB+CbktRrHZTOPeh/3T1TVlcuiJvvymvCkLY7oK1/jHW9Oc87BllK3OuDR0p4
cIwDrEQRPE+GG9phCoSnZVZBxg8H24oPTRd5/q+H7uieDJ/oXizm2j3Um5MfG0am
o0I/dUn+ZHNN/ePRvF+IcvlVYAroYBJ5SMphVJuBm6OWnZLtRI0QGSdWYVNjw6RM
YIpYqk5UVaAuaMmYNf+G6c++ZutwXrKRJM5Dt7ltVEUVcCs40yFHvk6/hcpOuJ41
KR+HJ2VyWIo3iQAf2g==
-----END CERTIFICATE-----`

	realKey = `-----BEGIN PRIVATE KEY-----
MIIEvAIBADANBgkqhkiG9w0BAQEFAASCBKYwggSiAgEAAoIBAQDbVl1bo7fGZaIn
7QlwJhmHDHTlf1nMv864ezcMzJ7ydc4kQywnHoRxDOkC5VquGZ0Jk8VPcYjc2k3T
KFwYfRMucie6urpPcUgFS6Roq9sJAZ4you/va1H4Z1WxobMPDE9urOvC2y46iFP5
cSaCE4ZIDsCUYoothWDglGKYUaPuH7ROfbikvVefyB2oLzJCtj4qtBqU3CEyZx1t
MXeOHEOswkXsRJtKLfEpLXLpA4ZVjH/Du4tfCqnwEBZVUoyiAYS0H9mcIbjBQu3a
H25+svEzb6OxER2eigOFw0QWLPUKEATpjS213EqL0qU26fNHjYJ5cyj4Eu/jyRjI
I/7pRCIfAgMBAAECggEAFE55IzhTldsn+aFQ6+CjIWUuV2cEddfWOC80KAuztYfh
l2rephqFsX+7/OgmDpNRfib/r/y3apcNeHy2lg/SXEz2T6vk/uDihZb8uDIc+8b6
Ef8Szqw1cRWEEgeB7+U4X2tEAozPSV0AxUnMAmPzXS18d+BtoYxFLVWfkTGRx0Rd
OdjWW4D7QlaZIYrR0W2uJrUhnneuns15TyaewVh24l6zhAZwe4rEyEgwBP3yb0aQ
YaPIM2m3zDbfcR1kOvCiambWV6M4Zg2dyMbCs9JF7i+DndLC3o/jWsRwBrp8Y6aO
gIzwNu60y8/sdfkbJ/gVxoGUTagQBNWUfop6Jk4xIQKBgQD5pR4xnbcyNipbcn4s
ULcP4b/B6c2ClXkdSDz/VMAvYZKcmm9sWcQEtoHfXayr/cZ/LblYr+8Gy/Avj7s5
ESmQ0eN5g7iI6l1NSRuV7GSvsdTTTao72M5keCc6EoM0ruN1gqo2wUOC0qP98wJg
M573jxlNVhsR6jlIyvvVTw/SIQKBgQDg670O+g6IAHVso29bdUGWcNBijtwySSHk
WwxP49PErzLfcUSHGZ/v0jcdGo+7Fx0/rO7cE6JDWNYZ3ev9wR3vKB3BOPAzI9Ve
eiqJdUHs2YkEcGGOvvin0pIRxUgdNwn0zk08Mq+Ieg3A0R5QLeQMMBm778E9/LNu
A5I/C4nsPwKBgA9RHzIiIBxXkG+97ZngdermifJm2vIZI641QXDVDVma3fj3zMBU
HZ/AZuWChNakomopLwcO/FZpatowMmeE8wzso81P1KGp54GXa7beIytYeNtiF4DG
g5tMd/OrMRupY1FRbAoh/3dmXyN0pn+qiyVbRU0mbFDEEzGxKpRi3nChAoGAVpzQ
9/R5JgwvK7+gATMdJ6aXyGxFBSQ+ZeZdzmHoSaRBzeObRP8sJLjpuk5hLOWQwNWC
QcNZx99syxc3akc0lMT4+FBJxxe0caZPvREnauK2LbxtBQArVszyrN8wjveD4P6U
pWrLR53gr/CXYL7bQ4o+Tq3b11f5bJL5fUefPBcCgYArUuZLfSve1N3ciFF2YNg8
ZuhYnsx9YVhUsCCoXzMbpOH2nDu+4P72YlrnMvpIEI5S3aQL3kkWlzUWhjhDrjJV
KOogbXsH3Bs5j5xdT625lC1zwtKR2dMHrIGiyMaMu4SrqodkzMGErnMP+fWyxSto
G0M1tbDJHxArOoW/JFApvg==
-----END PRIVATE KEY-----`
)

type testHealthServer struct {
	grpc_health_v1.UnimplementedHealthServer
}

func (s *testHealthServer) Check(context.Context, *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	return &grpc_health_v1.HealthCheckResponse{
		Status: grpc_health_v1.HealthCheckResponse_SERVING,
	}, nil
}

// TestHTTPSWithRealCertificate tests SSL certificate validity extraction with a real self-signed certificate for HTTP
func TestHTTPSWithRealCertificate(t *testing.T) {
	// Load the certificate and key
	cert, err := tls.X509KeyPair([]byte(realCert), []byte(realKey))
	if err != nil {
		t.Fatalf("Failed to load certificate and key: %v", err)
	}

	// Parse the certificate to get actual validity dates
	certBlock, _ := pem.Decode([]byte(realCert))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		t.Fatalf("Failed to decode certificate")
	}
	parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	expectedNotAfter := parsedCert.NotAfter.UnixMicro()

	// Create a TLS server with the self-signed certificate
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "Hello, World!")
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	server.StartTLS()
	defer server.Close()

	// Create an HTTP task
	task := &HTTPTask{
		Task: &Task{
			Name: "Test HTTP with real certificate",
		},
		URL:              server.URL,
		Method:           "GET",
		SuccessWhenLogic: "and",
		SuccessWhen: []*HTTPSuccess{
			{
				StatusCode: []*SuccessOption{
					{
						Is: "200",
					},
				},
			},
		},
		AdvanceOptions: &HTTPAdvanceOption{
			Certificate: &HTTPOptCertificate{
				IgnoreServerCertificateError: true, // Skip verification for self-signed cert
			},
		},
	}

	// Set the child field to point to the task itself
	task.Task.child = task

	// Clear the task to reset certificate fields
	task.clear()

	// Initialize and run the task
	if err := task.init(); err != nil {
		t.Fatalf("Failed to init task: %v", err)
	}

	if err := task.run(); err != nil {
		t.Fatalf("Failed to run task: %v", err)
	}

	// Get results and check if SSL certificate validity information is included
	_, fields := task.GetResults()
	if _, ok := fields["ssl_cert_not_after"]; !ok {
		t.Error("ssl_cert_not_after not found in results")
	}
	if _, ok := fields["ssl_cert_expires_in_days"]; !ok {
		t.Error("ssl_cert_expires_in_days not found in results")
	}

	// Verify the extracted values are valid timestamps
	var notAfter int64
	var expiresIn int64
	hasValidValues := false
	if na, ok := fields["ssl_cert_not_after"].(int64); ok {
		notAfter = na
		if notAfter <= 0 {
			t.Error("Invalid SSL certificate validity timestamp")
		}

		if notAfter != expectedNotAfter {
			t.Errorf("ssl_cert_not_after mismatch: expected %d, got %d", expectedNotAfter, notAfter)
		}
	} else {
		t.Error("ssl_cert_not_after is not an int64")
	}
	if v, ok := fields["ssl_cert_expires_in_days"].(int64); ok {
		expiresIn = v
		hasValidValues = true
		if expiresIn <= 0 {
			t.Error("Invalid SSL certificate remaining validity")
		}
	} else {
		t.Error("ssl_cert_expires_in_days is not an int64")
	}

	if hasValidValues {
		t.Logf("HTTP real certificate expiry: notAfter=%d, expiresIn=%d", notAfter, expiresIn)
	}
}

// TestWebSocketWithRealCertificate tests SSL certificate validity extraction with a real self-signed certificate for WebSocket
func TestWebSocketWithRealCertificate(t *testing.T) {
	// Load the certificate and key
	cert, err := tls.X509KeyPair([]byte(realCert), []byte(realKey))
	if err != nil {
		t.Fatalf("Failed to load certificate and key: %v", err)
	}

	// Parse the certificate to get actual validity dates
	certBlock, _ := pem.Decode([]byte(realCert))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		t.Fatalf("Failed to decode certificate")
	}
	parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	expectedNotAfter := parsedCert.NotAfter.UnixMicro()

	// Create a WebSocket server with TLS
	upgrader := websocket.Upgrader{}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("WebSocket upgrade error: %v", err)
			return
		}
		defer c.Close()
		for {
			mt, message, err := c.ReadMessage()
			if err != nil {
				break
			}
			err = c.WriteMessage(mt, message)
			if err != nil {
				break
			}
		}
	}))
	server.TLS = &tls.Config{
		Certificates: []tls.Certificate{cert},
	}
	server.StartTLS()
	defer server.Close()

	// Create a WebSocket task
	task := &WebsocketTask{
		Task: &Task{
			Name: "Test WebSocket with real certificate",
		},
		URL:              server.URL, // httptest server URL will be https, we'll convert it to wss
		Message:          "Hello, WebSocket!",
		SuccessWhenLogic: "and",
		SuccessWhen: []*WebsocketSuccess{
			{
				ResponseMessage: []*SuccessOption{
					{
						Contains: "Hello, WebSocket!",
					},
				},
			},
		},
	}

	// Convert https URL to wss
	task.URL = strings.Replace(task.URL, "https://", "wss://", 1)

	// Set the child field to point to the task itself
	task.Task.child = task

	// Clear the task to reset certificate fields
	task.clear()

	// Initialize and run the task
	if err := task.init(); err != nil {
		t.Fatalf("Failed to init task: %v", err)
	}

	if err := task.run(); err != nil {
		t.Fatalf("Failed to run task: %v", err)
	}

	// Get results and check if SSL certificate validity information is included
	_, fields := task.GetResults()
	if _, ok := fields["ssl_cert_not_after"]; !ok {
		t.Error("ssl_cert_not_after not found in results")
	}
	if _, ok := fields["ssl_cert_expires_in_days"]; !ok {
		t.Error("ssl_cert_expires_in_days not found in results")
	}

	// Verify the extracted values are valid timestamps
	var notAfter int64
	var expiresIn int64
	hasValidValues := false
	if na, ok := fields["ssl_cert_not_after"].(int64); ok {
		notAfter = na
		if notAfter <= 0 {
			t.Error("Invalid SSL certificate validity timestamp")
		}

		if notAfter != expectedNotAfter {
			t.Errorf("ssl_cert_not_after mismatch: expected %d, got %d", expectedNotAfter, notAfter)
		}
	} else {
		t.Error("ssl_cert_not_after is not an int64")
	}
	if v, ok := fields["ssl_cert_expires_in_days"].(int64); ok {
		expiresIn = v
		hasValidValues = true
		if expiresIn <= 0 {
			t.Error("Invalid SSL certificate remaining validity")
		}
	} else {
		t.Error("ssl_cert_expires_in_days is not an int64")
	}

	if hasValidValues {
		t.Logf("WebSocket real certificate expiry: notAfter=%d, expiresIn=%d", notAfter, expiresIn)
	}
}

// TestGRPCWithRealCertificate tests SSL certificate validity extraction with a real self-signed certificate for gRPC
func TestGRPCWithRealCertificate(t *testing.T) {
	// Load the certificate and key
	cert, err := tls.X509KeyPair([]byte(realCert), []byte(realKey))
	if err != nil {
		t.Fatalf("Failed to load certificate and key: %v", err)
	}

	// Parse the certificate to get actual validity dates
	certBlock, _ := pem.Decode([]byte(realCert))
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		t.Fatalf("Failed to decode certificate")
	}
	parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}
	expectedNotAfter := parsedCert.NotAfter.UnixMicro()

	// Create a TLS gRPC server with the self-signed certificate
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer lis.Close()

	server := grpc.NewServer(grpc.Creds(credentials.NewServerTLSFromCert(&cert)))
	grpc_health_v1.RegisterHealthServer(server, &testHealthServer{})

	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	// Create a gRPC task
	task := &GRPCTask{
		Task: &Task{
			Name: "Test gRPC with real certificate",
		},
		Server:           lis.Addr().String(),
		SuccessWhenLogic: "and",
		SuccessWhen: []*GRPCSuccess{
			{
				Body: []*SuccessOption{
					{
						Contains: "SERVING",
					},
				},
			},
		},
		AdvanceOptions: &GRPCAdvanceOption{
			RequestOptions: &GRPCOptRequest{
				HealthCheck: &GRPCHealthCheckDiscovery{},
			},
			Certificate: &GRPCOptCertificate{
				IgnoreServerCertificateError: true, // Skip verification for self-signed cert
			},
		},
	}

	// Set the child field to point to the task itself
	task.Task.child = task

	// Clear the task to reset certificate fields
	task.clear()

	// Initialize and run the task
	if err := task.init(); err != nil {
		t.Fatalf("Failed to init task: %v", err)
	}

	if err := task.run(); err != nil {
		t.Fatalf("Failed to run task: %v", err)
	}

	// Get results and check if SSL certificate validity information is included
	_, fields := task.GetResults()
	if _, ok := fields["ssl_cert_not_after"]; !ok {
		t.Error("ssl_cert_not_after not found in results")
	}
	if _, ok := fields["ssl_cert_expires_in_days"]; !ok {
		t.Error("ssl_cert_expires_in_days not found in results")
	}

	// Verify the extracted values are valid timestamps
	var notAfter int64
	var expiresIn int64
	hasValidValues := false
	if na, ok := fields["ssl_cert_not_after"].(int64); ok {
		notAfter = na
		if notAfter <= 0 {
			t.Error("Invalid SSL certificate validity timestamp")
		}

		if notAfter != expectedNotAfter {
			t.Errorf("ssl_cert_not_after mismatch: expected %d, got %d", expectedNotAfter, notAfter)
		}
	} else {
		t.Error("ssl_cert_not_after is not an int64")
	}
	if v, ok := fields["ssl_cert_expires_in_days"].(int64); ok {
		expiresIn = v
		hasValidValues = true
		if expiresIn <= 0 {
			t.Error("Invalid SSL certificate remaining validity")
		}
	} else {
		t.Error("ssl_cert_expires_in_days is not an int64")
	}

	if hasValidValues {
		t.Logf("gRPC real certificate expiry: notAfter=%d, expiresIn=%d", notAfter, expiresIn)
	}
}
