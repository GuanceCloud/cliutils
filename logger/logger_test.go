package logger

import (
	"testing"
)

func TestLogger(t *testing.T) {
	rl, err := NewRootLogger("/tmp/x", OPT_ENC_CONSOLE|OPT_SHORT_CALLER, LVL_DEBUG)
	if err != nil {
		panic(err)
	}

	l := GetLogger(rl, "testing")

	l.Debug("test message")
}
