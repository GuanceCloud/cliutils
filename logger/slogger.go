// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package logger

import (
	"fmt"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var (
	totalSloggers int64
	slogs         = &sync.Map{}
)

type SLogerOpt func(*Logger)

func WithRateLimiter(limit float64) SLogerOpt {
	return func(sl *Logger) {
		if limit > 0 {
			sl.rlimit = rate.NewLimiter(rate.Limit(limit), 1)  // no burst
			sl.name = sl.name + fmt.Sprintf("-%.1f-rl", limit) // add suffix to logger name
		}
	}
}

func SLogger(name string, opts ...SLogerOpt) *Logger {
	if root == nil && defaultStdoutRootLogger == nil {
		panic("should not been here: root logger not set")
	}

	sl := &Logger{
		name: name,
	}

	for _, opt := range opts {
		opt(sl)
	}

	sl.zsl = slogger(sl.name, 1)

	return sl
}

func DefaultSLogger(name string) *Logger {
	return &Logger{zsl: slogger(name, 1)}
}

func TotalSLoggers() int64 {
	return atomic.LoadInt64(&totalSloggers)
}

func slogger(name string, callerSkip int) *zap.SugaredLogger {
	r := root // prefer root logger

	if r == nil {
		r = defaultStdoutRootLogger
	}

	if r == nil {
		panic("should not been here")
	}

	newlog := getSugarLogger(r, name, callerSkip)
	if root != nil {
		l, loaded := slogs.LoadOrStore(name, newlog)
		if !loaded {
			atomic.AddInt64(&totalSloggers, 1)
		}

		return l.(*zap.SugaredLogger)
	}

	return newlog
}

func getSugarLogger(l *zap.Logger, name string, callerSkip int) *zap.SugaredLogger {
	if callerSkip > 0 {
		return l.WithOptions(zap.AddCallerSkip(callerSkip)).Sugar().Named(name)
	} else {
		return l.Sugar().Named(name)
	}
}
