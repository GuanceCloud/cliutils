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

		arr := NewArray([]any{1, 2, 3})
		assert.Len(t, arr.Arr, 3)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("mixed-array", func(t *T.T) {
		var kvs KVs

		arr := NewArray([]any{1, 2.0, false})
		assert.Len(t, arr.Arr, 3)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("with-nil", func(t *T.T) {
		var kvs KVs

		arr := NewArray([]any{1, 2.0, nil})
		assert.Len(t, arr.Arr, 2)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})

	t.Run("with-non-baisc-type", func(t *T.T) {
		var kvs KVs

		type custom struct {
			some string
		}

		arr := NewArray([]any{1, 2.0, custom{some: "one"}})
		assert.Len(t, arr.Arr, 2)

		x, err := anypb.New(arr)
		assert.NoError(t, err)

		kvs = kvs.Add("k1", x, false, false)
		pt := NewPointV2("basic", kvs)

		t.Logf("%s", pt.Pretty())
	})
}
