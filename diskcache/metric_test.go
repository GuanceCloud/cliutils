// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"math/rand"
	"sync"
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
	//log.Printf("get %s bytes from %s", humanize.SI(float64(n), ""), humanize.SI(float64(start), ""))

	if start+n > len(data) {
		return data[len(data)-n:] // return last n bytes
	} else {
		return data[start : start+n]
	}
}

func TestDropMetric(t *T.T) {
	t.Skip()
	t.Run("drop", func(t *T.T) {
		reg := prometheus.NewRegistry()
		register(reg)

		p := t.TempDir()
		bsize := int64(1024)
		capacity := bsize * 2

		c, err := Open(WithPath(p), WithBatchSize(1024), WithCapacity(capacity))
		assert.NoError(t, err)

		data := make([]byte, 512)

		wg := sync.WaitGroup{}

		wg.Add(2)
		go func() {
			defer wg.Done()
			for {
				c.Put(getSamples(data))
			}
		}()

		go func() {
			defer wg.Done()
			for {
				c.Get(nil)
			}
		}()

		for {
			time.Sleep(time.Second * 3)

			mfs, err := reg.Gather()
			assert.NoError(t, err)

			t.Logf("\n%s", metrics.MetricFamily2Text(mfs))
			m := metrics.GetMetricOnLabels(mfs, "diskcache_size", c.labels...)
			require.NotNil(t, m)
			require.True(t, m.GetGauge().GetValue() >= 0.0, "got size %f", m.GetCounter().GetValue())
		}

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			resetMetrics()
		})
	})
}

func TestMetric(t *T.T) {

	t.Run("basic", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		smallBytes := make([]byte, 100)

		assert.NoError(t, c.Put(smallBytes))

		mfs, err := metrics.Gather()
		assert.NoError(t, err)

		t.Logf("\n%s", metrics.MetricFamily2Text(mfs))

		m := metrics.GetMetricOnLabels(mfs, "diskcache_put_total", c.labels...)
		require.NotNil(t, m, "labels: %+#v", c.labels)
		assert.Equal(t, float64(1), m.GetCounter().GetValue())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_put_bytes_total", c.labels...)
		require.NotNil(t, m)
		assert.Equal(t, float64(100), /* dataHeaderLen not counted in put_bytes */
			m.GetCounter().GetValue())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_size", c.labels...)
		require.NotNil(t, m)
		assert.Equal(t, float64(104), /* dataHeaderLen counted in size */
			m.GetGauge().GetValue())

		// these fileds all zero
		m = metrics.GetMetricOnLabels(mfs, "diskcache_dropped_batch", c.labels...)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get", c.labels...)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_bytes_total", c.labels...)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_latency", c.labels...)
		require.Nil(t, m)

		m = metrics.GetMetricOnLabels(mfs, "diskcache_rotate", c.labels...)
		require.Nil(t, m)

		// rotate to make it readble
		assert.NoError(t, c.rotate())
		assert.NoError(t, c.Get(nil))

		mfs, err = metrics.Gather()
		assert.NoError(t, err)
		t.Logf("\n%s", metrics.MetricFamily2Text(mfs))

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_total", c.labels...)
		require.NotNil(t, m)
		assert.Equal(t, float64(1), m.GetCounter().GetValue())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_get_bytes_total", c.labels...)
		require.NotNil(t, m)
		assert.Equal(t, float64(100), m.GetCounter().GetValue())

		m = metrics.GetMetricOnLabels(mfs, "diskcache_size", c.labels...)
		require.NotNil(t, m)
		assert.Equal(t, float64(100+dataHeaderLen+4 /*EOFHint*/), m.GetGauge().GetValue())

		assert.NoError(t, c.Close())
		mfs, err = metrics.Gather()
		assert.NoError(t, err)
		t.Logf("\n%s", metrics.MetricFamily2Text(mfs))

		// check if open/close time metric exist.
		m = metrics.GetMetricOnLabels(mfs, "diskcache_last_close_time", c.labels...)
		require.NotNil(t, m)
		m = metrics.GetMetricOnLabels(mfs, "diskcache_open_time", c.labels...)
		require.NotNil(t, m)

		t.Cleanup(func() {
			resetMetrics()
		})
	})
}
