// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"sort"
	"strings"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPointRander(t *T.T) {
	t.Run("rand-1", func(t *T.T) {
		r := NewRander()
		pts := r.Rand(1)

		require.Equal(t, 1, len(pts))

		pt := pts[0]
		fs := pt.Fields()
		require.True(t, len(fs) > 0)

		tags := pt.Tags()

		require.Equal(t, defFields, len(fs))
		require.Equal(t, defTags, len(tags))

		for _, tag := range tags {
			require.Equal(t, defKeyLen, len(tag.Key))
			require.Equal(t, defValLen, len(tag.GetS()))
		}

		for _, f := range fs {
			require.Equal(t, defKeyLen, len(f.Key))

			switch x := f.Val.(type) {
			case *Field_D:
				require.Equal(t, defValLen, len(x.D))
			default: // skip
			}
		}

		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-measurement-prefix", func(t *T.T) {
		r := NewRander(WithRandMeasurementPrefix("abc_"))
		pts := r.Rand(1)
		require.True(t, strings.HasPrefix(pts[0].Name(), "abc_"))

		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-pb", func(t *T.T) {
		r := NewRander(WithRandPB(true))
		pts := r.Rand(1)
		require.True(t, pts[0].HasFlag(Ppb))

		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-time", func(t *T.T) {
		ts := time.Unix(0, 123)
		r := NewRander(WithRandTime(ts))

		pts := r.Rand(1)
		require.Equal(t, ts.UnixNano(), pts[0].Time().UnixNano())
		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-text", func(t *T.T) {
		r := NewRander(WithRandText(4))

		pts := r.Rand(1)
		fs := pts[0].Fields()
		require.True(t, len(fs) > 0)

		require.True(t, fs.Has("message"), "fields: %s", fs.Pretty())
		require.True(t, fs.Has("error_message"))
		require.True(t, fs.Has("error_stack"))

		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-tags", func(t *T.T) {
		r := NewRander(WithRandTags(100))

		pts := r.Rand(1)

		require.Equal(t, 100, len(pts[0].Tags()))

		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-fields", func(t *T.T) {
		r := NewRander(WithRandFields(100))

		pts := r.Rand(1)

		fs := pts[0].Fields()
		require.True(t, len(fs) > 0)

		require.Equal(t, 100, len(fs))

		t.Logf("point: %s", pts[0].Pretty())
	})

	t.Run("with-key-val-len", func(t *T.T) {
		klen := 751 % defaultKeyLen // can't exceed default point key len
		vlen := 157
		r := NewRander(WithRandKeyLen(klen), WithRandValLen(vlen))

		pts := r.Rand(1)

		fs := pts[0].Fields()
		require.True(t, len(fs) > 0)

		tags := pts[0].Tags()

		for _, f := range fs {
			require.Equal(t, klen, len(f.Key))
			if x, ok := f.Val.(*Field_D); ok {
				require.Equal(t, vlen, len(x.D))
			}
		}

		for _, tag := range tags {
			require.Equal(t, klen, len(tag.Key))
			require.Equal(t, vlen, len(tag.GetS()))
		}
	})
}

func TestWithFixTags(t *T.T) {
	t.Run("with-fix-tags", func(t *T.T) {
		r := NewRander(WithFixedTags(true))

		pt1 := r.Rand(1)[0]
		pt2 := r.Rand(1)[0]

		t.Logf("tag keys: %v", r.tagKeys)
		t.Logf("tag vals: %v", r.tagVals)

		pt1tags := pt1.Tags()
		pt2tags := pt2.Tags()
		for idx, tag := range pt1tags {
			require.Equal(t, pt2tags[idx].Key, tag.Key, "%d not equal:\npt1: %s\n\npt2: %s", idx, pt1.Pretty(), pt2.Pretty())
			require.Equal(t, pt2tags[idx].Val, tag.Val)
		}
	})
}

func TestWithFixKeys(t *T.T) {
	t.Run("with-fix-keys", func(t *T.T) {
		r := NewRander(WithFixedKeys(true))

		require.Equal(t, r.getFieldKey(0), r.getFieldKey(0))
		require.NotEqual(t, r.getFieldKey(2), r.getFieldKey(3))

		pt1 := r.Rand(1)[0]
		pt2 := r.Rand(1)[0]

		// NOTE: sort kvs to keep assert ok
		sort.Sort(pt1.kvs)
		sort.Sort(pt2.kvs)

		pt1tags := pt1.Tags()
		pt2tags := pt2.Tags()
		for idx, tag := range pt1tags {
			assert.Equal(t, pt2tags[idx].Key, tag.Key)
		}

		pt1fs := pt1.Fields()
		pt2fs := pt2.Fields()
		t.Logf("field keys: %v", r.fieldKeys)
		for idx, f := range pt1fs {
			require.Equal(t,
				pt2fs[idx].Key,
				f.Key,
				"%d not equal:\npt1: %s\n\npt2: %s",
				idx,
				pt1.kvs.Pretty(),
				pt2.kvs.Pretty())
		}
	})
}
