// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"os"
	"runtime"
	"sync"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPidAlive(t *T.T) {
	t.Run("pid-1", func(t *T.T) {
		if runtime.GOOS != "windows" {
			assert.True(t, pidAlive(1))
		}
	})

	t.Run("pid-not-exist", func(t *T.T) {
		assert.False(t, pidAlive(-1))
	})

	t.Run("cur-pid", func(t *T.T) {
		assert.True(t, pidAlive(os.Getpid()))
	})
}

func TestLockUnlock(t *T.T) {
	t.Run("lock", func(t *T.T) {
		p := t.TempDir()

		wg := sync.WaitGroup{}

		wg.Add(3)
		go func() {
			defer wg.Done()
			fl := newFlock(p)

			assert.NoError(t, fl.lock())
			defer fl.unlock()

			time.Sleep(time.Second * 5)
		}()

		time.Sleep(time.Second) // wait 1st goroutine ok

		go func() {
			defer wg.Done()
			fl := newFlock(p)

			err := fl.lock()
			assert.Error(t, err)

			t.Logf("[expect] err: %s", err.Error())
		}()

		time.Sleep(time.Second) // wait 2nd goroutine ok

		go func() {
			defer wg.Done()
			fl := newFlock(p)

			// try lock until ok
			for {
				if err := fl.lock(); err != nil {
					t.Logf("[expect] err: %s", err.Error())
					time.Sleep(time.Second)
				} else {
					break
				}
			}
		}()

		wg.Wait()
	})
}
