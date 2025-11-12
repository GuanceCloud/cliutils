// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestRateLimitSLogger(t *T.T) {
	opt := &Option{
		Path:  "stdout",
		Level: DEBUG,
		Flags: OPT_ENC_CONSOLE | OPT_SHORT_CALLER,
	}

	assert.NoError(t, InitRoot(opt))

	t.Run(`basic`, func(t *T.T) {
		l := RateLimitSLogger("basic", 10) // limit 10 logs/sec
		x := 0
		tick := time.NewTicker(time.Second * 5)

	out:
		for {
			x++
			l.Infof("[%d] This is a frequently occurring log message.", x)
			l.Warnf("[%d] This is a frequently occurring log message.", x)
			l.Debugf("[%d] This is a frequently occurring log message.", x)
			l.Errorf("[%d] This is a frequently occurring log message.", x)

			select {
			case <-tick.C:
				log.Printf("triggered")
				break out
			default: // pass
			}
		}

		assert.True(t, x > 5)
	})

	t.Run(`default`, func(t *T.T) {
		l := DefaultRateLimitSLogger("default") // limit 10 logs/sec
		x := 0
		tick := time.NewTicker(time.Second * 1)

	out:
		for {
			x++
			l.Debugf("[%d] This is a frequently occurring log message.", x)

			select {
			case <-tick.C:
				log.Printf("triggered")
				break out
			default: // pass
			}
		}

		assert.True(t, x > 5)
	})
}

func BenchmarkMuitiLogs(b *testing.B) {
	opt := &Option{
		Path:  "/dev/null",
		Level: INFO,
		Flags: OPT_ENC_CONSOLE | OPT_SHORT_CALLER,
	}

	if err := InitRoot(opt); err != nil {
		b.Fatal(err)
	}

	b.Run(`basic`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			l := SLogger(fmt.Sprintf("bench-multi-%d", i))

			l.Debug("debug message")
			l.Info("info message")
			l.Warn("warn message")

			l.Debugf("debugf message: %s", "hello debug")
			l.Infof("info message: %s", "hello info")
			l.Warnf("warn message: %s", "hello warn")
		}
	})

	b.Run(`rate-limited`, func(b *T.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l := RateLimitSLogger(fmt.Sprintf("rate-limited-%d", i), 1)

			l.Debug("debug message")
			l.Info("info message")
			l.Warn("warn message")

			l.Debugf("debugf message: %s", "hello debug")
			l.Infof("info message: %s", "hello info")
			l.Warnf("warn message: %s", "hello warn")
		}
	})
}

func BenchmarkBasic(b *testing.B) {
	opt := &Option{
		Path:  "/dev/null",
		Level: INFO,
		Flags: OPT_ENC_CONSOLE | OPT_SHORT_CALLER,
	}

	if err := InitRoot(opt); err != nil {
		b.Fatal(err)
	}

	b.Run(`basic`, func(b *T.B) {
		l := SLogger("bench")

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			l.Debug("debug message")
			l.Info("info message")
			l.Warn("warn message")

			l.Debugf("debugf message: %s", "hello debug")
			l.Infof("info message: %s", "hello info")
			l.Warnf("warn message: %s", "hello warn")
		}
	})

	b.Run(`rate-limited`, func(b *T.B) {
		l := RateLimitSLogger("bench", 1)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			l.Debug("debug message")
			l.Info("info message")
			l.Warn("warn message")

			l.Debugf("debugf message: %s", "hello debug")
			l.Infof("info message: %s", "hello info")
			l.Warnf("warn message: %s", "hello warn")
		}
	})
}

func TestLoggerSideEffect(t *testing.T) {
	type abc struct {
		i int
	}

	opt := &Option{
		Level: INFO,
		Flags: OPT_DEFAULT,
	}

	f := func(x *abc) string {
		x.i++
		return fmt.Sprintf("%d", x.i)
	}

	if err := InitRoot(opt); err != nil {
		t.Error(err)
	}

	l := SLogger("TestLoggerSideEffect")

	x := &abc{}
	l.Debugf("%+#v", f(x)) // under info level, on debug, the f() still effected

	assert.Equal(t, 1, x.i)
}

func TestJsonLogging(t *testing.T) {
	opt := &Option{
		Path:  "log.json",
		Level: DEBUG,
		Flags: OPT_SHORT_CALLER | OPT_ROTATE,
	}

	assert.NoError(t, InitRoot(opt))

	_, err := os.Stat(opt.Path)
	require.NoError(t, err)

	l := SLogger("json")

	l.Info("this is the json message with short path")

	showLog(t, opt.Path)

	// check json elements
	j, err := os.ReadFile(opt.Path)
	assert.NoError(t, err)

	var logdata map[string]string

	assert.NoError(t, json.Unmarshal(j, &logdata))

	for _, k := range []string{
		NameKeyMod,
		NameKeyMsg,
		NameKeyLevel,
		NameKeyTime,
		NameKeyPos,
	} {
		_, ok := logdata[k]
		assert.True(t, ok)
	}

	Reset()

	opt1 := &Option{
		Path:  "abc.log",
		Level: DEBUG,
		Flags: OPT_ENC_CONSOLE | OPT_ROTATE,
	}

	if err := InitRoot(opt1); err != nil {
		t.Fatal(err)
	}

	l2 := SLogger("log")
	l2.Debug("this is the raw message with full path")
	showLog(t, opt1.Path)

	os.Remove(opt.Path)
	os.Remove(opt1.Path)
}

func TestEnvLogPath(t *testing.T) {
	cases := []struct {
		name    string
		envPath string
		msg     string
		fail    bool
	}{
		{
			name:    "stdout",
			envPath: "",
			msg:     "this is debug log",
		},
		{
			name:    "windows-nul",
			envPath: "nul",
		},
		{
			name:    "unix-null",
			envPath: "/dev/null",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			Reset()
			t.Setenv("LOGGER_PATH", tc.envPath)

			opt := &Option{Path: "" /* path not set, use env only */}

			err := InitRoot(opt)
			if tc.fail {
				assert.True(t, err != nil)
				t.Logf("expect error: %s", err)
			} else {
				assert.NoError(t, err)
			}

			l := SLogger(tc.name)
			l.Debug(tc.msg)
		})
	}
}

func TestLogAppend(t *testing.T) {
	Reset()

	f := "TestLogAppend.log"
	opt := &Option{
		Path:  f,
		Level: DEBUG,
		Flags: OPT_DEFAULT,
	}

	defer os.Remove(f)

	if err := InitRoot(opt); err != nil {
		t.Fatal(err)
	}

	l := SLogger("test1")
	l.Debug("this is the first time logging")

	Close()

	if err := InitRoot(opt); err != nil {
		t.Fatal(err)
	}

	l = SLogger("test1")
	l.Debug("this is the second append logging")

	Close()

	assert.Equal(t, 2, logLines(opt.Path))
	showLog(t, opt.Path)
}

func TestTotalSLoggers(t *testing.T) {
	Reset()

	f := "TestTotalSLoggers"
	opt := &Option{
		Path:  f,
		Level: DEBUG,
		Flags: OPT_DEFAULT,
	}

	defer os.Remove(f)

	if err := InitRoot(opt); err != nil {
		t.Fatal(err)
	}

	n := int64(1000)

	for i := int64(0); i < n; i++ {
		_ = SLogger(fmt.Sprintf("slogger-%d", i))
	}

	// should not create new SLogger any more
	for i := int64(0); i < n; i++ {
		_ = SLogger(fmt.Sprintf("slogger-%d", i))
	}

	total := TotalSLoggers()

	assert.Equalf(t, n, total, fmt.Sprintf("%d != %d", n, total))
}

func TestInitRoot(t *testing.T) {
	Reset()

	cases := []struct {
		name        string
		opt         *Option
		logs        [][2]string
		color, fail bool
	}{
		{
			name: "stdout-log-no-color",
			opt: &Option{
				Level: INFO,
				Flags: (OPT_DEFAULT | OPT_STDOUT),
			},
			logs: [][2]string{
				{INFO, "stdout info log"},
				{DEBUG, "stdout debug log"},
			},
			fail: false,
		},

		{
			name: "stdout-log-with-color",
			opt: &Option{
				Level: INFO,
				Flags: (OPT_DEFAULT | OPT_STDOUT | OPT_COLOR),
			},
			logs: [][2]string{
				{INFO, "stdout info log with color"},
				{DEBUG, "stdout debug log with color"},
			},
			fail: false,
		},

		{
			name: "normal case",
			opt: &Option{
				Path:  "0.log",
				Level: DEBUG,
				Flags: OPT_DEFAULT,
			},
			logs: [][2]string{
				{DEBUG, "abc123"},
				{INFO, "abc123"},
				{WARN, "abc123"},
			},
			color: false,
		},

		{
			name: "with color",
			opt: &Option{
				Path:  "1.log",
				Level: DEBUG,
				Flags: OPT_DEFAULT | OPT_COLOR,
			},
			logs: [][2]string{
				{DEBUG, "abc123"},
				{INFO, "abc123"},
				{WARN, "abc123"},
			},
			color: true,
		},

		{
			name: "stdout log with path set",
			opt: &Option{
				Path:  "2.log",
				Level: DEBUG,
				Flags: (OPT_DEFAULT | OPT_STDOUT),
			},
			logs: [][2]string{
				{DEBUG, "abc123"},
			},
			fail: true,
		},

		{
			name: "no flags",
			opt: &Option{
				Path:  "3.log",
				Level: DEBUG,
				Flags: OPT_ROTATE | OPT_ENC_CONSOLE,
			},
			logs: [][2]string{
				{DEBUG, "abc123"},
				{INFO, "abc123"},
				{WARN, "abc123"},
			},
			color: false,
		},
	}

	for idx, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := InitRoot(c.opt)
			l := SLogger(fmt.Sprintf("case-%d", idx))
			if c.fail {
				assert.Error(t, err)
				t.Logf("[%d] expected failing", idx)
				return
			}

			assert.NoError(t, err)

			for _, arr := range c.logs {
				switch arr[0] {
				case DEBUG:
					l.Debug(arr[1])
				case INFO:
					l.Info(arr[1])
				case WARN:
					l.Warn(arr[1])
				case ERROR:
					l.Error(arr[1])
				default:
					l.Debug(arr[1])
				}
			}

			Reset() // reset root logger
			if c.opt.Flags&OPT_STDOUT == 0 {
				t.Logf("case %d on file: %s", idx, c.opt.Path)
				assert.Equal(t, len(c.logs), logLines(c.opt.Path))
				assert.Equal(t, c.color, colorExits(c.opt.Path))
				showLog(t, c.opt.Path)
				os.Remove(c.opt.Path)
			}
		})
	}
}

func TestRotateOnDevNull(t *testing.T) {
	MaxSize = 1 // set to 1MB

	opt := &Option{
		Path:  "/dev/null",
		Level: INFO,
		Flags: OPT_ENC_CONSOLE | OPT_SHORT_CALLER | OPT_ROTATE, // set rotate
	}

	assert.NoError(t, InitRoot(opt))

	t.Logf("MaxSize: %d", MaxSize)

	l := SLogger(t.Name())
	logData := strings.Repeat("3.1415926x", 1024) // 10kb
	i := 0
	for {
		l.Info(logData)
		i++
		if i >= 200 { // 2MB
			break
		}
	}
}

type BufferSync struct {
	io.ReadWriter
}

func (b *BufferSync) Sync() error {
	return nil
}

func TestTrace(t *testing.T) {
	tests := []struct {
		name            string
		enabled         bool
		traceLoggerName string
		spanLoggerName  string
		message         string
		extractTrace    ExtractTrace
		expectedOutput  string
	}{
		{
			name:            "parse trace",
			enabled:         true,
			traceLoggerName: "traceID",
			spanLoggerName:  "span_id",
			message:         "test",
			extractTrace: func(ctx context.Context) Trace {
				return Trace{
					SpanID:  "2",
					TraceID: "1",
				}
			},
			expectedOutput: `{"level":"DEBUG","message":"test","traceID":"1","span_id":"2"}` + "\n",
		},
		{
			name:           "not trace",
			enabled:        false,
			message:        "test",
			extractTrace:   nil,
			expectedOutput: `{"level":"DEBUG","message":"test"}` + "\n",
		},
		{
			name:            "not parse",
			enabled:         true,
			traceLoggerName: "traceID",
			spanLoggerName:  "span_id",
			message:         "test",
			extractTrace:    nil,
			expectedOutput:  `{"level":"DEBUG","message":"test","traceID":"","span_id":""}` + "\n",
		},
	}

	buf := bytes.NewBufferString("")
	bsync := &BufferSync{buf}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ast := assert.New(t)
			opt := make([]CtxOption, 0)
			opt = append(opt,
				WithTraceKey(test.traceLoggerName, test.spanLoggerName),
				WithParseTrace(test.extractTrace),
			)
			if test.enabled {
				opt = append(opt, EnableTrace())
			}

			opt = append(opt, WithZapCore(
				zapcore.NewCore(
					zapcore.NewJSONEncoder(zapcore.EncoderConfig{
						MessageKey:  "message",
						LevelKey:    "level",
						EncodeLevel: zapcore.CapitalLevelEncoder,
					}),
					bsync,
					zapcore.DebugLevel,
				)))

			logger := NewLoggerCtx(opt...)

			ctx := context.Background()

			logger.DebugfCtx(ctx, test.message)
			re, err := io.ReadAll(bsync)
			if err != nil {
				log.Fatal(err)
			}
			ast.Equal(test.expectedOutput, string(re))
		})
	}
}

//nolint:forbidigo
func showLog(t *testing.T, f string) {
	t.Helper()
	logdata, err := os.ReadFile(f)
	assert.NoError(t, err)

	fmt.Printf("---------- %s ----------\n", f)
	fmt.Println(string(logdata))
}

func colorExits(f string) bool {
	logdata, err := os.ReadFile(f)
	if err != nil {
		panic(err)
	}

	// there should be `[0m` in log files if color enabled
	return bytes.Contains(logdata, []byte("[0m"))
}

func logLines(f string) int {
	logdata, err := os.ReadFile(f)
	if err != nil {
		panic(fmt.Sprintf("ReadFile(%s) failed: %s", f, err))
	}

	return len(bytes.Split(logdata, []byte("\n"))) - 1
}
