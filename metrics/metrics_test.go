// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package metrics

import (
	"bytes"
	"path/filepath"
	"sync"
	T "testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/assert"
)

func TestHistogram(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		vec := prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: filepath.Base(t.Name()),
			},
			[]string{"api", "status"},
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(vec)

		div := 10000.0

		for i := 0; i < 1000; i++ {
			switch i % 3 {
			case 0:
				vec.WithLabelValues("/v1/write/metric", "ok").Observe(float64(i) / div)
			case 1:
				vec.WithLabelValues("/v1/write/logging", "ok").Observe(float64(i) / div)
			default:
				vec.WithLabelValues("/v1/write/tracing", "ok").Observe(float64(i) / div)
			}
		}

		mfs, err := reg.Gather()
		assert.NoError(t, err)
		buf := bytes.NewBuffer(nil)
		for _, mf := range mfs {
			expfmt.MetricFamilyToText(buf, mf)
		}
		t.Logf("text:\n%s", buf.String())

		buf = bytes.NewBuffer(nil)
		for _, mf := range mfs {
			expfmt.MetricFamilyToOpenMetrics(buf, mf)
		}
		t.Logf("open metrics:\n%s", buf.String())
	})
}

func TestConcurrentAdd(t *T.T) {
	t.Run(`concurrent_set`, func(t *T.T) {
		vec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: filepath.Base(t.Name()),
			},
			[]string{"api", "status"},
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(vec)

		var (
			nwrk = 10
			wg   sync.WaitGroup
			max  = 10
		)

		wg.Add(nwrk)
		for i := 0; i < nwrk; i++ {
			go func() {
				defer wg.Done()

				n := 0
				for {
					vec.WithLabelValues("/v1/write/abc", "ok").Add(1.0)
					n++
					if n >= max {
						return
					}
				}
			}()
		}

		wg.Wait()
		mfs, err := reg.Gather()
		assert.NoError(t, err)

		m := GetMetricOnLabels(mfs, filepath.Base(t.Name()), `/v1/write/abc`, `ok`)
		assert.NotNil(t, m)
		assert.Equal(t, float64(nwrk*max), m.GetCounter().GetValue())
	})
}

func TestAdd(t *T.T) {
	t.Run(`add_count`, func(t *T.T) {
		vec := prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Help: "not-set",
				Name: filepath.Base(t.Name()),
			},
			[]string{"api", "status"},
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(vec)
		for i := 0; i < 100; i++ {
			vec.WithLabelValues("/v1/write/abc", "ok").Observe(float64(i))
			vec.WithLabelValues("/v1/write/def", "fail").Observe(float64(100 - i))
		}

		mfs, err := reg.Gather()
		assert.NoError(t, err)

		for _, mf := range mfs {
			t.Logf("metric: %s", mf.String())
		}

		m := GetMetricOnLabels(mfs, filepath.Base(t.Name()), "/v1/write/abc", "ok")
		assert.NotNil(t, m)
		assert.Equal(t, uint64(100), m.GetSummary().GetSampleCount())

		m = GetMetricOnLabels(mfs, filepath.Base(t.Name()), "/v1/write/def", "fail")
		assert.NotNil(t, m)
		assert.Equal(t, uint64(100), m.GetSummary().GetSampleCount())
	})
}

func BenchmarkAddValue(b *T.B) {
	b.Run(`summary_with_2_labels`, func(b *T.B) {
		b.StopTimer()
		vec := prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace: "ns1",
				Subsystem: "subsys",
				Help:      "not-set",
				Name:      filepath.Base(b.Name()),
			},
			[]string{"api", "status"},
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(vec)

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			vec.WithLabelValues("/v1/write/abc", "ok").Observe(123.4)
			vec.WithLabelValues("/v1/write/def", "fail").Observe(456.7)
		}

		b.StopTimer()

		mfs, err := reg.Gather()
		assert.NoError(b, err)
		for _, mf := range mfs {
			b.Logf("metric: %s", mf.String())
		}
	})

	b.Run(`count_with_2_labels`, func(b *T.B) {
		b.StopTimer()
		vec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ns1",
				Subsystem: "subsys",
				Help:      "not-set",
				Name:      filepath.Base(b.Name()),
			},
			[]string{"api", "status"},
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(vec)

		b.StartTimer()
		for i := 0; i < b.N; i++ {
			vec.WithLabelValues("/v1/write/a", "ok").Add(123.4)
			vec.WithLabelValues("/v1/write/b", "fail").Add(456.7)
		}

		b.StopTimer()

		mfs, err := reg.Gather()
		assert.NoError(b, err)
		for _, mf := range mfs {
			b.Logf("metric: %s", mf.String())
		}
	})

	b.Run("para", func(b *T.B) {
		b.StopTimer()
		vec := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "ns1",
				Subsystem: "subsys",
				Name:      filepath.Base(b.Name()),
			},
			[]string{"api", "status"},
		)

		reg := prometheus.NewRegistry()
		reg.MustRegister(vec)

		b.StartTimer()
		b.RunParallel(func(pb *T.PB) {
			for pb.Next() {
				vec.WithLabelValues("/v1/write/a", "ok").Add(123.4)
				vec.WithLabelValues("/v1/write/b", "fail").Add(456.7)
			}
		})

		b.StopTimer()

		mfs, err := reg.Gather()
		assert.NoError(b, err)
		for _, mf := range mfs {
			b.Logf("metric: %s", mf.String())
		}
	})
}
