// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	T "testing"

	"github.com/stretchr/testify/assert"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

func TestAny(t *T.T) {
	t.Run("basic", func(t *T.T) {
		var kvs KVs

		arr := MustNewArray([]any{1, 2, 3})
		assert.Len(t, arr.Arr, 3)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("mixed-array", func(t *T.T) {
		var kvs KVs

		arr := MustNewArray([]any{1, 2.0, false})
		assert.Len(t, arr.Arr, 3)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("with-nil", func(t *T.T) {
		var kvs KVs

		arr := MustNewArray([]any{1, 2.0, nil})
		assert.Len(t, arr.Arr, 3)

		x, err := anypb.New(arr)
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

		x, err := anypb.New(m)
		assert.NoError(t, err)

		assert.Equal(t, "type.googleapis.com/point.Map", x.TypeUrl)
		assert.True(t, x.MessageIs(&Map{}))

		t.Logf("any name: %s", x.MessageName())

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})
}

func TestAnyRaw(t *T.T) {
	t.Run("arr", func(t *T.T) {
		arr := MustNewArray([]any{1, 2.0})
		assert.Len(t, arr.Arr, 2)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		raw := MustAnyRaw(x)
		assert.Equal(t, []any{int64(1), 2.0}, raw)
	})
}
