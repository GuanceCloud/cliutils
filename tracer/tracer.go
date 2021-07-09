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

var (
	GlobalTracer *Tracer
	l            = logger.DefaultSLogger("dk_tracer")
)

type DDLog struct{}

func (ddl DDLog) Log(msg string) {
	l.Debug(msg)
}

type Option func(opt *Tracer)

type Tracer struct {
	Service  string `toml:"service"`
	Version  string `toml:"version"`
	Enabled  bool   `toml:"enabled"`
	Resource string `toml:"resource"`
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	addr     string
	Debug    bool `toml:"debug"`
	logger   ddtrace.Logger
}

func newTracer(opts ...Option) *Tracer {
	tracer := &Tracer{}
	for _, opt := range opts {
		opt(tracer)
	}

	return tracer
}

func (t *Tracer) Start(opts ...Option) {
	for _, opt := range opts {
		opt(t)
	}

	if !t.Enabled {
		return
	}

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
	opts := []ddtrace.StartSpanOption{
		tracer.SpanType(ext.SpanTypeHTTP),
		tracer.ServiceName(t.Service),
		tracer.ResourceName(resource),
	}

	return tracer.StartSpan(resource, opts...)
}

func (t *Tracer) Stop() {
	if t.Enabled {
		tracer.Stop()
	}
}

func (t *Tracer) Middleware(opts ...Option) gin.HandlerFunc {
	for _, opt := range opts {
		opt(t)
	}

	if !t.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	} else {
		return func(c *gin.Context) {
			ssopts := []ddtrace.StartSpanOption{
				tracer.ServiceName(t.Service),
				tracer.ResourceName(t.Resource),
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

// func init() {
// 	if cfg.Cfg.TracerConf != nil {
// 		GlobalTracer = newTracer(WithService("dataway", git.Version), WithEnable(cfg.Cfg.TracerConf.Enabled), WithAgentAddr(cfg.Cfg.TracerConf.Host, cfg.Cfg.TracerConf.Port), WithDebug(cfg.Cfg.TracerConf.Debug), WithLogger(DDLog{}))
// 	} else {
// 		GlobalTracer = newTracer(WithEnable(false))
// 	}
// }
