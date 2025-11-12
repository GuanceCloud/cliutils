// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package logger

import (
	"os"
	"sync"

	"go.uber.org/zap"
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
	*zap.SugaredLogger
	rlimit *rate.Limiter
}

func (l *Logger) Infof(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Infof(fmt, args...)
		}
	} else {
		l.SugaredLogger.Infof(fmt, args...)
	}
}

func (l *Logger) Info(fmt string) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Info(fmt)
		}
	} else {
		l.SugaredLogger.Info(fmt)
	}
}

func (l *Logger) Warnf(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Warnf(fmt, args...)
		}
	} else {
		l.SugaredLogger.Warnf(fmt, args...)
	}
}

func (l *Logger) Warn(fmt string) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Warn(fmt)
		}
	} else {
		l.SugaredLogger.Warn(fmt)
	}
}

func (l *Logger) Errorf(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Errorf(fmt, args...)
		}
	} else {
		l.SugaredLogger.Errorf(fmt, args...)
	}
}

func (l *Logger) Error(fmt string) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Error(fmt)
		}
	} else {
		l.SugaredLogger.Error(fmt)
	}
}

func (l *Logger) Debugf(fmt string, args ...any) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Debugf(fmt, args...)
		}
	} else {
		l.SugaredLogger.Debugf(fmt, args...)
	}
}

func (l *Logger) Debug(fmt string) {
	if l.rlimit != nil {
		if l.rlimit.Allow() {
			l.SugaredLogger.Debug(fmt)
		}
	} else {
		l.SugaredLogger.Debug(fmt)
	}
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
