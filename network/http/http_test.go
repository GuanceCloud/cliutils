// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"

	// nolint:gosec
	"crypto/md5"
	"fmt"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/testutil"
)

func TestSign(t *testing.T) {
	o := &SignOption{
		AuthorizationType: "ABC",
		SignHeaders:       []string{`Date`, `Content-MD5`, `Content-Type`},
		SK:                "sk-cba",
		AK:                "ak-123",
		Sign:              "",
	}

	r, err := http.NewRequest(`POST`, `http://1.2.3.4:1234/v1/api`, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	r.Header.Set(`Content-MD5`, fmt.Sprintf("%x", md5.Sum([]byte(`{}`)))) //nolint:gosec
	r.Header.Set(`Content-Type`, `application/json`)

	o.Sign, err = o.SignReq(r)
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set(`Authorization`, fmt.Sprintf("%s %s:%s", o.AuthorizationType, o.AK, o.Sign))
	log.Printf("[debug] %+#v", o)

	o2 := &SignOption{
		AuthorizationType: "ABC",
		SignHeaders:       []string{`Date`, `Content-MD5`, `Content-Type`},
		SK:                "sk-cba",
		AK:                "ak-123",
	}

	if err := o2.ParseAuth(r); err != nil {
		t.Fatal(err)
	}

	if sign, err := o2.SignReq(r); err != nil || o2.Sign != sign {
		t.Fatalf("sign failed: %v, %s <> %s", err, o2.Sign, sign)
	}
	log.Printf("[debug] %+#v", o)
}

func TestMagic(t *testing.T) {
	buf := &bytes.Buffer{}
	w := gzip.NewWriter(buf)
	_, err := w.Write(LZ4Magic)
	testutil.Ok(t, err)
	err = w.Close()
	testutil.Ok(t, err)

	if bytes.Compare(GzipMagic, buf.Bytes()[:len(GzipMagic)]) != 0 {
		t.Fatalf("gzip magic: %v expected, got: %v", GzipMagic, buf.Bytes()[:len(GzipMagic)])
	}

	zipBuf := &bytes.Buffer{}
	zw := zip.NewWriter(zipBuf)
	f, err := zw.Create("tmp")
	testutil.Ok(t, err)
	_, err = f.Write(GzipMagic)
	testutil.Ok(t, err)
	err = zw.Close()
	testutil.Ok(t, err)

	if bytes.Compare(ZIPMagic, zipBuf.Bytes()[:len(ZIPMagic)]) != 0 {
		t.Fatalf("zip magic: %v expected, got: %v", ZIPMagic, buf.Bytes()[:len(ZIPMagic)])
	}
}

func TestReadBody(t *testing.T) {

	type testCase struct {
		name            string
		reqBody         io.Reader
		contentEncoding string
	}

	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	_, err := gw.Write([]byte(testText))
	testutil.Ok(t, err)
	err = gw.Close()
	testutil.Ok(t, err)

	cases := []testCase{
		{
			name:            "plain",
			reqBody:         strings.NewReader(testText),
			contentEncoding: "",
		},
		{
			name:            "gzip",
			reqBody:         buf,
			contentEncoding: "gzip",
		},
		{
			name:            "invalid gzip",
			reqBody:         strings.NewReader(testText),
			contentEncoding: "gzip",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/", c.reqBody)
			testutil.Ok(t, err)
			if c.contentEncoding != "" {
				req.Header.Set("Content-Encoding", c.contentEncoding)
			}
			body, err := ReadBody(req)
			testutil.Ok(t, err)
			testutil.Equals(t, testText, string(body))
			
			_, err = req.Body.Read(make([]byte, 1))
			testutil.Assert(t, errors.Is(err, io.EOF), "io.EOF expected, %v got", err)
		})
	}

}
