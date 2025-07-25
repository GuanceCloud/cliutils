// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"errors"
	T "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRotate(t *T.T) {
	t.Run("rotate-on-0-datafile", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p), WithBatchSize(1024*2))
		if err != nil {
			t.Error(err)
			return
		}

		_1kb := make([]byte, 1024)

		// put 3kb
		for i := 0; i < 4; i++ {
			assert.NoError(t, c.Put(_1kb))
		}

		for { // get them all
			if err := c.Get(func(data []byte) error {
				assert.Len(t, data, 1024)
				return nil
			}); err != nil {
				if errors.Is(err, ErrNoData) {
					break
				} else {
					assert.NoError(t, err)
				}
			}
		}

		t.Logf("cache: %s", c)

		t.Logf("cache pos: %s", c.pos.String())

		// rotate it
		c.rotate()

		pos, err := posFromFile(c.pos.fname)
		assert.NoError(t, err)
		assert.Nil(t, pos)

		assert.Equal(t, ":-1", c.pos.String(), "cache: %s", c)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			ResetMetrics()
		})
	})

	t.Run("rotate", func(t *T.T) {
		p := t.TempDir()
		batchSize := int64(1024 * 1024)
		c, err := Open(WithPath(p), WithBatchSize(batchSize))
		if err != nil {
			t.Error(err)
			return
		}

		origFileCnt := len(c.dataFiles)
		maxRotate := 3

		t.Logf("files: %+#v", c.dataFiles)

		sample := bytes.Repeat([]byte("hello"), 1000) // 5kb
		total := 0
		for {
			require.NoError(t, c.Put(sample), "cache: %s", c)
			total += len(sample)

			// generate 3 batches
			if int64(total) > int64(maxRotate)*batchSize {
				break
			}
		}

		t.Logf("data files: %+#v", c.dataFiles)

		assert.Equal(t, origFileCnt+maxRotate, len(c.dataFiles))

		readBytes := 0
		fn := func(x []byte) error {
			assert.Equal(t, sample, x)
			readBytes += len(x)
			return nil
		}

		for {
			if len(c.dataFiles) == 0 {
				break
			}

			if err := c.Get(fn); err != nil {
				if errors.Is(err, ErrNoData) {
					t.Logf("read EOF")
				} else {
					t.Error(err)
				}
				break
			}
		}

		t.Logf("total read bytes: %d", readBytes)

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
			ResetMetrics()
		})
	})
}
