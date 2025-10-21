// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"math/rand"
	"sync"
	"sync/atomic"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// get random bytes from data
func getSamples(data []byte) []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// at least 1/10 of data
	n := (len(data)/10 + (r.Int() % len(data)))
	if n >= len(data) {
		n = len(data)
	}

	start := r.Int() % len(data)

	if start+n > len(data) {
		return data[len(data)-n:] // return last n bytes
	} else {
		return data[start : start+n]
	}
}

func TestPutGetMetrics(t *T.T) {
	t.Run("test-wakeup-count", func(t *T.T) {
		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		p := t.TempDir()
		c, err := Open(WithPath(p), WithWakeup(time.Second))
		require.NoError(t, err)

		data := make([]byte, 4)

		require.NoError(t, c.Put(data))

		time.Sleep(time.Second * 2)

		require.NoError(t, c.Get(nil))

		mfs, err := reg.Gather()
		assert.NoError(t, err)

		m := metrics.GetMetricOnLabels(mfs, "diskcache_wakeup_total", c.path)
		require.NotNil(t, m)
		require.Equal(t, 1.0, m.GetCounter().GetValue())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_rotate_total", c.path)
		require.NotNil(t, m)
		require.Equal(t, 1.0, m.GetCounter().GetValue())

		t.Cleanup(func() {
			ResetMetrics()
			assert.NoError(t, c.Close())
		})
	})

	// test if metric size ok
	t.Run("metrics-on-only-put", func(t *T.T) {
		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		p := t.TempDir()
		bsize := int64(100)

		c, err := Open(
			WithPath(p),
			WithBatchSize(bsize),
		)
		require.NoError(t, err)

		data := make([]byte, bsize/2)

		totalPut := 0
		for i := 0; i < 10; i++ {
			c.Put(data)
			totalPut += (len(data) + dataHeaderLen)
		}

		// check if size == totalPut
		mfs, err := reg.Gather()
		require.NoError(t, err)
		m := metrics.GetMetricOnLabels(mfs, "diskcache_size", c.path)
		require.NotNil(t, m)
		got := int(m.GetGauge().GetValue())
		assert.Equal(t, totalPut, got, "c.size: %d, size-expect=%d", c.size.Load(), got-totalPut)

		t.Cleanup(func() {
			ResetMetrics()
			assert.NoError(t, c.Close())
		})
	})

	t.Run("metrics-on-put-get", func(t *T.T) {
		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		p := t.TempDir()
		bsize := int64(100)

		c, err := Open(
			WithPath(p),
			WithBatchSize(bsize),
		)
		require.NoError(t, err)

		data := make([]byte, bsize/2)

		totalPutBytes := 0
		times := 10
		for i := 0; i < times; i++ {
			c.Put(data)
			totalPutBytes += len(data) // without dataHeaderLen
		}

		// force rotate
		assert.NoError(t, c.rotate())

		totalGet := 0
		for i := 0; i < times; i++ {
			c.Get(func(x []byte) error {
				totalGet += len(x)
				return nil
			})
		}

		c.Get(nil) // read EOF to tiger remove

		// check if size == totalPutBytes
		mfs, err := reg.Gather()
		require.NoError(t, err)
		m := metrics.GetMetricOnLabels(mfs, "diskcache_size", c.path)
		require.NotNil(t, m)
		got := int(m.GetGauge().GetValue())
		assert.Equal(t, 0, got, "c.size: %d", c.size.Load())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_bytes", c.path)
		require.NotNilf(t, m, "metrics:\n%s", c.path, metrics.MetricFamily2Text(mfs))
		assert.Equal(t, totalGet, int(m.GetSummary().GetSampleSum()))
		assert.Equal(t, totalGet, totalPutBytes)
		assert.Equal(t, uint64(times), m.GetSummary().GetSampleCount())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_latency", c.path)
		require.NotNil(t, m)
		assert.Equal(t, uint64(times), m.GetSummary().GetSampleCount())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_put_bytes", c.path)
		require.NotNil(t, m)
		assert.Equal(t, uint64(times), m.GetSummary().GetSampleCount())

		t.Cleanup(func() {
			ResetMetrics()
			assert.NoError(t, c.Close())
		})
	})
}

func TestMetric(t *T.T) {
	t.Run("basic", func(t *T.T) {
		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		smallBytes := make([]byte, 100)

		assert.NoError(t, c.Put(smallBytes))

		mfs, err := reg.Gather()

		assert.NoError(t, err)

		m := metrics.GetMetricOnLabels(mfs, "diskcache_put_bytes", c.path)
		require.NotNilf(t, m, "metrics:\n%s", c.path, metrics.MetricFamily2Text(mfs))
		assert.Equal(t, uint64(1), m.GetSummary().GetSampleCount())
		assert.Equal(t, float64(100), /* dataHeaderLen not counted in put_bytes */
			m.GetSummary().GetSampleSum())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_size", c.path)
		require.NotNil(t, m)
		assert.Equal(t, float64(104), /* dataHeaderLen counted in size */
			m.GetGauge().GetValue())

		// these fileds all zero
		m = metrics.GetMetricOnLabels(mfs, "diskcache_dropped_batch", c.path)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get", c.path)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_bytes_total", c.path)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_latency", c.path)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_rotate", c.path)
		require.Nil(t, m)

		// rotate to make it readble
		assert.NoError(t, c.rotate())
		assert.NoError(t, c.Get(nil))

		mfs, err = reg.Gather()
		assert.NoError(t, err)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_bytes", c.path)
		require.NotNil(t, m, "metrics:\n%s", c.path, metrics.MetricFamily2Text(mfs))
		assert.Equal(t, uint64(1), m.GetSummary().GetSampleCount())
		assert.Equal(t, float64(100), m.GetSummary().GetSampleSum())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_size", c.path)
		require.NotNil(t, m)
		assert.Equal(t, float64(100+dataHeaderLen /*EOFHint*/), m.GetGauge().GetValue())

		assert.NoError(t, c.Close())

		mfs, err = reg.Gather()
		assert.NoError(t, err)

		// check if open/close time metric exist.
		m = metrics.GetMetricOnLabels(mfs, "diskcache_last_close_time", c.path)
		require.NotNil(t, m, "metrics:\n%s", c.path, metrics.MetricFamily2Text(mfs))
		m = metrics.GetMetricOnLabels(mfs, "diskcache_open_time", c.labels...)
		require.NotNil(t, m)

		t.Cleanup(func() {
			ResetMetrics()
		})
	})
}

func TestPerfConcurrentPutGet(t *T.T) {
	p := t.TempDir()
	capacity := int64(1024 * 1024 * 1024)
	data := make([]byte, 1024*1024)

	t.Run("sing-put", func(t *T.T) {
		c, err := Open(
			WithPath(p),
			WithCapacity(capacity),
		)
		require.NoError(t, err)

		start := time.Now()
		tick := time.NewTicker(time.Second)
		defer tick.Stop()

		var total int64
		for {
			select {
			case <-tick.C:
				goto end
			default:
				x := getSamples(data)
				c.Put(x)
				total += int64(len(x))
			}
		}

	end:
		t.Logf("write perf(%d bytes): %d bytes/ms", total, total/int64(time.Since(start)/time.Millisecond))

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
		})
	})

	t.Run("multi-put", func(t *T.T) {
		c, err := Open(
			WithPath(p),
			WithCapacity(capacity),
		)
		require.NoError(t, err)

		start := time.Now()
		var total int64

		tick := time.NewTicker(time.Second)
		defer tick.Stop()

		// 10 worker
		wg := sync.WaitGroup{}
		wg.Add(10)
		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()
				for {
					select {
					case <-tick.C:
						return
					default:
						x := getSamples(data)
						c.Put(x)
						atomic.AddInt64(&total, int64(len(x)))
					}
				}
			}()
		}

		wg.Wait()

		t.Logf("write perf(%d bytes): %d bytes/ms", total, total/int64(time.Since(start)/time.Millisecond))

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
		})
	})

	t.Run("multi-put-get", func(t *T.T) {
		c, err := Open(
			WithPath(p),
			WithCapacity(capacity),
		)
		require.NoError(t, err)

		start := time.Now()
		var putTotal int64
		var getTotal int64

		tick := time.NewTicker(time.Second)
		defer tick.Stop()

		// put/get each 10 worker
		wg := sync.WaitGroup{}
		wg.Add(20)
		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()
				for {
					select {
					case <-tick.C:
						return
					default:
						x := getSamples(data)
						c.Put(x)
						atomic.AddInt64(&putTotal, int64(len(x)))
					}
				}
			}()
		}

		for i := 0; i < 10; i++ {
			go func() {
				defer wg.Done()
				for {
					select {
					case <-tick.C:
						return
					default:
						if err := c.Get(func(x []byte) error {
							atomic.AddInt64(&getTotal, int64(len(x)))
							return nil
						}); err != nil {
							return
						}
					}
				}
			}()
		}

		wg.Wait()

		t.Logf("write perf(%d bytes): put %d bytes/ms, get %dbytes/ms",
			putTotal,
			putTotal/int64(time.Since(start)/time.Millisecond),
			getTotal/int64(time.Since(start)/time.Millisecond),
		)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
		})
	})
}
