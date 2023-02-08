// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

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
