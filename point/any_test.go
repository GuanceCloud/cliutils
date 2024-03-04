// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	T "testing"

	types "github.com/gogo/protobuf/types"
	"github.com/stretchr/testify/assert"
)

func TestAny(t *T.T) {
	t.Run("basic", func(t *T.T) {
		var kvs KVs

		arr, err := NewArray(1, 2, 3)
		assert.NoError(t, err)
		assert.Len(t, arr.Arr, 3)

		x, err := types.MarshalAny(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("mixed-array", func(t *T.T) {
		var kvs KVs

		arr, err := NewArray(1, 2.0, false)
		assert.NoError(t, err)
		assert.Len(t, arr.Arr, 3)

		x, err := types.MarshalAny(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("with-nil", func(t *T.T) {
		var kvs KVs

		arr, err := NewArray(1, 2.0)
		assert.NoError(t, err)
		assert.Len(t, arr.Arr, 2)

		x, err := types.MarshalAny(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("with-non-baisc-type", func(t *T.T) {
		type custom struct {
			some string
		}

		_, err := NewArray([]any{1, 2.0, custom{some: "one"}})
		assert.Error(t, err)
		t.Logf("expect error %q", err)
	})

	t.Run("map", func(t *T.T) {
		var kvs KVs

		m := MustNewMap(map[string]any{"i1": 1, "i2": 2})
		assert.Len(t, m.Map, 2)

		x, err := types.MarshalAny(m)
		assert.NoError(t, err)

		assert.Equal(t, "type.googleapis.com/point.Map", x.TypeUrl)


		t.Logf("any type URL: %s", x.GetTypeUrl())

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})
}

func TestAnyRaw(t *T.T) {
	t.Run("arr", func(t *T.T) {
		arr, err := NewArray(1, 2.0)
		assert.NoError(t, err)
		assert.Len(t, arr.Arr, 2)

		x, err := types.MarshalAny(arr)
		assert.NoError(t, err)

		raw := MustAnyRaw(x)
		assert.Equal(t, []any{int64(1), 2.0}, raw)
	})
}

func TestNewArray(t *T.T) {
	t.Run(`basic-uint`, func(t *T.T) {
		u16s := []uint16{
			uint16(1),
			uint16(2),
			uint16(3),
		}

		x := MustNewUintArray(u16s...)

		raw, err := AnyRaw(x)
		assert.NoError(t, err)
		assert.Equal(t, []any{
			uint64(1),
			uint64(2),
			uint64(3),
		}, raw)
		t.Logf("any.Raw: %+#v", raw)
	})

	t.Run(`basic-int`, func(t *T.T) {
		i16s := []int16{
			int16(1),
			int16(2),
			int16(3),
		}

		raw, err := AnyRaw(MustNewIntArray(i16s...))
		assert.NoError(t, err)
		assert.Equal(t, []any{
			int64(1),
			int64(2),
			int64(3),
		}, raw)
		t.Logf("any.Raw: %+#v", raw)
	})

	t.Run(`basic-float`, func(t *T.T) {
		arr := []float64{
			float64(1.1),
			float64(2.2),
			float64(3.3),
		}

		raw, err := AnyRaw(MustNewFloatArray(arr...))
		assert.NoError(t, err)
		assert.Equal(t, []any{
			float64(1.1),
			float64(2.2),
			float64(3.3),
		}, raw)
		t.Logf("any.Raw: %+#v", raw)
	})

	t.Run(`basic-float32`, func(t *T.T) {
		arr := []float32{
			float32(1.1),
			float32(2.2),
			float32(3.1415926),
		}

		raw, err := AnyRaw(MustNewFloatArray(arr...))
		assert.NoError(t, err)
		assert.Len(t, raw, 3)
		assert.NotEqual(t, []any{ // float32 -> float64 not equal
			float64(1.1),
			float64(2.2),
			float64(3.1415926),
		}, raw)
		t.Logf("any.Raw: %+#v", raw)
	})

	t.Run(`basic-bool`, func(t *T.T) {
		arr := []bool{
			false, true,
		}

		raw, err := AnyRaw(MustNewBoolArray(arr...))
		assert.NoError(t, err)
		assert.Len(t, raw, 2)
		assert.Equal(t, []any{false, true}, raw)
		t.Logf("any.Raw: %+#v", raw)
	})

	t.Run(`basic-string`, func(t *T.T) {
		arr := []string{
			"s1", "s2", "s3",
		}

		raw, err := AnyRaw(MustNewStringArray(arr...))
		assert.NoError(t, err)
		assert.Len(t, raw, 3)
		assert.Equal(t, []any{"s1", "s2", "s3"}, raw)
		t.Logf("any.Raw: %+#v", raw)
	})
}
