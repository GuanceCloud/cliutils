// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"math"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func Test_bufStr(t *T.T) {
	t.Run("basic", func(t *T.T) {
		pt := New()

		assert.Equal(t, "abc", pt.bufStr("abc"))
		t.Logf("len(buf): %d, cap(buf): %d", len(pt.buf), cap(pt.buf))
	})

	t.Run("loop", func(t *T.T) {
		pt := emptyPoint()

		for i := 0; i < 100; i++ {
			randStr := cliutils.CreateRandomString(i)
			assert.Equal(t, randStr, pt.bufStr(randStr))
		}
		t.Logf("len(buf): %d, cap(buf): %d", len(pt.buf), cap(pt.buf))
	})
}

func TestAddKV(t *T.T) {
	t.Run("basic", func(t *T.T) {
		pt := New()

		pt.AddKV("f", 1.0, true)
		pt.AddKV("s", "hello", true, WithKVTagSet(true))
		pt.AddKV("i", 1, true)
		pt.AddKV("u", uint64(math.MaxUint64), true)
		pt.AddKV("b", false, true)
		pt.AddKV("d", []byte("world"), true)

		t.Logf("pt: %s", pt.Pretty())
	})

	t.Run("with-point-pool", func(t *T.T) {

		pp := NewReservedCapPointPool(100)
		SetPointPool(pp)
		t.Cleanup(func() {
			ClearPointPool()
		})

		pt := pp.Get()

		pt.AddKV("f", 1.0, true)
		pt.AddKV("s", "hello", true, WithKVTagSet(true))
		pt.AddKV("i", 1, true)
		pt.AddKV("u", uint64(math.MaxUint64), true)
		pt.AddKV("b", false, true)
		pt.AddKV("d", []byte("world"), true)

		t.Logf("pt: %s", pt.Pretty())
	})
}

func Test_pointSize(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		pp := NewReservedCapPointPool(100)

		SetPointPool(pp)

		reg := prometheus.NewRegistry()
		reg.MustRegister(Metrics()...)

		t.Cleanup(func() {
			reg.Unregister(pp)
			ClearPointPool()
		})

		pt := pp.Get()

		pt.AddKV("f", 1.0, true)
		pt.AddKV("s", "hello", true, WithKVTagSet(true))
		pt.AddKV("i", 1, true)
		pt.AddKV("u", uint64(math.MaxUint64), true)
		pt.AddKV("b", false, true)
		pt.AddKV("d", []byte("world"), true)

		pp.Put(pt)

		mfs, err := reg.Gather()
		assert.NoError(t, err)

		t.Logf("mfs: %s", metrics.MetricFamily2Text(mfs))
	})
}

func BenchmarkBufPool(b *T.B) {
	largeStr := cliutils.CreateRandomString((1 << 8))

	reg := prometheus.NewRegistry()

	pp := NewReservedCapPointPool(100)
	reg.MustRegister(Metrics()...)

	SetPointPool(pp)
	b.Cleanup(func() {
		reg.Unregister(pp)
		ClearPointPool()
	})

	b.ResetTimer()
	b.Run("with-buf-pool", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			pt := New().
				AddKV("s", "hello", true, WithKVTagSet(true)).
				AddKV("large-s", largeStr, true).
				SetName("with-buf-pool").
				SetTimestamp(time.Now().UnixNano()).
				Check()
			pp.Put(pt)
		}
	})

	b.ResetTimer()
	b.Run("without-buf-pool", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var kvs KVs

			kvs = kvs.AddV2("s", "hello", true, WithKVTagSet(true))
			kvs = kvs.AddV2("large-s", largeStr, true)
			pt := NewPointV2("without-buf-pool", kvs, WithPrecheck(false))

			pp.Put(pt)
		}
	})

	mfs, err := reg.Gather()
	assert.NoError(b, err)

	b.Logf("mfs: %s", metrics.MetricFamily2Text(mfs))
}

func TestCheckWithOptions(t *T.T) {
	t.Run(`multiple-check`, func(t *T.T) {
		pt := New().
			AddKV("nil", nil, true).
			AddKV("f1", 1.23, true).Check()

		t.Logf("pt: %s", pt.Pretty())

		pt = pt.cfg.check(pt)

		// the nil-value field still in point, should we remove the field after check?
		assert.Len(t, pt.pt.Warns, 3)

		t.Logf("pt: %s", pt.Pretty())
	})
}
