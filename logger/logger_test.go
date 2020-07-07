package logger

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRorate(t *testing.T) {
	l, _ := _NewRotateRootLogger("/tmp/x.log", DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR)

	l1 := GetSugarLogger(l, "test1")
	l2 := GetSugarLogger(l, "test2")

	l1.Info("this is msg")
	l2.Info("this is msg")
}

func TestXX(t *testing.T) {
	base := 4
	if err := SetGlobalRootLogger("/tmp/xlog", DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR); err != nil {
		t.Fatal(err)
	}

	l1 := SLogger("test")
	l2 := SLogger("test")

	wg := sync.WaitGroup{}

	for j := 0; j < base; j++ {
		wg.Add(2)
		go func() {
			i := 0
			defer wg.Done()
			for {
				l1.Debugf("L1: %v", l1)
				i++

				if i%(base*8) == 0 {
					fmt.Printf("[%d]L1: %d\n", j, i)
				}

				if i > base*32 {
					return
				}
			}
		}()

		go func() {
			i := 0
			defer wg.Done()
			for {
				l2.Debugf("L2: %v", l2)
				i++

				if i%(base*8) == 0 {
					fmt.Printf("[%d]L2: %d\n", j, i)
				}

				if i > base*32 {
					return
				}
			}
		}()
	}

	wg.Wait()
}

func TestColor(t *testing.T) {
	if err := SetGlobalRootLogger("", DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR); err != nil {
		t.Fatal(err)
	}

	l := SLogger("test")
	l.Debug("this is debug message")
	l.Info("this is info message")
	l.Warn("this is warn message")
	l.Error("this is error message")
	//l.Fatal("this is fatal message")
	//l.Panic("this is panic message")
}

func TestStdoutGlobalLogger(t *testing.T) {
	if err := SetGlobalRootLogger("", DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER); err != nil {
		t.Fatal(err)
	}

	l := SLogger("test")
	l.Debug("this is debug message")
	l.Info("this is info message")
}

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
