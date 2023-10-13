// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/GuanceCloud/cliutils/testutil"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

const testText = `观测云提供的系统全链路可观测解决方案，
可实现从底层基础设施到通用技术组件，
再到业务应用系统的全链路可观测，
将不可预知性变为确定已知性。
观测云提供快速实现系统可观测的解决方案，满足云、云原生、应用和业务上的监测需求。
通过自定义监测方案，实现实时可交互仪表板、高效观测基础设施、全链路应用性能可观测等功能，保障系统稳定性
观测云、全链路可观测、实时监测、自定义监测、云原生
观测云提供的系统全链路可观测解决方案，
可实现从底层基础设施到通用技术组件，
再到业务应用系统的全链路可观测，
将不可预知性变为确定已知性。
观测云提供快速实现系统可观测的解决方案，满足云、云原生、应用和业务上的监测需求。
通过自定义监测方案，实现实时可交互仪表板、高效观测基础设施、全链路应用性能可观测等功能，保障系统稳定性
观测云、全链路可观测、实时监测、自定义监测、云原生
观测云提供的系统全链路可观测解决方案，
可实现从底层基础设施到通用技术组件，
再到业务应用系统的全链路可观测，
将不可预知性变为确定已知性。
观测云提供快速实现系统可观测的解决方案，满足云、云原生、应用和业务上的监测需求。
通过自定义监测方案，实现实时可交互仪表板、高效观测基础设施、全链路应用性能可观测等功能，保障系统稳定性
观测云、全链路可观测、实时监测、自定义监测、云原生
`

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

func TestNewHashReader(t *testing.T) {
	src := []byte(testText)

	r := NewReaderWithHash(bytes.NewReader(src), md5.New())

	all, err := io.ReadAll(r)
	testutil.Ok(t, err)
	testutil.Equals(t, src, all)

	md5Sum := md5.Sum(src)
	s := r.Sum()

	testutil.Equals(t, md5Sum[:], s)

	fmt.Println(r.SumHex())
	fmt.Println(hex.EncodeToString(md5Sum[:]))
	testutil.Equals(t, hex.EncodeToString(md5Sum[:]), r.SumHex())
}

type testCase struct {
	name            string
	body            []byte
	contentEncoding string
}

func TestGzipReadWithMD5(t *testing.T) {

	gzipOut := &bytes.Buffer{}
	gw := gzip.NewWriter(gzipOut)
	_, err := gw.Write([]byte(testText))
	testutil.Ok(t, err)
	err = gw.Close()
	testutil.Ok(t, err)

	testCases := []testCase{
		{
			name:            "plain body",
			body:            []byte(testText),
			contentEncoding: "",
		},
		{
			name:            "gzip body",
			body:            gzipOut.Bytes(),
			contentEncoding: "gzip",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			req1, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(tc.body))
			testutil.Ok(t, err)
			if tc.contentEncoding != "" {
				req1.Header.Set("Content-Encoding", tc.contentEncoding)
			}

			req2, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(tc.body))
			testutil.Ok(t, err)
			if tc.contentEncoding != "" {
				req2.Header.Set("Content-Encoding", tc.contentEncoding)
			}

			ginCtx := &gin.Context{Request: req1}
			body1, sum1, err := GinReadWithMD5(ginCtx)
			testutil.Ok(t, err)

			body2, sum2, err := GzipReadWithMD5(req2)
			testutil.Ok(t, err)

			testutil.Equals(t, body1, body2)
			testutil.Equals(t, sum1, sum2)

		})
	}
}

func BenchmarkGzipReadWithMD5(b *testing.B) {

	text := strings.Repeat(testText, 100000)

	gzipOut := &bytes.Buffer{}
	gw := gzip.NewWriter(gzipOut)
	_, err := gw.Write([]byte(text))
	testutil.Ok(b, err)
	err = gw.Close()
	testutil.Ok(b, err)

	sr := strings.NewReader(text)

	b.Run("GinRead", func(t *testing.B) {
		sr.Reset(text)

		req, err := http.NewRequest(http.MethodPost, "/", sr)
		testutil.Ok(t, err)

		_, err = GinRead(&gin.Context{Request: req})
		testutil.Ok(t, err)
	})

	b.Run("GzipRead", func(t *testing.B) {

		sr.Reset(text)

		req, err := http.NewRequest(http.MethodPost, "/", sr)
		testutil.Ok(t, err)

		_, err = GzipRead(req)
		testutil.Ok(t, err)
	})

	b.Run("GinReadWithMD5", func(t *testing.B) {

		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(gzipOut.Bytes()))
		testutil.Ok(t, err)
		req.Header.Set("Content-Encoding", "gzip")

		body, _, err := GinReadWithMD5(&gin.Context{Request: req})
		testutil.Ok(t, err)
		testutil.Equals(t, string(body), text)
	})

	b.Run("GzipReadWithMD5", func(t *testing.B) {

		req, err := http.NewRequest(http.MethodPost, "/", bytes.NewReader(gzipOut.Bytes()))
		testutil.Ok(t, err)
		req.Header.Set("Content-Encoding", "gzip")

		body, _, err := GzipReadWithMD5(req)
		testutil.Ok(t, err)
		testutil.Equals(t, string(body), text)
	})

}
