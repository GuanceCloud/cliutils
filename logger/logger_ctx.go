package logger

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

type LoggerCtx interface {
	Debugf(template string, args ...any)
	Infof(template string, args ...any)
	Warnf(template string, args ...any)
	Errorf(template string, args ...any)
	Debug(args ...any)
	Info(args ...any)
	Warn(args ...any)
	Error(args ...any)

	DebugfCtx(ctx context.Context, template string, args ...any)
	InfofCtx(ctx context.Context, template string, args ...any)
	WarnfCtx(ctx context.Context, template string, args ...any)
	ErrorfCtx(ctx context.Context, template string, args ...any)
	DebugCtx(ctx context.Context, template string, args ...any)
	InfoCtx(ctx context.Context, template string, args ...any)
	WarnCtx(ctx context.Context, template string, args ...any)
	ErrorCtx(ctx context.Context, template string, args ...any)

	Named(name string) LoggerCtx
	With(fields ...zap.Field) LoggerCtx
}

var _ LoggerCtx = (*loggerCtx)(nil)

type loggerCtx struct {
	logger *zap.Logger
	config
}

func NewLoggerCtx(opts ...CtxOption) *loggerCtx {
	cfg := newDefaultConfig()
	for _, o := range opts {
		cfg = o.apply(cfg)
	}

	cfg.opts = append(cfg.opts, zap.AddCallerSkip(1))

	return &loggerCtx{
		logger: zap.New(cfg.core, cfg.opts...),
		config: cfg,
	}
}

func (l *loggerCtx) GetTrace(ctx context.Context) Trace {
	var traceResult Trace

	if l.parseTrace != nil {
		traceResult = l.parseTrace(ctx)
	}

	return traceResult
}

func (l *loggerCtx) getExtraFields(ctx context.Context) (fields []zap.Field) {
	if l.enabledTrace {
		trace := l.GetTrace(ctx)
		fields = append(fields, zap.String(l.traceKey, trace.TraceID), zap.String(l.spanKey, trace.SpanID))
	}
	return fields
}

func (l *loggerCtx) getMessage(template string, args []any) string {
	if len(args) == 0 {
		return template
	}

	if template != "" {
		return fmt.Sprintf(template, args...)
	}

	if len(args) == 1 {
		if str, ok := args[0].(string); ok {
			return str
		}
	}
	return fmt.Sprint(args...)
}

func (l *loggerCtx) Named(name string) LoggerCtx {
	return &loggerCtx{
		logger: l.logger.Named(name),
		config: l.config,
	}
}

func (l *loggerCtx) Debugf(template string, args ...any) {
	l.logger.Debug(l.getMessage(template, args))
}

func (l *loggerCtx) DebugfCtx(ctx context.Context, template string, args ...any) {
	l.logger.Debug(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) Infof(template string, args ...any) {
	l.logger.Info(l.getMessage(template, args))
}

func (l *loggerCtx) Warnf(template string, args ...any) {
	l.logger.Warn(l.getMessage(template, args))
}

func (l *loggerCtx) Errorf(template string, args ...any) {
	l.logger.Error(l.getMessage(template, args))
}

func (l *loggerCtx) InfofCtx(ctx context.Context, template string, args ...any) {
	l.logger.Info(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) WarnfCtx(ctx context.Context, template string, args ...any) {
	l.logger.Warn(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) ErrorfCtx(ctx context.Context, template string, args ...any) {
	l.logger.Error(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) Debug(args ...any) {
	l.logger.Debug(l.getMessage("", args))
}

func (l *loggerCtx) DebugCtx(ctx context.Context, template string, args ...any) {
	l.logger.Debug(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) Error(args ...any) {
	l.logger.Error(l.getMessage("", args))
}

func (l *loggerCtx) ErrorCtx(ctx context.Context, template string, args ...any) {
	l.logger.Error(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) Info(args ...any) {
	l.logger.Info(l.getMessage("", args))
}

func (l *loggerCtx) InfoCtx(ctx context.Context, template string, args ...any) {
	l.logger.Info(l.getMessage(template, args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) Warn(args ...any) {
	l.logger.Warn(l.getMessage("", args))
}

func (l *loggerCtx) WarnCtx(ctx context.Context, template string, args ...any) {
	l.logger.Warn(l.getMessage("", args), l.getExtraFields(ctx)...)
}

func (l *loggerCtx) With(fields ...zap.Field) LoggerCtx {
	if len(fields) == 0 {
		return l
	}
	temp := l.clone()
	temp.logger = l.logger.With(fields...)
	return temp
}

func (l *loggerCtx) clone() *loggerCtx {
	return &loggerCtx{config: l.config}
}
