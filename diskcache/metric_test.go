// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	T "testing"

	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
