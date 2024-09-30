// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkNosyncPutGet(b *T.B) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(Metrics()...)

	p := b.TempDir()
	c, err := Open(WithPath(p), WithNoSync(true), WithBatchSize(1024*1024*4), WithCapacity(4*1024*1024*1024))
	require.NoError(b, err)

	_1mb := make([]byte, 1024*1024)
	_1kb := make([]byte, 1024)
	_512kb := make([]byte, 1024*512)

	b.Run(`put-1mb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_1mb)
		}
	})

	b.Run(`put-1kb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_1kb)
		}
	})

	b.Run(`put-512kb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_512kb)
		}
	})

	b.Run(`get`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Get(func(_ []byte) error { return nil })
		}
	})

	mfs, err := reg.Gather()
	require.NoError(b, err)

	b.Cleanup(func() {
		assert.NoError(b, c.Close())
		ResetMetrics()
	})
}

func BenchmarkPutGet(b *T.B) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(Metrics()...)

	p := b.TempDir()
	c, err := Open(WithPath(p), WithBatchSize(1024*1024*4), WithCapacity(4*1024*1024*1024))
	require.NoError(b, err)

	_1mb := make([]byte, 1024*1024)
	_1kb := make([]byte, 1024)
	_512kb := make([]byte, 1024*512)

	b.Run(`put-1mb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_1mb)
		}
	})

	b.Run(`put-1kb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_1kb)
		}
	})

	b.Run(`put-512kb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_512kb)
		}
	})

	b.Run(`get`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Get(func(_ []byte) error { return nil })
		}
	})

	b.Run(`buf-get`, func(b *T.B) {
		buf := bytes.NewBuffer(nil)
		for i := 0; i < b.N; i++ {
			c.BufGet(buf, nil)
			buf.Reset()
		}
	})

	b.Run(`buf-get-with-callback`, func(b *T.B) {
		buf := bytes.NewBuffer(nil)
		for i := 0; i < b.N; i++ {
			c.BufGet(buf, func(_ []byte) error {
				return nil
			})
			buf.Reset()
		}
	})

	mfs, err := reg.Gather()
	require.NoError(b, err)

	b.Cleanup(func() {
		assert.NoError(b, c.Close())
		ResetMetrics()
	})
}

func TestConcurrentPutGet(t *T.T) {
	var (
		p      = t.TempDir()
		mb     = int64(1024 * 1024)
		sample = make([]byte, 5*7351)
		eof    = 0
		reg    = prometheus.NewRegistry()
	)

	reg.MustRegister(Metrics()...)

	c, err := Open(WithPath(p), WithBatchSize(4*mb), WithCapacity(128*mb))
	assert.NoError(t, err)

	defer c.Close()

	wg := sync.WaitGroup{}
	concurrency := 4

	fnPut := func(idx int) {
		defer wg.Done()
		nput := 0
		exceed100ms := 0

		for {
			start := time.Now()
			assert.NoError(t, c.Put(sample))

			cost := time.Since(start)
			if cost > 100*time.Millisecond {
				exceed100ms++
			}

			nput++

			if nput > 1000 {
				t.Logf("[%d] Put exit", idx)
				return
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go fnPut(i)
	}

	fnGet := func(idx int) {
		defer wg.Done()
		nget := 0
		readBytes := 0
		exceed100ms := 0

		for {
			nget++
			start := time.Now()
			if err := c.Get(func(x []byte) error {
				assert.Equal(t, sample, x)
				readBytes += len(x)

				cost := time.Since(start)
				if cost > 100*time.Millisecond {
					exceed100ms++
				}

				return nil
			}); err != nil {
				if errors.Is(err, ErrEOF) {
					time.Sleep(time.Second)
					eof++
					if eof >= 10 {
						break
					}
				} else {
					t.Logf("[%d]: %s", idx, err)
					time.Sleep(time.Second)
				}
			} else {
				eof = 0 // reset eof if Get ok
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go fnGet(i)
	}

	wg.Wait()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	t.Cleanup(func() {
		assert.NoError(t, c.Close())
		ResetMetrics()
	})
}

func TestPutOnCapacityReached(t *T.T) {
	t.Run(`reach-capacity-single-put`, func(t *T.T) {
		var (
			mb       = int64(1024 * 1024)
			p        = t.TempDir()
			capacity = 32 * mb
			large    = make([]byte, mb)
			small    = make([]byte, 1024*3)
			maxPut   = 4 * capacity
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		t.Logf("path: %s", p)

		c, err := Open(WithPath(p), WithCapacity(capacity), WithBatchSize(4*mb))
		assert.NoError(t, err)

		putBytes := 0

		n := 0
		for {
			switch n % 2 {
			case 0:
				c.Put(small)
				putBytes += len(small)
			case 1:
				c.Put(large)
				putBytes += len(large)
			}
			n++

			if int64(putBytes) > maxPut {
				break
			}
		}

		mfs, err := reg.Gather()
		require.NoError(t, err)

		m := metrics.GetMetricOnLabels(mfs,
			"diskcache_dropped_total",
			c.path,
			reasonExceedCapacity)

		require.NotNil(t, m, "got metrics:\n%s", metrics.MetricFamily2Text(mfs))
		assert.True(t, m.GetCounter().GetValue() > 0.0)

		t.Cleanup(func() {
			require.NoError(t, c.Close())
			ResetMetrics()
		})
	})

	t.Run(`reach-capacity-concurrent-put`, func(t *T.T) {
		var (
			mb       = int64(1024 * 1024)
			p        = t.TempDir()
			capacity = 128 * mb
			large    = make([]byte, mb)
			small    = make([]byte, 1024*3)
			maxPut   = 4 * capacity
			wg       sync.WaitGroup
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		t.Logf("path: %s", p)

		c, err := Open(WithPath(p), WithCapacity(capacity), WithBatchSize(4*mb))
		assert.NoError(t, err)

		total := int64(0)

		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()
				n := 0
				for {
					switch n % 2 {
					case 0:
						c.Put(small)
						atomic.AddInt64(&total, 1024*3)
					case 1:
						c.Put(large)
						atomic.AddInt64(&total, mb)
					}
					n++

					if atomic.LoadInt64(&total) > maxPut {
						return
					}
				}
			}()
		}

		wg.Wait()

		mfs, err := reg.Gather()
		require.NoError(t, err)

		m := metrics.GetMetricOnLabels(mfs,
			"diskcache_dropped_total",
			c.path,
			reasonExceedCapacity)
		require.NotNil(t, m, "got metrics:\n%s", metrics.MetricFamily2Text(mfs))

		assert.True(t, m.GetCounter().GetValue() > 0.0)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			ResetMetrics()
		})
	})
}

func TestStreamPut(t *T.T) {
	t.Run("basic", func(t *T.T) {
		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		raw := "0123456789"
		r := strings.NewReader(raw)

		var (
			p = t.TempDir()
		)

		t.Logf("path: %s", p)

		c, err := Open(WithPath(p), WithStreamSize(2))
		assert.NoError(t, err)

		assert.NoError(t, c.StreamPut(r, len(raw)))
		assert.NoError(t, c.Rotate())

		assert.NoError(t, c.Get(func(data []byte) error {
			assert.Equal(t, []byte(raw), data)
			return nil
		}))

		mfs, err := reg.Gather()
		require.NoError(t, err)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			ResetMetrics()
		})
	})
}
