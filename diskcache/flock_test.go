// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"sync"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLockUnlock(t *T.T) {
	t.Run("lock", func(t *T.T) {
		p := t.TempDir()

		wg := sync.WaitGroup{}

		wg.Add(3)
		go func() {
			defer wg.Done()
			fl := newFlock(p)

			ok, err := fl.TryLock()

			assert.True(t, ok)
			assert.NoError(t, err)
			defer fl.Unlock()

			time.Sleep(time.Second * 5)
		}()

		time.Sleep(time.Second) // wait 1st goroutine ok

		go func() {
			defer wg.Done()
			fl := newFlock(p)

			ok, err := fl.TryLock()
			assert.False(t, ok)
			assert.Error(t, err)

			t.Logf("[expect] err: %s", err.Error())
		}()

		time.Sleep(time.Second) // wait 2nd goroutine ok

		go func() {
			defer wg.Done()
			fl := newFlock(p)

			// try lock until ok
			for {
				if ok, err := fl.TryLock(); !ok {
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
