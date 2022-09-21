package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	tu "gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"
)

func TestBytesBody(t *testing.T) {
	errOK := NewNamespaceErr(nil, http.StatusOK, "")
	bytesBody := "this is bytes response body"

	router := gin.New()
	g := router.Group("")
	g.GET("/bytes-body", func(c *gin.Context) {
		c.Writer.Header().Set("Content-Type", "application/octet-stream")
		c.Writer.Header().Set("X-Latest-Time", fmt.Sprintf("%s", time.Now()))
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

	b, err := ioutil.ReadAll(resp.Body)
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
		c.Writer.Header().Set("X-Latest-Time", fmt.Sprintf("%s", time.Now()))
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
			for k, _ := range resp.Header {
				t.Logf("%s: %s", k, resp.Header.Get(k))
			}
			body, _ := ioutil.ReadAll(resp.Body)
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

	srv := http.Server{
		Addr:    ":8090",
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Log(err)
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
				body, err := ioutil.ReadAll(resp.Body)
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
