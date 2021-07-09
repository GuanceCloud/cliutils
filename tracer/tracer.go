package tracer

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/logger"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

var l = logger.DefaultSLogger("dk_tracer")

type DDLog struct{}

func (ddl DDLog) Log(msg string) { // use exist logger for ddtrace log
	l.Debug(msg)
}

type Option func(opt *Tracer)

type Tracer struct {
	Service string `toml:"service"`
	Version string `toml:"version"`
	Enabled bool   `toml:"enabled"`
	Host    string `toml:"host"`
	Port    int    `toml:"port"`
	addr    string
	Debug   bool `toml:"debug"`
	logger  ddtrace.Logger
}

func NewTracer(enabled bool, opts ...Option) *Tracer {
	tracer := &Tracer{Enabled: enabled}
	for _, opt := range opts {
		opt(tracer)
	}

	return tracer
}

func (t *Tracer) Start(opts ...Option) {
	if !t.Enabled {
		return
	}

	for _, opt := range opts {
		opt(t)
	}

	t.addr = fmt.Sprintf("%s:%d", t.Host, t.Port)

	sopts := []tracer.StartOption{
		tracer.WithEnv("prod"),
		tracer.WithService(t.Service),
		tracer.WithServiceVersion(t.Version),
		tracer.WithAgentAddr(t.addr),
		tracer.WithDebugMode(t.Debug),
		tracer.WithLogger(t.logger),
	}

	l.Infof("starting ddtrace on datakit...")
	tracer.Start(sopts...)
}

func (t *Tracer) StartSpan(resource string) ddtrace.Span {
	if !t.Enabled {
		return nil
	}

	opts := []ddtrace.StartSpanOption{
		tracer.SpanType(ext.SpanTypeHTTP),
		tracer.ServiceName(t.Service),
		tracer.ResourceName(resource),
	}

	return tracer.StartSpan(resource, opts...)
}

func (t *Tracer) SetSpanTag(span tracer.Span, key string, value interface{}) {
	if t.Enabled && span != nil {
		span.SetTag(key, value)
	}
}

func (t *Tracer) FinishSpan(span tracer.Span, opts ...ddtrace.FinishOption) {
	if t.Enabled && span != nil {
		span.Finish(opts...)
	}
}

func (t *Tracer) Middleware(resource string, opts ...Option) gin.HandlerFunc {
	if !t.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	} else {
		for _, opt := range opts {
			opt(t)
		}

		return func(c *gin.Context) {
			ssopts := []ddtrace.StartSpanOption{
				tracer.ServiceName(t.Service),
				tracer.ResourceName(resource),
				tracer.SpanType(ext.SpanTypeWeb),
				tracer.Tag(ext.HTTPMethod, c.Request.Method),
				tracer.Tag(ext.HTTPURL, c.Request.URL.Path),
				tracer.Measured(),
			}
			if spanctx, err := tracer.Extract(tracer.HTTPHeadersCarrier(c.Request.Header)); err == nil {
				ssopts = append(ssopts, tracer.ChildOf(spanctx))
			}

			span, ctx := tracer.StartSpanFromContext(c.Request.Context(), "http.request", ssopts...)
			defer span.Finish()

			c.Request = c.Request.WithContext(ctx)
			c.Next()

			status := c.Writer.Status()
			span.SetTag(ext.HTTPCode, strconv.Itoa(status))
			if status >= 500 && status < 600 {
				span.SetTag(ext.Error, fmt.Errorf("%d: %s", status, http.StatusText(status)))
			}
			if len(c.Errors) > 0 {
				span.SetTag("gin.errors", c.Errors.String())
			}
		}
	}
}

func (t *Tracer) Inject(span ddtrace.Span, header http.Header) {
	if t.Enabled {
		tracer.Inject(span.Context(), tracer.HTTPHeadersCarrier(header))
	}
}

func (t *Tracer) Stop() {
	if t.Enabled {
		tracer.Stop()
	}
}
