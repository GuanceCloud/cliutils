// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func getHttpCases(httpServer, httpsServer, proxyServer *httptest.Server) []struct {
	t         *HTTPTask
	fail      bool
	reasonCnt int
} {
	return []struct {
		t         *HTTPTask
		fail      bool
		reasonCnt int
	}{
		{
			fail:      false,
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_no_resp",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_no_resp", httpsServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					Certificate: &HTTPOptCertificate{
						IgnoreServerCertificateError: true,
						PrivateKey:                   string(tlsData["key"]),
						Certificate:                  string(tlsData["crt"]),
					},
					Secret: &HTTPSecret{
						NoSaveResponseBody: true,
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with certificate
		{
			fail:      false,
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_cert",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_with_cert", httpsServer.URL+"/_test_with_cert"),
				AdvanceOptions: &HTTPAdvanceOption{
					Certificate: &HTTPOptCertificate{
						IgnoreServerCertificateError: true,
						PrivateKey:                   string(tlsData["key"]),
						Certificate:                  string(tlsData["crt"]),
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},
		{
			fail:      true,
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_cert",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_with_cert", httpsServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					Certificate: &HTTPOptCertificate{
						IgnoreServerCertificateError: false, // bad certificate, expect fail
						PrivateKey:                   string(tlsData["key"]),
						Certificate:                  string(tlsData["crt"]),
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with proxy
		{
			fail:      false,
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_proxy",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "POST",
				URL:    fmt.Sprintf("%s/_test_with_proxy", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					Proxy: &HTTPOptProxy{
						URL:     proxyServer.URL,
						Headers: map[string]string{"X-proxy-header": "proxy-foo"},
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with body
		{
			fail:      true,
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_body",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_with_body", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestBody: &HTTPOptBody{
						BodyType: "application/unknown", // XXX: invalid body type
						Body:     `{"key": "value"}`,
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_body",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "POST",
				URL:    fmt.Sprintf("%s/_test_with_body", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestBody: &HTTPOptBody{
						BodyType: "None", // "application/json",
						Body:     `{"key": "value"}`,
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with headers
		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_headers",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				URL: fmt.Sprintf("%s/_test_with_headers", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestOptions: &HTTPOptRequest{
						Headers: map[string]string{
							"X-Header-1": "foo",
							"X-Header-2": "bar",
						},
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with auth
		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_basic_auth",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_with_basic_auth", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestOptions: &HTTPOptRequest{
						Auth: &HTTPOptAuth{
							Username: "foo",
							Password: "bar",
						},
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with cookie
		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_with_cookie",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_with_cookie", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestOptions: &HTTPOptRequest{
						Cookies: (&http.Cookie{
							Name:   "_test_with_cookie",
							Value:  "foo-bar",
							MaxAge: 0,
							Secure: true,
						}).String(),
					},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial for redirect
		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_redirect",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_redirect", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestOptions: &HTTPOptRequest{FollowRedirect: true},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"}, // allow redirect, should be 200
						},
					},
				},
			},
		},

		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_redirect_disabled",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_redirect", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestOptions: &HTTPOptRequest{FollowRedirect: false},
				},

				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "302"}, // disabled redirect, should be 302
						},
					},
				},
			},
		},

		{
			reasonCnt: 0,
			fail:      true,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_http_timeout_failed",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_http_timeout", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestTimeout: "1ms",
				},
				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		{
			reasonCnt: 0,
			fail:      false,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_http_timeout_ok",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_http_timeout", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestTimeout: "1s",
				},
				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
					},
				},
			},
		},

		// test dial with response time checking
		{
			reasonCnt: 1,
			fail:      true,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dialt_"),
					Name:       "_test_resp_time_less_10ms",
					Frequency:  "1s",
					Region:     "hangzhou",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_resp_time_less_10ms", httpServer.URL),
				SuccessWhen: []*HTTPSuccess{
					{ResponseTime: "10ms"},
				},
			},
		},

		// test dial with response headers
		{
			reasonCnt: 2,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_header_checking",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "GET",
				URL:    fmt.Sprintf("%s/_test_resp_time_less_10ms", httpServer.URL),
				SuccessWhen: []*HTTPSuccess{
					{
						Header: map[string][]*SuccessOption{
							"Cache-Control": {
								{MatchRegex: `max-ag=\d`}, // expect fail: max-age
							},
							"Server": {
								{Is: `Apache`}, // expect fail
							},

							"Date": {
								{Contains: "GMT"}, // ok: Date always use GMT
							},
							"NotExistHeader1": {
								{NotMatchRegex: `.+`}, // ok
							},
							"NotExistHeader2": {
								{IsNot: `abc`}, // ok
							},
							"NotExistHeader3": {
								{NotContains: `def`}, // ok
							},
						},
					},
				},
			},
		},

		// test multipart/form-data
		{
			reasonCnt: 0,
			t: &HTTPTask{
				Task: &Task{
					ExternalID: cliutils.XID("dtst_"),
					Name:       "_test_multipart_form_data",
					Region:     "hangzhou",
					Frequency:  "1s",
				},
				Method: "POST",
				URL:    fmt.Sprintf("%s/_test_multipart_form_data", httpServer.URL),
				AdvanceOptions: &HTTPAdvanceOption{
					RequestBody: &HTTPOptBody{
						Form: map[string]string{
							"foo": "bar",
						},
						BodyType: "multipart/form-data",
						Files: []HTTPOptBodyFile{
							{
								Name:             "foo.txt",
								OriginalFileName: "foo.txt",
								Content:          base64.StdEncoding.EncodeToString([]byte("foo.content")),
								Type:             "text/plain",
								Encoding:         "base64",
								Size:             3,
							},
						},
					},
				},
				SuccessWhen: []*HTTPSuccess{
					{
						StatusCode: []*SuccessOption{
							{Is: "200"},
						},
						Header: map[string][]*SuccessOption{
							"foo": {
								{Is: "bar"}, // expect fail: max-age
							},
							"foo.txt": {
								{Is: "foo.content"},
							},
						},
					},
				},
			},
		},
	}
}

func TestDialHTTP(t *testing.T) {
	createServer := func(proxyServer *httptest.Server, isHttps bool) *httptest.Server {
		gin.SetMode(gin.ReleaseMode)

		r := gin.New()
		gin.DisableConsoleColor()
		r.Use(gin.Recovery())

		addTestingRoutes(t, r, proxyServer, isHttps)

		if isHttps {
			return httptest.NewTLSServer(r)
		} else {
			return httptest.NewServer(r)
		}
	}

	proxyServer := proxyServer(t)
	httpServer := createServer(proxyServer, false) // http server
	httpsServer := createServer(proxyServer, true) // https server

	defer httpServer.Close()
	defer httpsServer.Close()
	defer proxyServer.Close()

	httpCases := getHttpCases(httpServer, httpsServer, proxyServer)

	for _, c := range httpCases {
		c.t.SetChild(c.t)
		if err := c.t.Init(); err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		if err := c.t.Run(); err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		ts, fs := c.t.GetResults()

		t.Logf("ts: %+#v \n fs: %+#v \n ", ts, fs)

		reasons, _ := c.t.CheckResult()
		if len(reasons) != c.reasonCnt {
			t.Errorf("case %s expect %d reasons, but got %d reasons:\n\t%s",
				c.t.Name, c.reasonCnt, len(reasons), strings.Join(reasons, "\n\t"))
		} else if len(reasons) > 0 {
			t.Logf("case %s reasons:\n\t%s",
				c.t.Name, strings.Join(reasons, "\n\t"))
		}

		if c.reasonCnt > 0 || c.fail {
			assert.Equal(t, "FAIL", ts["status"])
		} else {
			assert.Equal(t, "OK", ts["status"])
		}
	}
}

func proxyServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Logf("proxied request coming")
		for k := range req.Header {
			t.Logf("proxied header: %s: %s", k, req.Header.Get(k))
		}

		fmt.Fprintf(w, "ok")
	}))
}

func proxyHandler(t *testing.T, target string) gin.HandlerFunc {
	t.Helper()
	remote, err := url.Parse(target)
	if err != nil {
		t.Error(err)
		return nil
	}

	return func(c *gin.Context) {
		director := func(_ *http.Request) {
			req := c.Request

			req.URL.Scheme = remote.Scheme
			req.URL.Host = remote.Host
			req.URL.RawQuery = remote.RawQuery

			req.Header["X-proxy-header"] = []string{c.Request.Header.Get("X-proxy-header")}
			delete(req.Header, "X-proxy-header")
		}
		proxy := &httputil.ReverseProxy{Director: director}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

var tlsData = map[string][]byte{
	"csr": []byte(`-----BEGIN CERTIFICATE REQUEST-----
MIICtzCCAZ8CAQAwcjELMAkGA1UEBhMCQ04xDDAKBgNVBAgMA2ZvbzEMMAoGA1UE
BwwDYmFyMQ0wCwYDVQQKDARmb28xMQ0wCwYDVQQLDARiYXIxMQ0wCwYDVQQDDARm
b28yMRowGAYJKoZIhvcNAQkBFgtmb29AYmFyLmNvbTCCASIwDQYJKoZIhvcNAQEB
BQADggEPADCCAQoCggEBANEQvuwHLDTsu+QuIEXc4R8aTSFTgFl0CPz3GzAhZnYt
/MgZ66iu6W7FplTiqIPoSgTqccCcWPlOgaad0BfkmbuYaoo9SiF5/ewip6QXfpBQ
Va34Q92E3EfBv5vyuCgMyNbjXb+hHbRvYmgOUeL0J9jRMJ1l981pgYgjGdzNFJTz
S2FDVoaoexOkoOXblAI9tqbBi+9+Sbu7Q/DgeudeF07VmI+cZzZX73Oo2EzwHHXn
FI5OAbRBjKsQyU9O6TgarA/5n0hAH2bcHkoCxq4iVgHuZjK2xV8hJU8b4jGevDVE
TCwE07LJJfP2RnlYe7nBqNlNXApMSllUgX4c8RhUuQ0CAwEAAaAAMA0GCSqGSIb3
DQEBCwUAA4IBAQC4Z2ELAmnrPSoghuEyKuM2GsvRqOIUHKKwM/lCWxOE/o/pQDTY
OcC+2BwSimChoBd1TY3vM03TYxzY5jlHqfwLAFqJv51DFlTasHUhlo8+7IVR+6TE
WH9latBruNVSDZ5/qL1dfbLoBw6yyQi4kYdSg1T5CBtGVCe3iBC42NmXHqp5/XXB
dQAILNu1lzVi4dM6FbHcr6FTSZBIyYrHTYLPIj4aUQ/p5iO1jYvfM8DiXR0OWfzw
VFjOt6N0pYsfLgeOHA8v6NZMQ+N59Ne0Dl7Pg7bK56qP+l0R2hY0smXH/IPrGaFF
Qf01BfPoTUfoyV195ZF8BpeVtT1HBs3of/+6
-----END CERTIFICATE REQUEST-----`),
	"crt": []byte(`-----BEGIN CERTIFICATE-----
MIIDYDCCAkgCCQCgwJeKz0Yl5jANBgkqhkiG9w0BAQUFADByMQswCQYDVQQGEwJD
TjEMMAoGA1UECAwDZm9vMQwwCgYDVQQHDANiYXIxDTALBgNVBAoMBGZvbzExDTAL
BgNVBAsMBGJhcjExDTALBgNVBAMMBGZvbzIxGjAYBgkqhkiG9w0BCQEWC2Zvb0Bi
YXIuY29tMB4XDTIxMDIyMjA0MDAwNloXDTIyMDIyMjA0MDAwNlowcjELMAkGA1UE
BhMCQ04xDDAKBgNVBAgMA2ZvbzEMMAoGA1UEBwwDYmFyMQ0wCwYDVQQKDARmb28x
MQ0wCwYDVQQLDARiYXIxMQ0wCwYDVQQDDARmb28yMRowGAYJKoZIhvcNAQkBFgtm
b29AYmFyLmNvbTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBANEQvuwH
LDTsu+QuIEXc4R8aTSFTgFl0CPz3GzAhZnYt/MgZ66iu6W7FplTiqIPoSgTqccCc
WPlOgaad0BfkmbuYaoo9SiF5/ewip6QXfpBQVa34Q92E3EfBv5vyuCgMyNbjXb+h
HbRvYmgOUeL0J9jRMJ1l981pgYgjGdzNFJTzS2FDVoaoexOkoOXblAI9tqbBi+9+
Sbu7Q/DgeudeF07VmI+cZzZX73Oo2EzwHHXnFI5OAbRBjKsQyU9O6TgarA/5n0hA
H2bcHkoCxq4iVgHuZjK2xV8hJU8b4jGevDVETCwE07LJJfP2RnlYe7nBqNlNXApM
SllUgX4c8RhUuQ0CAwEAATANBgkqhkiG9w0BAQUFAAOCAQEAn8TzH9LvNyhH+cqa
gRc8Gqj0ccPf1LkW9dIlTlk31HBHzfKI7xhul23PimMuz6hg3YCAttXhKXrVoiIJ
1rQUngGr0e2CkesxfeaMxDPPRCRiLPRLzsryjvJI/eS2rmxtmUyC0X5aR+/2F8Ha
p2JXig4KUhYwMmttnd/Qbjmc0C397zKudBxkIoxprIN/gVhRBJJRqxN8bgeL8JsH
2HfsA/SzFDUOYPQhw0EnyLukRuQi0C3soKL4fIUGqonJPQ0TIceJRMGrtIj0h7j+
oNbJXTP7ABRYVmFRYViczu86MWsbHkif4bWqhPJeC0K+cp1UuwykJ+4XzM5WDR/+
InEHyg==
-----END CERTIFICATE-----`),

	"key": []byte(`-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDREL7sByw07Lvk
LiBF3OEfGk0hU4BZdAj89xswIWZ2LfzIGeuoruluxaZU4qiD6EoE6nHAnFj5ToGm
ndAX5Jm7mGqKPUohef3sIqekF36QUFWt+EPdhNxHwb+b8rgoDMjW412/oR20b2Jo
DlHi9CfY0TCdZffNaYGIIxnczRSU80thQ1aGqHsTpKDl25QCPbamwYvvfkm7u0Pw
4HrnXhdO1ZiPnGc2V+9zqNhM8Bx15xSOTgG0QYyrEMlPTuk4GqwP+Z9IQB9m3B5K
AsauIlYB7mYytsVfISVPG+Ixnrw1REwsBNOyySXz9kZ5WHu5wajZTVwKTEpZVIF+
HPEYVLkNAgMBAAECggEAJp/9ZgX9ONnz7LhI5h9kyCZH0bxnnh89+d59e2rwTy03
4pBHZabLIdgKXuxxTc2Av1/BHGDGX2kNswa9B20IqgwCwv+Hzp+HNjVA26QrkeYF
rlqLz0VYnTlCeUFinKOgB3OCQoE1x7w8ZhUfM9r/8aLUZIAORDkV4Vz6zjxlbQ8g
JxHrZ5eZexTzSVylVFZda3AgtqMr1N6ZzMejtYqttGGDDmh372QgykvxhmEIeHAf
g1bW86oOedxxfZ0003/F9He6qvdWmAKfbQczCNKBPHgGpdcuTTBsj/ieB/31AZG9
R1CUopzAklrUXzv1SBxw/5mJdOcmTUH4Hpdl4vXh0QKBgQD99FiKIKRxWZiHcbV4
X2wl0AZsMUbUT+BVKRbdfYk0pTstSKaQMpEB2ojvVqW8HVN83+jJxWUxxGWnT0Mn
wfw9lavhNS14klj+rJw6zI4m2lcI8t+P9HxTMDfBU+LiMnlUFK44u7Mx6Vr/dm9p
53o0aGapLOQfwps+UdJ86ZCAKwKBgQDSv9az1zHE1AtJx7UlreduzXrYjzJqrgYX
ufjLu+aTsSWNXIlIxG5gkKbkF6R4VVmpXkF7B8nJ3IrsrRuwMZpMjyhLl2LLCnGL
XgAgz/SNjxS4Clo1PVcP2ZoANVnPs/+DRlI1aTqXHZA5sJ1d2a9e385ndZ+/Qg+q
giRNOsfXpwKBgQC2dwnmtO1yQ93D839frbAWuxDiS8WIZpvYlF1JZxleKhoKv1ht
4uctXcdlr+wE7U0/O+IWly3ORD6Fp/2oY0jJNvD4Ly0spHotAfh+htrcL6S5WUgo
NpHdc5eb4JnzzDBAqVtEiBiIlBI92urSPO8hGKIqi4adC0Zf0IpcFbUtYQKBgF24
Iepn0CIPidWNkejnpPuJNRAI3grCyMLUWOeA79DN/j0W4ZYShGM88HqOaP16Nx0y
ZTwpAntaMA2ADcgUxuE06F51O+G/Cy9G5hexYrdw4W3WbLcwR/8sbWeaUg4jpYTj
SLunx/5bjz+YYuLRY0N1k3w+uoN7BSx2I16UvToRAoGAEFhhsGTxXLeNOMDU1jhJ
cbbypRkGjSoxUbn7apEMwdpeDPQwWwkwi634rjVcTIQuO/8HMbjZi2AZcM5TWNY0
HHrpiTXtbrUfbKX2TEk3DSevJ9EZEuewxALtsaRQgX4WyHlxpYDXNSjag04Nn+/x
9WKHZvRf3lbLY7GAR/emacU=
-----END PRIVATE KEY-----`),
}

func addTestingRoutes(t *testing.T, r *gin.Engine, proxyServer *httptest.Server, isHttps bool) {
	t.Helper()
	r.GET("/_test_resp_time_less_10ms", func(c *gin.Context) {
		time.Sleep(time.Millisecond * 11)
		c.Data(http.StatusOK, ``, nil)
	})

	r.GET("/_test_header_checking", func(c *gin.Context) {
		c.DataFromReader(http.StatusOK, 0, "", bytes.NewBufferString(""),
			map[string]string{
				"Cache-Control": "max-age=1024",
				"Server":        "dialtesting-server",
			})
	})

	r.GET("/_test_redirect", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/_redirect_to_me")
	})
	r.GET("/_redirect_to_me", func(c *gin.Context) {
		t.Log("redirect ok")
		c.Data(http.StatusOK, ``, nil)
	})

	r.GET("/_test_with_cookie", func(c *gin.Context) {
		cookies := c.Request.Cookies()
		for _, c := range cookies {
			t.Logf("%s", c.String())
		}

		c.Data(http.StatusOK, ``, nil)
	})

	r.GET("/_test_with_basic_auth", func(c *gin.Context) {
		user, pwd, ok := c.Request.BasicAuth()
		if !ok {
			t.Errorf("basic auth failed")
		} else {
			t.Logf("user: %s, password: %s", user, pwd)
		}

		c.Data(http.StatusOK, ``, nil)
	})

	r.GET("/_test_no_resp", func(c *gin.Context) {
		c.Data(http.StatusOK, string(tlsData["key"]), nil)
	})

	r.GET("/_test_with_headers", func(c *gin.Context) {
		for k := range c.Request.Header {
			t.Logf("%s: %s", k, c.Request.Header.Get(k))
		}

		c.Data(http.StatusOK, ``, nil)
	})

	r.GET("/_test_http_timeout", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.Data(http.StatusOK, "", nil)
	})

	r.POST("/_test_with_body", func(c *gin.Context) {
		defer c.Request.Body.Close() //nolint:errcheck
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			t.Error(err)
		}

		t.Logf("body: %s", string(body))

		c.Data(http.StatusOK, ``, nil)
	})

	r.POST("_test_multipart_form_data", func(c *gin.Context) {
		defer c.Request.Body.Close() //nolint:errcheck
		foo := c.PostForm("foo")
		form, err := c.MultipartForm()
		if err != nil {
			c.Error(err)
			return
		}
		c.Header("foo", foo)

		files := form.File["foo.txt"]
		if len(files) > 0 {
			f := files[0]
			openFile, err := f.Open()
			if err != nil {
				c.Error(err)
				return
			}
			defer openFile.Close()
			fileContent, err := io.ReadAll(openFile)
			if err != nil {
				c.Error(err)
				return
			}
			c.Header(f.Filename, string(fileContent))
		}
		c.Data(http.StatusOK, ``, nil)
	})

	r.GET("/_test_with_proxy",
		proxyHandler(t, fmt.Sprintf("%s/_test_with_proxy", proxyServer.URL)))

	if isHttps {
		r.GET("/_test_with_cert", func(c *gin.Context) {
			t.Logf("request tls: %+#v", c.Request.TLS)
			c.Data(http.StatusOK, ``, nil)
		})
	}
}

func TestPrepareTemplate(t *testing.T) {
	globalVars := map[string]Variable{
		"global_id": {
			Name:  "global",
			Value: "global",
		},
	}

	task := &HTTPTask{
		URL:        "http://localhost:8000/{{global}}",
		PostScript: "{{local}}",
		Task: &Task{
			ConfigVars: []*ConfigVar{
				{
					Name:  "local",
					Type:  "local",
					Value: "local",
				},
				{
					Name: "global",
					Type: "global",
					ID:   "global_id",
				},
			},
		},
	}
	task.SetChild(task)
	v, err := json.Marshal(task)

	assert.NoError(t, err)
	task.SetTaskJSONString(string(v))

	err = task.RenderTemplateAndInit(globalVars)
	assert.NoError(t, err)

	assert.Equal(t, "http://localhost:8000/global", task.URL)
	assert.Equal(t, "{{local}}", task.PostScript)
}

func TestPostScript(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"token": "tkn_123"}`))
	}))

	ct := &HTTPTask{
		Task: &Task{
			Frequency: "1s",
		},
		URL: server.URL,
		PostScript: `

	if response["status_code"] != 200 {
		result["is_failed"] = true
		result["error_message"] = "error"
	}	else {
		result["is_failed"] = false
	}

	body = load_json(response["body"])

	vars["token"] = body["token"]
	`,
	}

	task, err := NewTask("", ct)
	assert.NoError(t, err)

	vars := map[string]Variable{}
	assert.NoError(t, task.RenderTemplateAndInit(vars))
	assert.NoError(t, task.Run())

	tags, fields := task.GetResults()

	assert.Equal(t, "tkn_123", ct.postScriptResult.Vars["token"])
	assert.Equal(t, "OK", tags["status"])
	assert.EqualValues(t, 1, fields["success"])
}

func TestRenderTemplate(t *testing.T) {
	ct := &HTTPTask{
		URL: "http://localhost:8000/{{path}}",
		SuccessWhen: []*HTTPSuccess{
			{
				Body: []*SuccessOption{
					{
						Contains: "{{body}}",
					},
				},
				Header: map[string][]*SuccessOption{
					"Content-Type": {
						{
							Contains: "{{content_type}}",
						},
					},
				},
				ResponseTime: "{{response_time}}",
			},
		},
		AdvanceOptions: &HTTPAdvanceOption{
			RequestOptions: &HTTPOptRequest{
				Headers: map[string]string{
					"header": "{{header}}",
				},
				Cookies: "{{cookies}}",
				Auth: &HTTPOptAuth{
					Username: "{{username}}",
					Password: "{{password}}",
				},
			},
			RequestBody: &HTTPOptBody{
				Body: "{{request_body}}",
				Form: map[string]string{
					"{{form_key}}": "{{form_value}}",
				},
			},
		},
	}

	fm := template.FuncMap{
		"request_body": func() string {
			return "request_body"
		},
		"form_key": func() string {
			return "form_key"
		},
		"form_value": func() string {
			return "form_value"
		},
		"header": func() string {
			return "header"
		},
		"cookies": func() string {
			return "cookies"
		},
		"username": func() string {
			return "username"
		},
		"password": func() string {
			return "password"
		},
		"path": func() string {
			return "test"
		},
		"response_time": func() string {
			return "100ms"
		},
		"content_type": func() string {
			return "application/json"
		},
		"body": func() string {
			return "OK"
		},
	}

	task, err := NewTask("", ct)
	assert.NoError(t, err)

	ct, ok := task.(*HTTPTask)
	assert.True(t, ok)
	assert.NoError(t, ct.renderTemplate(fm))
	assert.Equal(t, "http://localhost:8000/test", ct.URL)

	// success when
	assert.Equal(t, "100ms", ct.SuccessWhen[0].ResponseTime)
	assert.Equal(t, "application/json", ct.SuccessWhen[0].Header["Content-Type"][0].Contains)
	assert.Equal(t, "OK", ct.SuccessWhen[0].Body[0].Contains)

	// advance options
	// request options
	assert.Equal(t, "header", ct.AdvanceOptions.RequestOptions.Headers["header"])
	assert.Equal(t, "cookies", ct.AdvanceOptions.RequestOptions.Cookies)
	assert.Equal(t, "username", ct.AdvanceOptions.RequestOptions.Auth.Username)
	assert.Equal(t, "password", ct.AdvanceOptions.RequestOptions.Auth.Password)

	// request body
	assert.Equal(t, "request_body", ct.AdvanceOptions.RequestBody.Body)
	assert.Equal(t, "form_value", ct.AdvanceOptions.RequestBody.Form["form_key"])
	assert.Equal(t, "", ct.AdvanceOptions.RequestBody.Form["{{form_key}}"])
}
