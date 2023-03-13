// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	T "testing"

	"github.com/stretchr/testify/assert"
)

func TestMetric(t *T.T) {
	t.Run("basic", func(t *T.T) {
		p := t.TempDir()
		c, err := Open(WithPath(p))
		assert.NoError(t, err)

		smallBytes := make([]byte, 100)

		assert.NoError(t, c.Put(smallBytes))

		m := c.Metrics()

		assert.False(t, m.Fields().Get([]byte(`nosync`)).GetB())
		assert.Equal(t, int64(1), m.Fields().Get([]byte(`put`)).GetI())

		assert.Equal(t, int64(100), /* dataHeaderLen not counted in put_bytes */
			m.Fields().Get([]byte(`put_bytes`)).GetI())

		assert.Equal(t, int64(100+dataHeaderLen), m.Fields().Get([]byte(`size`)).GetI())

		assert.Equal(t, int64(0), // these fileds all zero
			m.Fields().Get([]byte(`data_files`)).GetI()+
				m.Fields().Get([]byte(`dropped_batch`)).GetI()+
				m.Fields().Get([]byte(`get`)).GetI()+
				m.Fields().Get([]byte(`get_bytes`)).GetI()+
				m.Fields().Get([]byte(`get_cost_avg`)).GetI()+
				m.Fields().Get([]byte(`rotate_count`)).GetI())
		t.Logf("metric: %s", m.LineProto())

		// rotate to make it readble
		assert.NoError(t, c.rotate())
		assert.NoError(t, c.Get(nil))

		m = c.Metrics()

		assert.Equal(t, int64(1), m.Fields().Get([]byte(`get`)).GetI())
		assert.Equal(t, int64(100), /* dataHeaderLen not counted in get_bytes */
			m.Fields().Get([]byte(`get_bytes`)).GetI())

		assert.Equal(t, int64(100+dataHeaderLen+4 /*EOFHint*/), m.Fields().Get([]byte(`size`)).GetI())

		t.Cleanup(func() {
			assert.NoError(t, c.Close())
		})
	})
}
