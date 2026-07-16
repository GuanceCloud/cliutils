// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpen(t *T.T) {
	t.Run("reopen-with-or-without-pos", func(t *T.T) {
		p := t.TempDir()

		// lock then no-lock
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		assert.FileExists(t, filepath.Join(p, ".lock"))

		assert.NoError(t, c.Close())
		assert.FileExists(t, filepath.Join(p, ".lock"))

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

	t.Run("test-empty-pos-file", func(t *T.T) {
		p := t.TempDir()
		posFile := filepath.Join(p, ".pos")

		f, err := os.Create(posFile)
		assert.NoError(t, err)

		dq, err := Open(WithPath(p), WithCapacity(1<<29), WithNoFallbackOnError(true))
		assert.NoError(t, err)
		assert.NoError(t, dq.Close())

		_, err = f.WriteString("1234")
		assert.NoError(t, err)
		assert.NoError(t, f.Sync())

		dq, err = Open(WithPath(p), WithCapacity(1<<29), WithNoFallbackOnError(true))
		assert.NoError(t, err)
		assert.Equal(t, int64(0), dq.pos.Seek)
		assert.NoError(t, dq.Close())

		_, err = f.WriteString("5678")
		assert.NoError(t, err)
		assert.NoError(t, f.Sync())

		dq, err = Open(WithPath(p), WithCapacity(1<<29), WithNoFallbackOnError(true))
		assert.NoError(t, err)
		assert.Equal(t, int64(0), dq.pos.Seek)
		assert.NoError(t, dq.Close())

		assert.NoError(t, f.Close())
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

func TestClosedCacheRejectsPutAfterOwnershipTransfer(t *T.T) {
	p := t.TempDir()
	c1, err := Open(WithPath(p), WithNoSync(true), WithNoPos(true))
	require.NoError(t, err)
	require.NoError(t, c1.Put([]byte("before-close")))
	require.NoError(t, c1.Close())

	c2, err := Open(WithPath(p), WithNoSync(true), WithNoPos(true))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, c2.Close())
		ResetMetrics()
	})

	require.NoError(t, c2.Put([]byte("current-owner")))
	require.ErrorIs(t, c1.Put([]byte("after-close")), ErrClosed)
	require.NoError(t, c2.Rotate())

	var got [][]byte
	for {
		err := c2.Get(func(data []byte) error {
			got = append(got, append([]byte(nil), data...))
			return nil
		})
		if errors.Is(err, ErrNoData) {
			break
		}
		require.NoError(t, err)
	}

	require.Equal(t, [][]byte{[]byte("before-close"), []byte("current-owner")}, got)
}

func TestClosedCacheRejectsOperations(t *T.T) {
	t.Cleanup(ResetMetrics)

	tests := []struct {
		name string
		do   func(*DiskCache) error
	}{
		{
			name: "put",
			do: func(c *DiskCache) error {
				return c.Put([]byte("data"))
			},
		},
		{
			name: "stream-put",
			do: func(c *DiskCache) error {
				return c.StreamPut(strings.NewReader("data"), len("data"))
			},
		},
		{
			name: "stream-put-invalid-size",
			do: func(c *DiskCache) error {
				return c.StreamPut(strings.NewReader(""), 0)
			},
		},
		{
			name: "get",
			do: func(c *DiskCache) error {
				return c.Get(nil)
			},
		},
		{
			name: "buf-callback-get",
			do: func(c *DiskCache) error {
				return c.BufCallbackGet(func() []byte { return make([]byte, 16) }, nil)
			},
		},
		{
			name: "buf-get",
			do: func(c *DiskCache) error {
				return c.BufGet(make([]byte, 16), nil)
			},
		},
		{
			name: "rotate",
			do: func(c *DiskCache) error {
				return c.Rotate()
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *T.T) {
			c, err := Open(WithPath(t.TempDir()), WithNoSync(true))
			require.NoError(t, err)
			require.NoError(t, c.Close())

			dataPath := filepath.Join(c.Path(), "data")
			beforeData, err := os.Stat(dataPath)
			require.NoError(t, err)
			beforeFDs, canCountFDs := countOpenFileDescriptors(c.Path())

			require.ErrorIs(t, tc.do(c), ErrClosed)

			afterData, err := os.Stat(dataPath)
			require.NoError(t, err)
			require.Equal(t, beforeData.Size(), afterData.Size())
			if canCountFDs {
				afterFDs, ok := countOpenFileDescriptors(c.Path())
				require.True(t, ok)
				require.Equal(t, beforeFDs, afterFDs)
			}
		})
	}
}

func countOpenFileDescriptors(path string) (int, bool) {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return 0, false
	}

	prefix := filepath.Clean(path) + string(os.PathSeparator)
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if err != nil {
			continue
		}

		target = strings.TrimSuffix(target, " (deleted)")
		if strings.HasPrefix(filepath.Clean(target), prefix) {
			count++
		}
	}

	return count, true
}

func TestRepeatedCloseDoesNotReleaseNextOwnerLock(t *T.T) {
	p := t.TempDir()
	c1, err := Open(WithPath(p))
	require.NoError(t, err)
	require.NoError(t, c1.Close())

	c2, err := Open(WithPath(p))
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, c2.Close())
		ResetMetrics()
	})

	require.NoError(t, c1.Close())
	c3, err := Open(WithPath(p))
	if c3 != nil {
		assert.NoError(t, c3.Close())
	}
	require.Error(t, err)
}

func TestCloseReleasesOwnershipAfterDescriptorError(t *T.T) {
	p := t.TempDir()
	c, err := Open(WithPath(p), WithNoSync(true))
	require.NoError(t, err)
	require.NoError(t, c.Put([]byte("data")))
	require.NoError(t, c.Rotate())
	require.NoError(t, c.Get(nil))

	readFD := c.rfd
	writeFD := c.wfd
	posFD := c.pos.fd
	require.NotNil(t, readFD)
	require.NotNil(t, writeFD)
	require.NotNil(t, posFD)
	require.NoError(t, readFD.Close())

	t.Cleanup(func() {
		_ = writeFD.Close()
		_ = posFD.Close()
		if c.flock != nil {
			_ = c.flock.unlock()
		}
		ResetMetrics()
	})

	closeErr := c.Close()
	require.Error(t, closeErr)
	require.EqualError(t, c.Close(), closeErr.Error())
	_, err = writeFD.WriteString("must-be-closed")
	require.Error(t, err)
	_, err = posFD.WriteString("must-be-closed")
	require.Error(t, err)

	c2, err := Open(WithPath(p))
	require.NoError(t, err)
	require.NoError(t, c2.Close())
}

func TestCloseAggregatesDescriptorErrors(t *T.T) {
	p := t.TempDir()
	c, err := Open(WithPath(p), WithNoSync(true))
	require.NoError(t, err)
	require.NoError(t, c.Put([]byte("data")))
	require.NoError(t, c.Rotate())
	require.NoError(t, c.Get(nil))

	require.NoError(t, c.rfd.Close())
	require.NoError(t, c.wfd.Close())
	require.NoError(t, c.pos.fd.Close())

	closeErr := c.Close()
	require.Error(t, closeErr)
	require.Contains(t, closeErr.Error(), "fd_type=read_fd")
	require.Contains(t, closeErr.Error(), "fd_type=write_fd")
	require.Contains(t, closeErr.Error(), "failed_to_close_position_file")

	c2, err := Open(WithPath(p))
	require.NoError(t, err)
	require.NoError(t, c2.Close())
	ResetMetrics()
}
