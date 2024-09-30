// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPut(t *T.T) {
	t.Run(`buf-get`, func(t *T.T) {
		testDir := t.TempDir()
		err := os.MkdirAll(testDir, 0o755)
		assert.NoError(t, err)

		dq, err := Open(WithPath(testDir), WithCapacity(1<<30))
		assert.NoError(t, err)

		raw := []byte("hello message-1")
		assert.NoError(t, dq.Put(raw))

		buf := bytes.NewBuffer(nil)

		for {
			if err := dq.BufGet(buf, nil); err != nil {
				t.Log(time.Now().Format(time.RFC3339Nano),
					" fail to get message: ", err)
				time.Sleep(time.Second * 1)
			} else {
				assert.Equal(t, raw, buf.Bytes())
				break
			}
		}

		raw2 := []byte("hello message-2")
		assert.NoError(t, dq.Put(raw2))

		ok := false
		buf.Reset()

		for i := 0; i < 10; i++ {
			if err := dq.BufGet(buf, func(msg []byte) error {
				t.Logf("get message: %q\n", string(msg))
				ok = true
				return nil
			}); err != nil {
				t.Log(time.Now().Format(time.RFC3339Nano), " fail to get message: ", err)
				time.Sleep(time.Second * 1)
			} else {
				break
			}
		}

		assert.True(t, ok, "expected consume 1 message in 10 seconds, but got no message")

		assert.NoError(t, dq.Close())
	})

	t.Run(`basic`, func(t *T.T) {
		testDir := t.TempDir()
		err := os.MkdirAll(testDir, 0o755)
		assert.NoError(t, err)

		dq, err := Open(WithPath(testDir), WithCapacity(1<<30))
		assert.NoError(t, err)

		assert.NoError(t, dq.Put([]byte("hello message-1")))

		for {
			if err := dq.Get(func(msg []byte) error {
				t.Logf("get message: %q\n", string(msg))
				return nil
			}); err != nil {
				t.Log(time.Now().Format(time.RFC3339Nano), " fail to get message: ", err)
				time.Sleep(time.Second * 1)
			} else {
				break
			}
		}

		assert.NoError(t, dq.Put([]byte("hello message-2")))

		ok := false

		for i := 0; i < 10; i++ {
			if err := dq.Get(func(msg []byte) error {
				t.Logf("get message: %q\n", string(msg))
				ok = true
				return nil
			}); err != nil {
				t.Log(time.Now().Format(time.RFC3339Nano), " fail to get message: ", err)
				time.Sleep(time.Second * 1)
			} else {
				break
			}
		}

		assert.True(t, ok, "expected consume 1 message in 10 seconds, but got no message")

		assert.NoError(t, dq.Close())
	})
}

func TestDropInvalidDataFile(t *T.T) {
	t.Run(`get-on-0bytes-data-file`, func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		require.NoError(t, err)

		// put some data and rotate 10 datafiles
		data := make([]byte, 100)
		for i := 0; i < 10; i++ {
			assert.NoError(t, c.Put(data))
			assert.NoError(t, c.rotate())

			// destroy the datafile
			if i%2 == 0 {
				assert.NoError(t, os.Truncate(c.dataFiles[i], 0))
			}
		}

		assert.Len(t, c.dataFiles, 10)

		round := 0
		for {
			err := c.Get(func(get []byte) error {
				// switch to 2nd file
				assert.Equalf(t, data, get, "at round %d, get %d bytes",
					round, len(get))
				return nil
			})
			if err != nil {
				require.ErrorIs(t, err, ErrEOF)
				break
			}
			round++
		}

		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)
		mfs, err := reg.Gather()
		require.NoError(t, err)

		assert.Equalf(t, float64(5),
			metrics.GetMetricOnLabels(mfs,
				"diskcache_dropped_total",
				c.path,
				reasonBadDataFile,
			).GetCounter().GetValue(),
			"got metrics\n%s", metrics.MetricFamily2Text(mfs))
	})
}

func TestFallbackOnError(t *T.T) {
	t.Run(`get-erro-on-EOF`, func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		require.NoError(t, err)

		// put some data
		data := make([]byte, 100)
		assert.NoError(t, c.Put(data))

		assert.NoError(t, c.rotate())

		require.NoError(t, c.Get(func(_ []byte) error {
			return nil // ignore the data
		}))

		err = c.Get(func(_ []byte) error {
			assert.True(t, 1 == 2) // should not been here
			return nil
		})

		assert.ErrorIs(t, err, ErrEOF)
		t.Logf("get: %s", err)

		if errors.Is(err, ErrEOF) {
			t.Logf("we should ignore the error")
		}
	})

	t.Run(`fallback-on-error`, func(t *T.T) {
		ResetMetrics()

		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		data := make([]byte, 100)
		assert.NoError(t, c.Put(data))

		assert.NoError(t, c.rotate())

		// should get error when callback fail
		require.Error(t, c.Get(func(_ []byte) error {
			return fmt.Errorf("get error")
		}))

		assert.Equal(t, int64(0), c.pos.Seek)

		// should no error when callback ok
		assert.NoError(t, c.Get(func(x []byte) error {
			assert.Equal(t, data, x)
			return nil
		}))

		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)
		mfs, err := reg.Gather()
		require.NoError(t, err)

		assert.Equalf(t, float64(1),
			metrics.GetMetricOnLabels(mfs, "diskcache_seek_back_total", c.path).GetCounter().GetValue(),
			"got metrics\n%s", metrics.MetricFamily2Text(mfs))

		t.Cleanup(func() {
			ResetMetrics()
		})
	})

	t.Run(`fallback-on-eof-error`, func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		// while on EOF, Fn error ignored
		assert.ErrorIs(t, c.Get(func(_ []byte) error {
			return fmt.Errorf("get error")
		}), ErrEOF)

		// still got EOF
		assert.ErrorIs(t, c.Get(func(x []byte) error {
			return nil
		}), ErrEOF)
	})

	t.Run(`no-fallback-on-error`, func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p), WithNoFallbackOnError(true))
		assert.NoError(t, err)

		data := make([]byte, 100)
		assert.NoError(t, c.Put(data))

		assert.NoError(t, c.rotate())

		c.Get(func(_ []byte) error {
			return fmt.Errorf("get error")
		})

		assert.ErrorIs(t, c.Get(func(x []byte) error {
			return nil
		}), ErrEOF)
	})
}

func TestPutGet(t *T.T) {
	t.Run(`clean-pos-on-eof`, func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		data := make([]byte, 100)
		if err := c.Put(data); err != nil {
			t.Error(err)
		}

		assert.NoError(t, c.rotate())
		assert.NoError(t, c.Get(func(data []byte) error { return nil }))
		assert.Error(t, c.Get(func(data []byte) error { return nil })) // EOF

		pos, err := posFromFile(c.pos.fname)
		assert.NoError(t, err)

		t.Logf("pos: %s", pos)

		t.Cleanup(func() {
			c.Close()
			ResetMetrics()
		})
	})

	t.Run("put-get", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		str := "hello world"
		if err := c.Put([]byte(str)); err != nil {
			t.Error(err)
		}

		assert.Equal(t, int64(len(str)+dataHeaderLen), c.curBatchSize)

		if err := c.Get(func(data []byte) error {
			t.Logf("get: %s", string(data))
			return nil
		}); err != nil {
			t.Logf("get: %s", err)
		}

		t.Cleanup(func() {
			c.Close()
			os.RemoveAll(p)
		})
	})

	t.Run(`get-without-pos`, func(t *T.T) {
		p := t.TempDir()

		kbdata := make([]byte, 1024)

		c, err := Open(WithPath(p), WithNoPos(true))
		assert.NoError(t, err)

		for i := 0; i < 10; i++ { // write 10kb
			require.NoError(t, c.Put(kbdata), "cache: %s", c)
		}

		// force rotate
		assert.NoError(t, c.rotate())

		// read 2 cache
		assert.NoError(t, c.Get(func(data []byte) error {
			assert.Len(t, data, len(kbdata))
			return nil
		}))

		assert.NoError(t, c.Get(func(data []byte) error {
			assert.Len(t, data, len(kbdata))
			return nil
		}))

		// close the cache for next re-Open()
		assert.NoError(t, c.Close())

		c2, err := Open(WithPath(p), WithNoPos(true))
		assert.NoError(t, err)
		defer c2.Close()

		require.Lenf(t, c2.dataFiles, 1, "cache: %s", c2)

		var ncached int
		for {
			if err := c2.Get(func(_ []byte) error {
				ncached++
				return nil
			}); err != nil {
				if errors.Is(err, ErrEOF) {
					t.Logf("cache EOF")
					break
				} else {
					assert.NoError(t, err)
				}
			}
		}

		// without .pos, still got 10 cache
		assert.Equal(t, 10, ncached, "cache: %s", c2)
	})

	t.Run(`get-with-pos`, func(t *T.T) {
		p := t.TempDir()

		kbdata := make([]byte, 1024)

		c, err := Open(WithPath(p),
			WithCapacity(int64(len(kbdata)*10)),
			WithBatchSize(int64(len(kbdata)*2)))
		assert.NoError(t, err)

		for i := 0; i < 10; i++ { // write 10kb
			require.NoError(t, c.Put(kbdata), "cache: %s", c)
		}

		// create a read pos
		assert.NoError(t, c.Get(func(data []byte) error {
			assert.Len(t, data, len(kbdata))
			return nil
		}))
		assert.Equal(t, int64(len(kbdata)+dataHeaderLen), c.pos.Seek)
		assert.NoError(t, c.Close())

		_, err = os.Stat(c.pos.fname)
		require.NoError(t, err)

		// reopen the cache
		c2, err := Open(WithPath(p),
			WithCapacity(int64(len(kbdata)*10)),
			WithBatchSize(int64(len(kbdata)*2)))
		require.NoError(t, err, "get error: %s", err)
		assert.Equal(t, c2.pos.Seek, int64(len(kbdata)+dataHeaderLen))

		assert.NoError(t, c2.Get(func(data []byte) error {
			assert.Len(t, data, len(kbdata))
			return nil
		}))
		assert.Equal(t, int64(len(kbdata)+dataHeaderLen), c.pos.Seek)
		assert.NoError(t, c2.Close())
		assert.Equal(t, c2.pos.Seek, int64(2*(len(kbdata)+dataHeaderLen)))

		t.Cleanup(func() {
			c2.Close()
			os.RemoveAll(p)
		})
	})
}
