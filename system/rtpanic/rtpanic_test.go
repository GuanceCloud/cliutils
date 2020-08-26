package rtpanic

import (
	"fmt"
	"log"
	"testing"
)

func TestRecover(t *testing.T) {
	var f RecoverCallback
	panicCnt := 0

	f = func(trace []byte, err error) {
		defer Recover(f, nil)

		if trace != nil {
			panicCnt++
			log.Printf("try panic(at %d), err: %v, trace: %s", panicCnt, err, string(trace))
			if panicCnt >= 1 {
				return
			}
		}

		panic(fmt.Errorf("panic error"))
	}

	f(nil, nil)
}
