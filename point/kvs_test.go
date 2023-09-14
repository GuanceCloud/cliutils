// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"math"
	"sort"
	T "testing"

	"github.com/stretchr/testify/assert"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

func TestKVs(t *T.T) {
	t.Run("add-tag", func(t *T.T) {
		kvs := NewKVs(map[string]any{"f1": 123})

		kvs = kvs.AddTag(`t1`, `v1`)
		assert.Equal(t, []byte(`v1`), kvs.Get(`t1`).GetD())
		assert.Equal(t, 1, kvs.TagCount())

		// add new tag t2
		kvs = kvs.Add(`t2`, []byte(`v2`), true, true)
		assert.Equal(t, []byte(`v2`), kvs.Get(`t2`).GetD())
		assert.Equal(t, 2, kvs.TagCount())

		// replace t2's value v3
		kvs = kvs.Add(`t2`, []byte(`v3`), true, true)
		assert.Equal(t, []byte(`v3`), kvs.Get(`t2`).GetD())
		assert.Equal(t, 2, kvs.TagCount())

		// invalid tag value(must be []byte/string), switch to field
		kvs = kvs.Add(`tag-as-field`, 123, true, true)
		assert.Equal(t, int64(123), kvs.Get(`tag-as-field`).GetI())
		assert.Equal(t, 2, kvs.TagCount())

		// invalid tag override exist
		kvs = kvs.Add(`t2`, false, true, true)
		assert.Equal(t, false, kvs.Get(`t2`).GetB())
		assert.Equal(t, 1, kvs.TagCount())
	})

	t.Run(`new-empty`, func(t *T.T) {
		kvs := NewKVs(nil)
		assert.Equal(t, 0, len(kvs))
	})

	t.Run(`new-invalid-float`, func(t *T.T) {
		kvs := NewKVs(map[string]any{
			"f1": math.NaN(),
			"f2": math.Inf(1),
		})

		assert.Equal(t, 2, len(kvs))
	})

	t.Run(`new-all-types`, func(t *T.T) {
		kvs := NewKVs(map[string]any{
			"f1": 123,
			"f2": uint64(123),
			"f3": 3.14,
			"f4": "hello",
			"f5": []byte(`world`),
			"f6": false,
			"f7": true,
			"f8": func() *anypb.Any {
				x, _ := anypb.New(&AnyDemo{Demo: "haha"})
				return x
			}(),
			"f9": struct{}{},
		})
		assert.Equal(t, 9, len(kvs))

		assert.Equal(t, int64(123), kvs.Get(`f1`).GetI())
		assert.Equal(t, uint64(123), kvs.Get(`f2`).GetU())
		assert.Equal(t, 3.14, kvs.Get(`f3`).GetF())
		assert.Equal(t, []byte(`hello`), kvs.Get(`f4`).GetD())
		assert.Equal(t, []byte(`world`), kvs.Get(`f5`).GetD())
		assert.Equal(t, false, kvs.Get(`f6`).GetB())
		assert.Equal(t, true, kvs.Get(`f7`).GetB())

		x := kvs.Get(`f8`).GetA()
		assert.NotNil(t, x)
		t.Logf("any: %s", x)
		t.Logf("any.type: %q", x.TypeUrl)
		t.Logf("any.value: %q", x.Value)

		assert.Nil(t, kvs.Get(`f9`).Val)

		t.Logf("kvs:\n%s", kvs.Pretty())
	})

	t.Run(`add-kv`, func(t *T.T) {
		kvs := NewKVs(nil)

		kvs = kvs.MustAddKV(NewKV(`t1`, false, WithKVTagSet(true))) // set tag failed on bool value
		kvs = kvs.MustAddKV(NewKV(`t2`, "v1", WithKVTagSet(true)))
		kvs = kvs.MustAddKV(NewKV(`t3`, []byte("v2"), WithKVTagSet(true)))

		kvs = kvs.MustAddKV(NewKV(`f1`, "foo"))
		kvs = kvs.MustAddKV(NewKV(`f2`, 123, WithKVUnit("MB"), WithKVType(MetricType_COUNT)))
		kvs = kvs.MustAddKV(NewKV(`f3`, 3.14, WithKVUnit("some"), WithKVType(MetricType_GAUGE)))

		assert.Equal(t, 6, len(kvs))

		t.Logf("kvs:\n%s", kvs.Pretty())
	})

	// any update to kvs should keep them sorted
	t.Run(`test-sorted`, func(t *T.T) {
		kvs := NewKVs(nil)

		assert.True(t, sort.IsSorted(kvs)) // empty kvs sorted

		kvs = kvs.Add(`f2`, false, false, false)
		kvs = kvs.Add(`f1`, 123, false, false)
		kvs = kvs.MustAddTag(`t1`, "v1")

		assert.True(t, sort.IsSorted(kvs))

		kvs = kvs.Del(`f1`)
		assert.True(t, sort.IsSorted(kvs))

		kvs = kvs.MustAddKV(NewKV(`f3`, 3.14))
		assert.True(t, sort.IsSorted(kvs))
	})
}
