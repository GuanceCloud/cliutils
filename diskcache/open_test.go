// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"os"
	"path/filepath"
	"sync"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOpen(t *T.T) {
	t.Run("reopen-with-or-without-pos", func(t *T.T) {
		p := t.TempDir()

		// lock then no-lock
		c, err := Open(WithPath(p))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

		c2, err := Open(WithPath(p), WithNoPos(true))
		assert.NoError(t, err)
		assert.NoError(t, c2.Close())

		// no-lock then lock
		c, err = Open(WithPath(p), WithNoPos(true))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

		c2, err = Open(WithPath(p))
		assert.NoError(t, err)
		assert.NoError(t, c2.Close())
	})

	t.Run("reopen-with-or-without-lock", func(t *T.T) {
		p := t.TempDir()

		// lock then no-lock
		c, err := Open(WithPath(p))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

		c2, err := Open(WithPath(p), WithNoLock(true))
		assert.NoError(t, err)
		assert.NoError(t, c2.Close())

		// no-lock then lock
		c, err = Open(WithPath(p), WithNoLock(true))
		assert.NoError(t, err)
		assert.NoError(t, c.Close())

		c2, err = Open(WithPath(p))
		assert.NoError(t, err)
		assert.NoError(t, c2.Close())
	})

	t.Run("open-with-lock", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		fi, err := os.Stat(filepath.Join(p, ".lock"))
		assert.NoError(t, err)

		t.Logf("lock file info: %+#v", fi)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			os.RemoveAll(p)
		})
	})

	t.Run("open-twice-with-error", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		_, err = Open(WithPath(p))
		assert.Error(t, err, "expect lock failed")

		t.Logf("[expected] Open: %q", err)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			os.RemoveAll(p)
		})
	})

	t.Run("multi-open-until-ok", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		// hold the cache for 5 seconds
		go func() {
			time.Sleep(time.Second * 2)
			assert.NoError(t, c.Close())
			t.Logf("c Closed")
		}()

		var c2 *DiskCache
		for { // retry until ok
			c2, err = Open(WithPath(p))
			if err != nil {
				t.Logf("[expected] Open: %q, path: %s", err, p)
				time.Sleep(time.Second)
			} else {
				break
			}
		}

		t.Cleanup(func() {
			assert.NoError(t, c2.Close())
		})
	})

	t.Run("multi-open-wihout-lock", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p), WithNoLock(true))
		assert.NoError(t, err)

		var wg sync.WaitGroup

		wg.Add(1)
		// hold the cache for 5 seconds
		go func() {
			defer wg.Done()
			time.Sleep(time.Second * 5)
			assert.NoError(t, c.Close())
		}()

		c2, err := Open(WithPath(p), WithNoLock(true))
		assert.NoError(t, err) // no error on re-Open() without lock
		defer c2.Close()

		wg.Wait()
	})
}

func TestClose(t *T.T) {
	t.Run("multl-close", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		assert.NoError(t, c.Close())
		assert.NoError(t, c.Close())

		t.Cleanup(func() {
			os.RemoveAll(p)
		})
	})
}
