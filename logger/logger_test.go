package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	tu "gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"
)

func BenchmarkMuitiLogs(b *testing.B) {
	opt := &Option{
		Path:  "/dev/null",
		Level: INFO,
		Flags: OPT_ENC_CONSOLE | OPT_SHORT_CALLER,
	}

	if err := InitRoot(opt); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < b.N; i++ {
		l := SLogger(fmt.Sprintf("bench-multi-%d", i))

		l.Debug("debug message")
		l.Info("info message")
		l.Warn("warn message")

		l.Debugf("debugf message: %s", "hello debug")
		l.Infof("info message: %s", "hello info")
		l.Warnf("warn message: %s", "hello warn")
	}
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

	l := SLogger("bench")
	for i := 0; i < b.N; i++ {
		l.Debug("debug message")
		l.Info("info message")
		l.Warn("warn message")

		l.Debugf("debugf message: %s", "hello debug")
		l.Infof("info message: %s", "hello info")
		l.Warnf("warn message: %s", "hello warn")
	}
}

func TestJsonLogging(t *testing.T) {
	opt := &Option{
		Path:  "abc.json",
		Level: DEBUG,
		Flags: OPT_SHORT_CALLER | OPT_ROTATE,
	}

	if err := InitRoot(opt); err != nil {
		t.Fatal(err)
	}

	l := SLogger("json")
	l.Debug("this is the json message with short path")
	showLog(opt.Path)

	// check json elements
	j, err := ioutil.ReadFile(opt.Path)
	if err != nil {
		t.Error(err)
	}

	var logdata map[string]string

	if err := json.Unmarshal(j, &logdata); err != nil {
		t.Error(err)
	}

	for _, k := range []string{
		NameKeyMod,
		NameKeyMsg,
		NameKeyLevel,
		NameKeyTime,
		NameKeyPos,
	} {
		_, ok := logdata[k]
		tu.Assert(t, ok, "%s not exist")
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
	showLog(opt1.Path)

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
			os.Clearenv()

			if err := os.Setenv("LOGGER_PATH", tc.envPath); err != nil {
				t.Fatal(err)
			}

			opt := &Option{Path: "" /* path not set, use env only */}

			err := InitRoot(opt)
			if tc.fail {
				tu.Assert(t, err != nil, "")
				t.Logf("expect error: %s", err)
			} else {
				tu.Equals(t, nil, err)
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

	tu.Equals(t, 2, logLines(opt.Path))
	showLog(opt.Path)
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

	tu.Assert(t, n == total, fmt.Sprintf("%d != %d", n, total))
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
				tu.NotOk(t, err, "")
				t.Logf("[%d] expected failing", idx)
				return
			}

			tu.Ok(t, err)

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
				tu.Equals(t, len(c.logs), logLines(c.opt.Path))
				tu.Equals(t, c.color, colorExits(c.opt.Path))
				showLog(c.opt.Path)
				os.Remove(c.opt.Path)
			}
		})
	}
}

//nolint:forbidigo
func showLog(f string) {
	logdata, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}

	fmt.Printf("---------- %s ----------\n", f)
	fmt.Println(string(logdata))
}

func colorExits(f string) bool {
	logdata, err := ioutil.ReadFile(f)
	if err != nil {
		panic(err)
	}

	// there should be `[0m` in log files if color enabled
	return bytes.Contains(logdata, []byte("[0m"))
}

func logLines(f string) int {
	logdata, err := ioutil.ReadFile(f)
	if err != nil {
		panic(fmt.Sprintf("ReadFile(%s) failed: %s", f, err))
	}

	return len(bytes.Split(logdata, []byte("\n"))) - 1
}
