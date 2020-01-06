package http

import (
	"bytes"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

const (
	HdrTraceID = `X-Trace-ID`
)

func TraceIDMiddleware(c *gin.Context) {
	tid := c.Request.Header.Get(HdrTraceID)
	if len(tid) == 0 {
		tid = cliutils.UUID(`trace_`)
		c.Request.Header.Set(HdrTraceID, tid)
	}

	c.Writer.Header().Set(HdrTraceID, tid)
	c.Next()
}

func CorsMiddleware(c *gin.Context) {
	switch c.Request.Method {
	case `OPTIONS`, `HEAD`:
		c.AbortWithStatus(204)
		return
	}

	c.Next()
}

type bodyLoggerWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func BodyLoggerMiddleware(c *gin.Context) {

	w := &bodyLoggerWriter{
		ResponseWriter: c.Writer,
		body:           bytes.NewBufferString(``),
	}

	c.Writer = w
	c.Next()

	tid := c.Writer.Header().Get(HdrTraceID)
	code := c.Writer.Status()
	switch code {
	case http.StatusOK:
		log.Printf("[debug][%s] body size: %d", tid, w.body.Len())
	}
}
