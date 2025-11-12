// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
)

const (
	// 禁用 JSON 形式输出.
	OPT_ENC_CONSOLE = 1 //nolint:golint,stylecheck
	// 显示代码路径时，不显示全路径.
	OPT_SHORT_CALLER = 2 //nolint:stylecheck,golint
	// 日志写到 stdout.
	OPT_STDOUT = 4 //nolint:stylecheck,golint
	// 日志内容中追加颜色.
	OPT_COLOR = 8 //nolint:stylecheck,golint
	// 日志自动切割.
	OPT_ROTATE = 32 //nolint:stylecheck,golint
	// 默认日志 flags.
	OPT_DEFAULT = OPT_ENC_CONSOLE | OPT_SHORT_CALLER | OPT_ROTATE //nolint:stylecheck,golint

	DEBUG  = "debug"
	INFO   = "info"
	WARN   = "warn"
	ERROR  = "error"
	PANIC  = "panic"
	DPANIC = "dpanic"
	FATAL  = "fatal"
)

var (
	MaxSize    = 32 // MB
	MaxBackups = 5
	MaxAge     = 30 // day

	mtx = &sync.Mutex{}
)

type Logger struct {
	name   string
	zsl    *zap.SugaredLogger
	rlimit *rate.Limiter
}

func (l *Logger) Name() string {
	return l.name
}

func (l *Logger) Infof(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Infof(fmt, args...)
		}
	} else {
		l.zsl.Infof(fmt, args...)
	}
}

func (l *Logger) Info(args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Info(args...)
		}
	} else {
		l.zsl.Info(args...)
	}
}

func (l *Logger) Warnf(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Warnf(fmt, args...)
		}
	} else {
		l.zsl.Warnf(fmt, args...)
	}
}

func (l *Logger) Warn(args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Warn(args...)
		}
	} else {
		l.zsl.Warn(args...)
	}
}

func (l *Logger) Errorf(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Errorf(fmt, args...)
		}
	} else {
		l.zsl.Errorf(fmt, args...)
	}
}

func (l *Logger) Error(args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Error(args...)
		}
	} else {
		l.zsl.Error(args...)
	}
}

func (l *Logger) Debugf(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Debugf(fmt, args...)
		}
	} else {
		l.zsl.Debugf(fmt, args...)
	}
}

func (l *Logger) Debug(args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.zsl.Debug(args...)
		}
	} else {
		l.zsl.Debug(args...)
	}
}

func (l *Logger) Fatalf(fmt string, args ...any) {
	// fatal log not rate limited
	l.zsl.Fatalf(fmt, args...)
}

func (l *Logger) Fatal(args ...any) {
	// fatal log not rate limited
	l.zsl.Fatal(args...)
}

func (l *Logger) Panicf(fmt string, args ...any) {
	// panic log not rate limited
	l.zsl.Panicf(fmt, args...)
}

func (l *Logger) Panic(args ...any) {
	// panic log not rate limited
	l.zsl.Panic(args...)
}

func (l *Logger) Level() zapcore.Level {
	return l.zsl.Level()
}

type Option struct {
	Path     string
	Level    string
	MaxSize  int
	Flags    int
	Compress bool
}

func Reset() {
	mtx.Lock()
	defer mtx.Unlock()
	root = nil

	slogs = &sync.Map{}

	defaultStdoutRootLogger = nil

	totalSloggers = 0

	if err := doInitStdoutLogger(); err != nil {
		panic(err.Error())
	}
}

func Close() {
	if root != nil {
		if err := root.Sync(); err != nil {
			_ = err // pass
		}
	}
}

//nolint:gochecknoinits
func init() {
	if err := doInitStdoutLogger(); err != nil {
		panic(err.Error())
	}

	if v, ok := os.LookupEnv("LOGGER_PATH"); ok {
		opt := &Option{
			Level: DEBUG,
			Flags: OPT_DEFAULT,
			Path:  v,
		}

		if err := setRootLoggerFromEnv(opt); err != nil {
			panic(err.Error())
		}
	}
}
