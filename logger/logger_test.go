package logger

import (
	"fmt"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestWinGlobalLogger(t *testing.T) {
	if err := SetGlobalRootLogger("C:\\Program Files\\DataFlux\\datakit\\datakit.log", DEBUG, OPT_STDOUT|OPT_ENC_CONSOLE|OPT_SHORT_CALLER); err != nil {
		t.Fatal(err)
	}

	l := SLogger("test")

	l.Debug("this is debug message")
	l.Info("this is info message")
}

func TestGlobalLoggerNotSet(t *testing.T) {
	sl := SLogger("sugar-module")
	sl.Debugf("sugar debug msg")
}

func TestGlobalLogger(t *testing.T) {
	SetGlobalRootLogger("/tmp/log.globle", DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER)

	sl := SLogger("sugar-module")
	sl.Debugf("sugar debug msg")

	//l := Logger("x-module")
	l := GetLogger(defaultRootLogger, "x-module")
	fmt.Printf("%+#v", l)
	//l.Debug("normal msg: ", zap.String("url", "http://1.2.3.4"), zap.Int("attempts", 3), zap.Duration("costs", time.Millisecond))
	//l.Debug("normal msg: ", zap.String("url", "http://1.2.3.4"))

	f := zap.Duration("backoff", time.Second)
	fmt.Println(f)

	l.Info("failed to fetch URL",
		// Structured context as strongly typed Field values.
		zap.String("url", "baidu.com"),
		zap.Int("attempt", 3),
		zap.Int("attempt", 4))
	//		zap.Duration("backoff", time.Millisecond),
	//	)
}

func TestLogger(t *testing.T) {
	rl, err := NewRootLogger("/tmp/x", INFO, OPT_ENC_CONSOLE|OPT_SHORT_CALLER)
	if err != nil {
		panic(err)
	}

	sl := GetSugarLogger(rl, "testing")
	sl.Debug("test message")
	sl.Info("this is info msg: ", "info msg")

	l := GetLogger(rl, "debug")
	l.Debug("this is debug: ", zap.Int("int", 42))
}
