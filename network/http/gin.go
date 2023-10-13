// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/md5" //nolint:gosec
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/textproto"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/logger"
	"github.com/gin-gonic/gin"
)

const (
	XAgentIP       = "X-Agent-Ip"
	XAgentUID      = "X-Agent-Uid"
	XCQRP          = "X-CQ-RP"
	XDatakitInfo   = "X-Datakit-Info"
	XDatakitUUID   = "X-Datakit-UUID" // deprecated
	XDBUUID        = "X-DB-UUID"
	XDomainName    = "X-Domain-Name"
	XLua           = "X-Lua"
	XPrecision     = "X-Precision"
	XRP            = "X-RP"
	XSource        = "X-Source"
	XTableName     = "X-Table-Name"
	XToken         = "X-Token"
	XTraceID       = "X-Trace-Id"
	XVersion       = "X-Version"
	XWorkspaceUUID = "X-Workspace-UUID"
)

const (
	HeaderWildcard = "*"
	HeaderGlue     = ", "
)

var (
	// Although CORS-safelisted request headers(Accept/Accept-Language/Content-Language/Content-Type) are always allowed
	// and don't usually need to be listed in Access-Control-Allow-Headers,
	// listing them anyway will circumvent the additional restrictions that apply.
	// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Allow-Headers#bypassing_additional_restrictions
	defaultCORSHeader = newCORSHeaders([]string{
		"Content-Type",
		"Content-Length",
		"Accept-Encoding",
		"X-CSRF-Token",
		"Authorization",
		"Accept",
		"Accept-Language",
		"Content-Language",
		"Origin",
		"Cache-Control",
		"X-Requested-With",

		// dataflux headers
		XToken,
		XDatakitUUID,
		XRP,
		XPrecision,
		XLua,
		"*",
	})
	allowHeaders      = defaultCORSHeader.String()
	realIPHeader      = []string{"X-Forwarded-For", "X-Real-IP", "RemoteAddr"}
	MaxRequestBodyLen = 128

	l = logger.DefaultSLogger("gin")
)

func Init() {
	l = logger.SLogger("gin")

	if v, ok := os.LookupEnv("MAX_REQUEST_BODY_LEN"); ok {
		if i, err := strconv.ParseInt(v, 10, 64); err != nil {
			l.Warnf("invalid MAX_REQUEST_BODY_LEN, expect int, got %s, ignored", v)
		} else {
			MaxRequestBodyLen = int(i)
		}
	}
}

type CORSHeaders map[string]struct{}

func newCORSHeaders(headers []string) CORSHeaders {
	ch := make(CORSHeaders, len(headers))
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}
		ch[textproto.CanonicalMIMEHeaderKey(header)] = struct{}{}
	}
	return ch
}

func (c CORSHeaders) String() string {
	headers := make([]string, 0, len(c))
	hasWildcard := false
	for k := range c {
		if k == HeaderWildcard {
			hasWildcard = true
			continue
		}
		headers = append(headers, k)
	}

	sort.Strings(headers)

	if hasWildcard {
		headers = append(headers, "*")
	}

	return strings.Join(headers, HeaderGlue)
}

func (c CORSHeaders) Add(requestHeaders string) string {
	if requestHeaders == "" {
		return allowHeaders
	}
	headers := make([]string, 0)
	for _, key := range strings.Split(requestHeaders, ",") {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		key = textproto.CanonicalMIMEHeaderKey(key)
		if _, ok := c[key]; !ok {
			headers = append(headers, key)
		}
	}
	if len(headers) == 0 {
		return allowHeaders
	}
	return strings.Join(headers, HeaderGlue) + HeaderGlue + allowHeaders
}

func GinLogFormatter(param gin.LogFormatterParams) string {
	realIP := param.ClientIP
	for _, h := range realIPHeader {
		if v := param.Request.Header.Get(h); v != "" {
			realIP = v
		}
	}

	if param.ErrorMessage != "" {
		return fmt.Sprintf("[GIN] %v | %3d | %8v | %15s | %-7s %#v -> %s\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			net.ParseIP(realIP),
			param.Method,
			param.Path,
			param.ErrorMessage)
	} else {
		return fmt.Sprintf("[GIN] %v | %3d | %8v | %15s | %-7s %#v\n",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			param.StatusCode,
			param.Latency,
			net.ParseIP(realIP),
			param.Method,
			param.Path)
	}
}

func CORSMiddleware(c *gin.Context) {
	allowOrigin := c.GetHeader("origin")
	requestHeaders := c.GetHeader("Access-Control-Request-Headers")
	if allowOrigin == "" {
		allowOrigin = "*"
	}

	c.Writer.Header().Set("Access-Control-Allow-Origin", allowOrigin)
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	if requestHeaders != "" {
		c.Writer.Header().Set("Access-Control-Allow-Headers", defaultCORSHeader.Add(requestHeaders))
	} else {
		c.Writer.Header().Set("Access-Control-Allow-Headers", allowHeaders)
	}
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

	// The default value is only 5 seconds, so we explicitly set it to reduce the count of OPTIONS requests.
	// see https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Access-Control-Max-Age#directives
	c.Writer.Header().Set("Access-Control-Max-Age", "7200")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(http.StatusNoContent)
		return
	}

	c.Next()
}

func TraceIDMiddleware(c *gin.Context) {
	if c.Request.Method == `OPTIONS` {
		c.Next()
	} else {
		tid := c.Request.Header.Get(XTraceID)
		if tid == "" {
			tid = cliutils.XID(`trace_`)
			c.Request.Header.Set(XTraceID, tid)
		}

		c.Writer.Header().Set(XTraceID, tid)
		c.Next()
	}
}

func FormatRequest(r *http.Request) string {
	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request := []string{url}

	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers

	for name, headers := range r.Header {
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// Return the request as a string
	return strings.Join(request, "|")
}

type bodyLoggerWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w bodyLoggerWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func RequestLoggerMiddleware(c *gin.Context) {
	w := &bodyLoggerWriter{
		ResponseWriter: c.Writer,
		body:           bytes.NewBufferString(``),
	}

	c.Writer = w
	c.Next()

	body := w.body.String()

	l.Infof("%s %s %d, RemoteAddr: %s, Request: [%s], Body: %s",
		c.Request.Method,
		c.Request.URL,
		c.Writer.Status(),
		c.Request.RemoteAddr,
		FormatRequest(c.Request),
		body[:len(body)%MaxRequestBodyLen]+"...")
}

// Deprecated, use GzipReadWithMD5 instead
func GinReadWithMD5(c *gin.Context) (buf []byte, md5str string, err error) {
	buf, err = readBody(c)
	if err != nil {
		return
	}

	md5str = fmt.Sprintf("%x", md5.Sum(buf)) //nolint:gosec

	if c.Request.Header.Get("Content-Encoding") == "gzip" {
		buf, err = Unzip(buf)
	}

	return
}

// Deprecated, use GzipRead instead.
func GinRead(c *gin.Context) (buf []byte, err error) {
	buf, err = readBody(c)
	if err != nil {
		return
	}

	if c.Request.Header.Get("Content-Encoding") == "gzip" {
		buf, err = Unzip(buf)
	}

	return
}

func GinGetArg(c *gin.Context, hdr, param string) (v string, err error) {
	v = c.Request.Header.Get(hdr)
	if v == "" {
		v = c.Query(param)
		if v == "" {
			err = fmt.Errorf("HTTP header %s and query param %s missing", hdr, param)
		}
	}
	return
}

type ReaderWithHash struct {
	r io.Reader
	h hash.Hash
}

func NewReaderWithHash(r io.Reader, h hash.Hash) *ReaderWithHash {
	return &ReaderWithHash{
		r: r,
		h: h,
	}
}

func (h *ReaderWithHash) Read(p []byte) (int, error) {
	n, err := h.r.Read(p)
	if n > 0 {
		_, we := h.h.Write(p[:n])
		if we != nil {
			return n, fmt.Errorf("unable to write data to hasher: %w", we)
		}
	}
	return n, err
}

func (h *ReaderWithHash) Sum() []byte {
	return h.h.Sum(nil)
}

func (h *ReaderWithHash) SumHex() string {
	return hex.EncodeToString(h.Sum())
}

func (h *ReaderWithHash) Close() error {
	return nil
}

func gzipReadMD5AndClose(req *http.Request, md5Sum bool, closeBody bool) ([]byte, string, error) {

	var (
		rc io.ReadCloser
		rh *ReaderWithHash
	)

	rc = req.Body
	if closeBody {
		defer req.Body.Close()
	}

	if md5Sum {
		rh = NewReaderWithHash(rc, md5.New())
		defer rh.Close()
		rc = rh
	}

	// as an HTTP server, we do not need to close the Body
	switch req.Header.Get("Content-Encoding") {
	case "gzip":
		bufReader := bufio.NewReader(rc)
		magic, err := bufReader.Peek(len(GzipMagic))
		if err != nil {
			return nil, "", fmt.Errorf("unable to peek 2 bytes from Body: %w", err)
		}

		if bytes.Compare(GzipMagic, magic) == 0 {
			rc, err = gzip.NewReader(bufReader)
			if err != nil {
				return nil, "", fmt.Errorf("unable to init gzip reader: %w", err)
			}
			defer rc.Close()
		} else {
			l.Warnf(`illegal gzip format while Content-Encoding = gzip, magic %v expected, got %v`, GzipMagic, magic)
			rc = io.NopCloser(bufReader)
		}
	}

	body, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", fmt.Errorf("unable to successfully read: %w, bytes has read: %d, http Content-Length: %d", err, len(body), req.ContentLength)
	}

	if md5Sum && rh != nil {
		return body, rh.SumHex(), nil
	}

	return body, "", nil

}

// GzipReadWithMD5 will automatically unzip the Request.Body and calculate its MD5 sum,
// it WILL close the Request.Body on return.
func GzipReadWithMD5(req *http.Request) ([]byte, string, error) {
	return gzipReadMD5AndClose(req, true, true)
}

// GzipRead will automatically unzip the Request.Body, it WILL close the Request.Body on return.
func GzipRead(req *http.Request) ([]byte, error) {
	body, _, err := gzipReadMD5AndClose(req, false, true)
	return body, err
}

// Deprecated, you should first consider using GzipRead or GzipReadWithMD5
func Unzip(in []byte) (out []byte, err error) {
	gzr, err := gzip.NewReader(bytes.NewBuffer(in))
	if err != nil {
		return
	}

	out, err = ioutil.ReadAll(gzr)
	if err != nil {
		return
	}

	if err := gzr.Close(); err != nil {
		_ = err // pass
	}
	return
}

func readBody(c *gin.Context) ([]byte, error) {
	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}

	defer func() {
		if err := c.Request.Body.Close(); err != nil {
			_ = err // pass
		}
	}()
	return body, nil
}
