// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (
	"github.com/GuanceCloud/cliutils/testutil"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func BenchmarkAllMiddlewares(b *testing.B) {
	cases := []struct {
		name string
		ms   []gin.HandlerFunc
	}{
		{
			name: "none",
			ms:   []gin.HandlerFunc{},
		},
		{
			name: "all",
			ms: []gin.HandlerFunc{
				CORSMiddleware, TraceIDMiddleware, RequestLoggerMiddleware,
			},
		},
		{
			name: "cors",
			ms: []gin.HandlerFunc{
				CORSMiddleware,
			},
		},
		{
			name: "trace-id",
			ms: []gin.HandlerFunc{
				TraceIDMiddleware,
			},
		},
		{
			name: "request-logger",
			ms: []gin.HandlerFunc{
				RequestLoggerMiddleware,
			},
		},
	}

	for _, bc := range cases {
		b.Run(bc.name, func(b *testing.B) {
			r := gin.New()

			for _, m := range bc.ms {
				r.Use(m)
			}

			r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
				Formatter: GinLogFormatter,
			}))

			v1 := r.Group("/v1")
			v1.GET("/get", func(c *gin.Context) { c.Data(400, "application/json", []byte(`{"error": "get-error"}`)) })

			srv := &http.Server{
				Addr:    `localhost:1234`,
				Handler: r,
			}

			go func() {
				if err := srv.ListenAndServe(); err != nil {
					b.Log(err)
				}
			}()

			time.Sleep(time.Second)

			for i := 0; i < b.N; i++ {
				resp, err := http.Get("http://localhost:1234/v1/get")
				if err != nil {
					b.Logf("get error: %s, ignored", err)
				}

				if resp.Body != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
			}
			srv.Close()
		})
	}
}

func TestCORSHeaders_Add(t *testing.T) {
	// Accept, Accept-Encoding, Accept-Language, Authorization, Cache-Control, Content-Language, Content-Length, Content-Type, Origin, X-Csrf-Token, X-Datakit-Uuid, X-Lua, X-Precision, X-Requested-With, X-Rp, X-Token, *
	defaultHeaders := defaultCORSHeader.String()

	h1 := defaultCORSHeader.Add("content-type  , X-PRECISION")
	testutil.Equals(t, defaultHeaders, h1)

	h2 := defaultCORSHeader.Add("  ")
	testutil.Equals(t, defaultHeaders, h2)

	h3 := defaultCORSHeader.Add("x-Foo ,cache-control , X-BAR")
	testutil.Equals(t, "X-Foo, X-Bar, "+defaultHeaders, h3)

	h4 := defaultCORSHeader.Add(" * ")
	testutil.Equals(t, defaultHeaders, h4)

	h5 := defaultCORSHeader.Add("x-forwarded-for ,x-real-ip , x-client-ip")
	testutil.Equals(t, "X-Forwarded-For, X-Real-Ip, X-Client-Ip, "+defaultHeaders, h5)

}

func TestMiddlewares(t *testing.T) {
	r := gin.New()

	r.Use(CORSMiddleware)
	r.Use(TraceIDMiddleware)
	r.Use(RequestLoggerMiddleware)
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		Formatter: GinLogFormatter,
	}))

	t.Setenv("MAX_REQUEST_BODY_LEN", "4")
	Init()

	v1 := r.Group("/v1")
	v1.GET("/get", func(c *gin.Context) { c.Data(400, "application/json", []byte(`{"error": "get-error"}`)) })
	v1.GET("/get500", func(c *gin.Context) { c.Data(500, "application/json", []byte(`{"error": "get-error"}`)) })
	v1.POST("/post", func(c *gin.Context) { c.Data(400, "application/json", []byte(`{"error": "post-error"}`)) })
	v1.GET("/getok", func(c *gin.Context) { c.Data(200, "application/json", []byte(`{"get": "ok"}`)) })
	v1.POST("/postok", func(c *gin.Context) { c.Data(200, "application/json", []byte(`{"post": "ok"}`)) })

	srv := &http.Server{
		Addr:    `localhost:1234`,
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			t.Log(err)
		}
	}()

	defer srv.Close()

	time.Sleep(time.Second)

	resp, err := http.Get("http://localhost:1234/v1/get")
	if err != nil {
		t.Logf("get error: %s, ignored", err)
	}

	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp, err = http.Get("http://localhost:1234/v1/get500")
	if err != nil {
		t.Logf("get error: %s, ignored", err)
	}

	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp, err = http.Post("http://localhost:1234/v1/post", "", nil)
	if err != nil {
		t.Logf("get error: %s, ignored", err)
	}

	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp, err = http.Get("http://localhost:1234/v1/getok")
	if err != nil {
		t.Logf("get error: %s, ignored", err)
	}

	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	resp, err = http.Post("http://localhost:1234/v1/postok", "", nil)
	if err != nil {
		t.Logf("get error: %s, ignored", err)
	}

	if resp.Body != nil {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}
