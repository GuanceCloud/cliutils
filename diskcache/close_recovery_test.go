// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCloseMakesCurrentBatchRecoverable(t *testing.T) {
	path := t.TempDir()
	want := []byte("persist-before-close")

	cache, err := Open(WithPath(path), WithBatchSize(1<<20))
	require.NoError(t, err)
	require.NoError(t, cache.Put(want))
	require.NoError(t, cache.Close())

	reopened, err := Open(WithPath(path), WithBatchSize(1<<20))
	require.NoError(t, err)
	defer reopened.Close() //nolint:errcheck

	var got []byte
	require.NoError(t, reopened.Get(func(data []byte) error {
		got = append(got, data...)
		return nil
	}))
	assert.Equal(t, want, got)
}
