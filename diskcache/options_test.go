// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOptions(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		c := defaultInstance()

		WithWakeup(-time.Minute)(c) // invalid
		assert.Equal(t, c.wakeup, time.Second*3)

		WithWakeup(time.Minute)(c)
		assert.Equal(t, c.wakeup, time.Minute)

		WithBatchSize(1024)(c)
		assert.Equal(t, int64(1024), c.batchSize)

		WithBatchSize(-1024)(c)
		assert.Equal(t, int64(1024), c.batchSize)

		WithCapacity(100)(c)
		assert.Equal(t, int64(100), c.capacity)

		WithCapacity(-100)(c)
		assert.Equal(t, int64(100), c.capacity)

		WithExtraCapacity(-200)(c)
		assert.Equal(t, int64(100), c.capacity)

		WithExtraCapacity(-100)(c)
		assert.Equal(t, int64(100), c.capacity)

		WithExtraCapacity(-50)(c)
		assert.Equal(t, int64(50), c.capacity)

		WithMaxDataSize(100)(c)
		assert.Equal(t, int32(100), c.maxDataSize)

		WithNoSync(false)(c)
		assert.False(t, c.noSync)
	})
}
