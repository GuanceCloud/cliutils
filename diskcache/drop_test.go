// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"errors"
	"sync"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDropBatch(t *T.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(Metrics()...)

	p := t.TempDir()
	capacity := int64(32 * 1024 * 1024)
	c, err := Open(WithPath(p),
		WithBatchSize(4*1024*1024),
		WithCapacity(capacity))
	if err != nil {
		t.Error(err)
		return
	}

	sample := bytes.Repeat([]byte("hello"), 7351)
	n := 0
	for {
		if err := c.Put(sample); err != nil {
			t.Error(err)
		}

		n++

		if int64(n*len(sample)) > capacity {
			break
		}
	}

	mfs, err := reg.Gather()
	assert.NoError(t, err)

	m := metrics.GetMetricOnLabels(mfs,
		"diskcache_dropped_data",
		c.path,
		reasonExceedCapacity)

	require.NotNil(t, m, "got metrics\n%s", metrics.MetricFamily2Text(mfs))

	assert.Equal(t,
		uint64(1),
		m.GetSummary().GetSampleCount(),
		"got metrics\n%s", metrics.MetricFamily2Text(mfs))

	t.Cleanup(func() {
		assert.NoError(t, c.Close())
		ResetMetrics()
	})
}

func TestDropDuringGet(t *T.T) {
	reg := prometheus.NewRegistry()
	reg.MustRegister(Metrics()...)

	p := t.TempDir()
	capacity := int64(2 * 1024 * 1024)
	c, err := Open(WithPath(p), WithBatchSize(1*1024*1024), WithCapacity(capacity))
	assert.NoError(t, err)

	sample := bytes.Repeat([]byte("hello"), 7351)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { // fast put
		defer wg.Done()
		n := 0
		for {
			if err := c.Put(sample); err != nil {
				t.Error(err)
				return
			}
			n++

			if int64(n*len(sample)) > capacity {
				return
			}
		}
	}()

	time.Sleep(time.Second) // wait new data write

	// slow get
	i := 0
	eof := 0
	for {
		time.Sleep(time.Millisecond * 100)
		if err := c.Get(func(x []byte) error {
			assert.Equal(t, sample, x)
			return nil
		}); err != nil {
			if errors.Is(err, ErrNoData) {
				t.Logf("[%s] read: %s", time.Now(), err)
				time.Sleep(time.Second)
				eof++
				if eof > 5 {
					break
				}
			} else {
				t.Logf("%s", err)
				time.Sleep(time.Second)
			}
		} else {
			eof = 0 // reset EOF if put ok
		}
		i++
	}

	wg.Wait()

	t.Cleanup(func() {
		assert.NoError(t, c.Close())
		ResetMetrics()
	})
}
