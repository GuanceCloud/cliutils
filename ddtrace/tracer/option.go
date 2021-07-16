package tracer

import (
	"fmt"
	"time"

	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

func WithAgentAddr(host string, port int) Option {
	addr := fmt.Sprintf("%s:%d", host, port)
	return func(opt *Tracer) {
		opt.Host = host
		opt.Port = port
		opt.addr = addr
	}
}

func WithService(name, version string) Option {
	return func(opt *Tracer) {
		opt.Service = name
		opt.Version = version
	}
}

func WithDebug(debug bool) Option {
	return func(opt *Tracer) {
		opt.Debug = debug
	}
}

func WithLogger(logger ddtrace.Logger) Option {
	return func(opt *Tracer) {
		opt.logger = logger
	}
}

func WithFinishTime(t time.Time) tracer.FinishOption {
	return tracer.FinishTime(t)
}

func WithError(err error) tracer.FinishOption {
	return tracer.WithError(err)
}
