// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"os"
	T "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPutGet(t *T.T) {
	t.Run(`clean-pos-on-eof`, func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		data := make([]byte, 100)
		if err := c.Put(data); err != nil {
			t.Error(err)
		}

		assert.NoError(t, c.rotate())
		assert.NoError(t, c.Get(func(data []byte) error { return nil }))
		assert.Error(t, c.Get(func(data []byte) error { return nil })) // EOF

		pos, err := posFromFile(c.pos.fname)
		assert.NoError(t, err)

		t.Logf("pos: %s", pos)

		t.Logf("metric: %s", c.Metrics().LineProto())

		t.Cleanup(func() {
			c.Close()
		})
	})

	t.Run("put-get", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		str := "hello world"
		if err := c.Put([]byte(str)); err != nil {
			t.Error(err)
		}

		assert.Equal(t, int64(len(str)+dataHeaderLen), c.curBatchSize)

		if err := c.Get(func(data []byte) error {
			t.Logf("get: %s", string(data))
			return nil
		}); err != nil {
			t.Logf("get: %s", err)
		}

		t.Logf("metric: %s", c.Metrics().LineProto())

		t.Cleanup(func() {
			c.Close()
			os.RemoveAll(p)
		})
	})

	t.Run(`get-with-pos`, func(t *T.T) {
		p := t.TempDir()

		kbdata := make([]byte, 1024)

		c, err := Open(WithPath(p),
			WithCapacity(int64(len(kbdata)*10)),
			WithBatchSize(int64(len(kbdata)*2)))
		assert.NoError(t, err)

		for i := 0; i < 10; i++ { // write 10kb
			require.NoError(t, c.Put(kbdata), "cache: %s", c)
		}

		// create a read pos
		assert.NoError(t, c.Get(func(data []byte) error {
			assert.Len(t, data, len(kbdata))
			return nil
		}))
		assert.Equal(t, int64(len(kbdata)+dataHeaderLen), c.pos.Seek)
		assert.NoError(t, c.Close())

		_, err = os.Stat(c.pos.fname)
		require.NoError(t, err)

		// reopen the cache
		c2, err := Open(WithPath(p),
			WithCapacity(int64(len(kbdata)*10)),
			WithBatchSize(int64(len(kbdata)*2)))
		require.NoError(t, err, "get error: %s", err)
		assert.Equal(t, c2.pos.Seek, int64(len(kbdata)+dataHeaderLen))

		assert.NoError(t, c2.Get(func(data []byte) error {
			assert.Len(t, data, len(kbdata))
			return nil
		}))
		assert.Equal(t, int64(len(kbdata)+dataHeaderLen), c.pos.Seek)
		assert.NoError(t, c2.Close())
		assert.Equal(t, c2.pos.Seek, int64(2*(len(kbdata)+dataHeaderLen)))

		t.Cleanup(func() {
			c2.Close()
			os.RemoveAll(p)
		})
	})
}
