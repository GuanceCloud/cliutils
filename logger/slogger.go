// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package logger

import (
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var (
	totalSloggers int64
	slogs         = &sync.Map{}
)

func SLogger(name string) *Logger {
	if root == nil && defaultStdoutRootLogger == nil {
		panic("should not been here: root logger not set")
	}

	return &Logger{SugaredLogger: slogger(name, 0)}
}

func RateLimitSLogger(name string, logsPerSec float64) *RateLimitedLogger {
	if root == nil && defaultStdoutRootLogger == nil {
		panic("should not been here: root logger not set")
	}

	if logsPerSec > 0 {
		return &RateLimitedLogger{
			// we have re-defined new Logger functions(Infof/Warnf/...), so setup callstack skip 1.
			l:      &Logger{SugaredLogger: slogger(name, 1)},
			rlimit: rate.NewLimiter(rate.Limit(logsPerSec), 1), // no burst
		}
	} else {
		return &RateLimitedLogger{
			// we have re-defined new Logger functions(Infof/Warnf/...), so setup callstack skip 1.
			l: &Logger{SugaredLogger: slogger(name, 1)},
		}
	}
}

func DefaultSLogger(name string) *Logger {
	return &Logger{SugaredLogger: slogger(name, 0)}
}

func DefaultRateLimitSLogger(name string) *Logger {
	return RateLimitSLogger(name, 0)
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
