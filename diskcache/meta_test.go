// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	T "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMeta(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		var (
			mb       = int64(1024 * 1024)
			p        = t.TempDir()
			capacity = 32 * mb
			large    = make([]byte, mb)
			small    = make([]byte, 1024*3)
			maxPut   = 4 * capacity
		)

		c, err := Open(WithPath(p),
			WithCapacity(capacity),
			WithMaxDataSize(int32(mb)),
			WithBatchSize(4*mb))
		assert.NoError(t, err)

		putBytes := 0

		n := 0
		for {
			switch n % 2 {
			case 0:
				c.Put(small)
				putBytes += len(small)
			case 1:
				c.Put(large)
				putBytes += len(large)
			}
			n++

			if int64(putBytes) > maxPut {
				break
			}
		}

		assert.True(t, c.Size() > 0)
		assert.Equal(t, capacity, c.Capacity())
		assert.Equal(t, int32(mb), c.MaxDataSize())
		assert.Equal(t, 4*mb, c.MaxBatchSize())

		t.Cleanup(func() {
			require.NoError(t, c.Close())
			ResetMetrics()
		})
	})
}
