package http

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

const (
	HdrTraceID = `X-Trace-ID`
)

func CORSMiddleware(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
	c.Writer.Header().Set("Access-Control-Allow-Headers",
		"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-Token, X-DatakitUUID, X-RP, X-Precision")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

	if c.Request.Method == "OPTIONS" {
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}

func TraceIDMiddleware(c *gin.Context) {
	if c.Request.Method == `OPTIONS` {
		c.Next()
	} else {
		tid := c.Request.Header.Get(HdrTraceID)
		if len(tid) == 0 {
			tid = cliutils.UUID(`trace_`)
			c.Request.Header.Set(HdrTraceID, tid)
		}

		c.Writer.Header().Set(HdrTraceID, tid)
		c.Next()
	}
}

type bodyLoggerWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func FormatRequest(r *http.Request) string {
	var request []string

	// Add the request string
	url := fmt.Sprintf("%v %v %v", r.Method, r.URL, r.Proto)
	request = append(request, url)
	// Add the host
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// Loop through headers

	for name, headers := range r.Header {
		// name = strings.ToLower(name)
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	if r.Method == "POST" {
		request = append(request, "\n")
	}

	// Return the request as a string
	return strings.Join(request, "\n")
}

func RequestLoggerMiddleware(c *gin.Context) {

	tid := c.Writer.Header().Get(HdrTraceID)
	w := &bodyLoggerWriter{
		ResponseWriter: c.Writer,
		body:           bytes.NewBufferString(``),
	}

	c.Writer = w
	c.Next()

	code := c.Writer.Status()
	switch code / 200 {
	case 1:
		log.Printf("[debug][%s] body size: %d, url: %s, method: %s",
			tid, w.body.Len(), c.Request.URL, c.Request.Method)
	default:
		log.Printf("[warn][%s] Status: %d, RemoteAddr: %s, Request: %s, error: %s",
			tid, code, c.Request.RemoteAddr, FormatRequest(c.Request), w.body.String())
	}
}
