// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	tu "github.com/GuanceCloud/cliutils/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestBytesBody(t *testing.T) {
	errOK := NewNamespaceErr(nil, http.StatusOK, "")
	bytesBody := "this is bytes response body"

	router := gin.New()
	g := router.Group("")
	g.GET("/bytes-body", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/octet-stream")
		c.Writer.Header().Set("X-Latest-Time", time.Now().String())
		errOK.HttpBody(c, []byte(bytesBody))
	})

	ts := httptest.NewServer(router)

	defer ts.Close()

	time.Sleep(time.Second)

	resp, err := http.Get(fmt.Sprintf("%s%s", ts.URL, "/bytes-body"))
	if err != nil {
		t.Error(err)
	}

	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}

	for k, v := range resp.Header {
		t.Logf("%s: %v", k, v)
	}

	tu.Equals(t, bytesBody, string(b))
}

func TestNoSniff(t *testing.T) {
	errOK := NewNamespaceErr(nil, http.StatusOK, "")
	bytesBody := "this is bytes response body"

	router := gin.New()
	g := router.Group("")
	g.GET("/bytes-body", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/octet-stream")
		c.Writer.Header().Set("X-Latest-Time", time.Now().String())
		errOK.HttpBody(c, []byte(bytesBody))
	})

	g.GET("/obj-body", func(c *gin.Context) {
		errOK.HttpBody(c, map[string]string{})
	})

	g.GET("/err-body", func(c *gin.Context) {
		HttpErr(c, fmt.Errorf("mocked error"))
	})

	ts := httptest.NewServer(router)
	defer ts.Close()
	time.Sleep(time.Second)

	// --------------------

	for _, x := range []string{
		"/bytes-body", "/obj-body", "/err-body",
	} {
		t.Run(x, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("%s%s", ts.URL, x))
			if err != nil {
				t.Error(err)
			}
			assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
			for k := range resp.Header {
				t.Logf("%s: %s", k, resp.Header.Get(k))
			}
			body, _ := io.ReadAll(resp.Body)
			t.Logf("body: %s", body)
			resp.Body.Close()
		})
	}
}

func TestHTTPErr(t *testing.T) {
	errTest := NewNamespaceErr(errors.New("test error"), http.StatusForbidden, "testing")
	errOK := NewNamespaceErr(nil, http.StatusOK, "")

	DefaultNamespace = "testing2"
	errTest2 := NewErr(errors.New("test error2"), http.StatusForbidden)

	router := gin.New()
	g := router.Group("")

	okbody := map[string]interface{}{
		"data1": 1,
		"data2": "abc",
	}

	g.GET("/err", func(c *gin.Context) { HttpErr(c, errTest) })
	g.GET("/err2", func(c *gin.Context) { HttpErr(c, errTest2) })
	g.GET("/err3", func(c *gin.Context) { HttpErr(c, fmt.Errorf("500 error")) })
	g.GET("/errf", func(c *gin.Context) { HttpErrf(c, errTest, "%s: %s", "this is a test error", "ignore me") })
	g.GET("/ok", func(c *gin.Context) { errOK.WriteBody(c, okbody) })
	g.GET("/ok2", func(c *gin.Context) { errOK.HttpBody(c, okbody) })
	g.GET("/oknilbody", func(c *gin.Context) { errOK.HttpBody(c, nil) })
	g.GET("/errmsg", func(c *gin.Context) { err := Error(errTest, "this is a error with specific message"); HttpErr(c, err) })
	g.GET("/errfmsg", func(c *gin.Context) {
		err := Errorf(errTest, "%s: %v", "this is a message with fmt", map[string]int{"abc": 123})
		HttpErr(c, err)
	})
	g.GET("/errfmsg-with-nil-args", func(c *gin.Context) {
		err := Errorf(ErrTooManyRequest, "Errorf without args")
		HttpErr(c, err)
	})

	srv := http.Server{
		Addr:    ":8090",
		Handler: router,
	}

	go func() {
		if e := srv.ListenAndServe(); e != nil && errors.Is(e, http.ErrServerClosed) {
			t.Log(e)
		}
	}()

	time.Sleep(time.Second)
	defer srv.Close()

	cases := []struct {
		u      string
		expect string
	}{
		{
			u:      "http://localhost:8090/errmsg",
			expect: `{"error_code":"testing.testError","message":"this is a error with specific message"}`,
		},

		{
			u:      "http://localhost:8090/errfmsg",
			expect: `{"error_code":"testing.testError","message":"this is a message with fmt: map[abc:123]"}`,
		},

		{
			u: "http://localhost:8090/err",
			expect: func() string {
				j, err := json.Marshal(errTest)
				if err != nil {
					t.Fatal(err)
				}
				return string(j)
			}(),
		},
		{
			u: "http://localhost:8090/err2",
			expect: func() string {
				j, err := json.Marshal(errTest2)
				if err != nil {
					t.Fatal(err)
				}
				return string(j)
			}(),
		},
		{
			u:      "http://localhost:8090/err3",
			expect: `{"error_code":"testing2.500Error"}`,
		},
		{
			u:      "http://localhost:8090/errf",
			expect: `{"error_code":"testing.testError","message":"this is a test error: ignore me"}`,
		},
		{
			u: "http://localhost:8090/ok",
			expect: func() string {
				j, err := json.Marshal(okbody)
				if err != nil {
					t.Fatal(err)
				}
				return string(j)
			}(),
		},

		{
			u: "http://localhost:8090/ok2",
			expect: func() string {
				x := struct {
					Content interface{} `json:"content"`
				}{
					Content: okbody,
				}

				j, err := json.Marshal(x)
				if err != nil {
					t.Fatal(err)
				}
				return string(j)
			}(),
		},

		{
			u:      "http://localhost:8090/oknilbody",
			expect: "",
		},
		{
			u: "http://localhost:8090/errfmsg-with-nil-args",
			expect: func() string {
				msg := `{"error_code":"reachMaxAPIRateLimit","message":"Errorf without args"}`
				var x interface{}
				if err := json.Unmarshal([]byte(msg), &x); err != nil {
					t.Fatal(err)
				}
				j, err := json.Marshal(x)
				if err != nil {
					t.Fatal(err)
				}
				return string(j)
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.u, func(t *testing.T) {
			resp, err := http.Get(tc.u)
			if err != nil {
				t.Logf("get error: %s, ignored", err)
				return
			}

			for k, v := range resp.Header {
				t.Logf("%s: %v", k, v)
			}

			if resp.Body != nil {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Error(err)
					return
				}

				tu.Equals(t, tc.expect, string(body))
				resp.Body.Close()
			} else {
				t.Error("body should not be nil")
			}
		})
	}
}

func TestErrorf(t *testing.T) {
	var nilArgs []interface{} = nil
	format := "sprintf with nil args"
	str := fmt.Sprintf(format, nilArgs...)
	fmt.Println(str)

	errWithEmptyFmt := Errorf(NewErr(errors.New("bad gateway"), http.StatusBadGateway), "")
	assert.Nil(t, errWithEmptyFmt.Args)

	errWithoutArgs := Errorf(ErrUnexpectedInternalServerError, "Errorf without args")
	assert.Nil(t, errWithoutArgs.Args)

	errWithArgs := Errorf(ErrTooManyRequest, "Errorf with args: %s", "###ERROR###")
	assert.NotNil(t, errWithArgs.Args)

	g := gin.New()

	type testCase struct {
		name        string
		url         string
		err         *MsgError
		expectedMsg string
	}

	cases := []testCase{
		{
			name:        "Errorf With Empty fmt",
			url:         "/errorf-with-empty-fmt",
			err:         errWithEmptyFmt,
			expectedMsg: "",
		},
		{
			name:        "Errorf without args",
			url:         "/errorf-without-args",
			err:         errWithoutArgs,
			expectedMsg: "Errorf without args",
		},
		{
			name:        "Errorf with args",
			url:         "/errorf-with-args",
			err:         errWithArgs,
			expectedMsg: "Errorf with args: ###ERROR###",
		},
	}

	for i := range cases {
		tc := cases[i]
		g.GET(tc.url, func(c *gin.Context) {
			HttpErr(c, tc.err)
		})
	}

	type response struct {
		ErrCode string `json:"error_code"`
		Message string `json:"message"`
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			g.ServeHTTP(resp, req)

			assert.Equal(t, resp.Result().StatusCode, tc.err.HttpCode)

			var rp response
			decoder := json.NewDecoder(resp.Body)
			err := decoder.Decode(&rp)
			assert.NoError(t, err)

			assert.NotEmpty(t, rp.ErrCode)
			assert.Equal(t, tc.expectedMsg, rp.Message)
		})
	}
}
