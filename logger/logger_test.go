package logger

import (
	"flag"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

var (
	flagLogFile = flag.String("f", "/dev/null", "log file path")
)

func _init() {
	flag.Parse()
}

func TestLogger5(t *testing.T) {
	_init()

	SetStdoutRootLogger(DEBUG, OPT_DEFAULT)
	l1 := SLogger("test")
	l1.Info("haha")

	SetGlobalRootLogger("/tmp/x.log", DEBUG, OPT_DEFAULT)
	l2 := SLogger("test")
	l2.Info("haha")
}

func TestLogger4(t *testing.T) {
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_DEFAULT)

	l := SLogger("test")
	l.Debug("this is debug msg")
	l.Info("this is info msg")
	l.Error("this is error msg")
}

func TestLogger3(t *testing.T) {
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR|OPT_RESERVED_LOGGER)

	l := SLogger("test")
	l.Debug("this is debug msg")
	l.Info("this is info msg")
	l.Error("this is error msg")
}

func TestLogger2(t *testing.T) {
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR)

	l := SLogger("test")
	l.Debug("this is debug msg")
	l.Info("this is info msg")
	l.Error("this is error msg")
	//l.Panic("this is panic msg")
}

func TestRorate(t *testing.T) {
	_init()
	l, _ := _NewRotateRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR)

	l1 := getSugarLogger(l, "test1")
	l2 := getSugarLogger(l, "test2")

	l1.Info("this is msg")
	l2.Info("this is msg")
}

func TestLogger1(t *testing.T) {
	_init()
	base := 4
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR)

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
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER|OPT_COLOR)

	l := SLogger("test")
	l.Debug("this is debug message")
	l.Info("this is info message")
	l.Warn("this is warn message")
	l.Error("this is error message")
	//l.Fatal("this is fatal message")
	//l.Panic("this is panic message")
}

func TestStdoutGlobalLogger(t *testing.T) {
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER)

	l := SLogger("test")
	l.Debug("this is debug message")
	l.Info("this is info message")
}

func TestWinGlobalLogger(t *testing.T) {
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_STDOUT|OPT_ENC_CONSOLE|OPT_SHORT_CALLER)

	l := SLogger("test")

	l.Debug("this is debug message")
	l.Info("this is info message")
}

func TestGlobalLoggerNotSet(t *testing.T) {
	_init()
	sl := SLogger("sugar-module")
	sl.Debugf("sugar debug msg")
}

func TestGlobalLogger(t *testing.T) {
	_init()
	SetGlobalRootLogger(*flagLogFile, DEBUG, OPT_ENC_CONSOLE|OPT_SHORT_CALLER)

	sl := SLogger("sugar-module")
	sl.Debugf("sugar debug msg")

	//l := Logger("x-module")
	l := getLogger(defaultRootLogger, "x-module")
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
	_init()
	rl, err := newRootLogger(*flagLogFile, INFO, OPT_ENC_CONSOLE|OPT_SHORT_CALLER)
	if err != nil {
		panic(err)
	}

	sl := getSugarLogger(rl, "testing")
	sl.Debug("test message")
	sl.Info("this is info msg: ", "info msg")

	l := getLogger(rl, "debug")
	l.Debug("this is debug: ", zap.Int("int", 42))
}
