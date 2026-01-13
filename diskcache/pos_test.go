// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"fmt"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDump(t *T.T) {
	t.Run(`dump-undump`, func(t *T.T) {
		p := &pos{
			Seek: 1024 * 1024 * 1024,
			Name: fmt.Appendf(nil, "data.%032d", 1234),
		}

		data, err := p.MarshalBinary()
		assert.NoError(t, err)

		t.Logf("get dump: %x", data)

		var p2 pos

		assert.NoError(t, p2.UnmarshalBinary(data))

		assert.Equal(t, int64(1024*1024*1024), p2.Seek)
		assert.Equal(t, fmt.Appendf(nil, "data.%032d", 1234), p2.Name)

		t.Logf("pos: %s", p)
	})

	t.Run(`seek--1`, func(t *T.T) {
		p := &pos{
			Seek: -1,
			Name: fmt.Appendf(nil, "data.%032d", 1234),
		}

		data, err := p.MarshalBinary()
		assert.NoError(t, err)

		t.Logf("get dump: %x", data)

		var p2 pos
		assert.NoError(t, p2.UnmarshalBinary(data))
		assert.Equal(t, int64(-1), p2.Seek)
		assert.Equal(t, fmt.Appendf(nil, "data.%032d", 1234), p2.Name)

		t.Logf("pos: %s", p)
	})

	t.Run(`seek-0`, func(t *T.T) {
		p := &pos{
			Name: fmt.Appendf(nil, "data.%032d", 1234),
			Seek: 0,
		}

		data, err := p.MarshalBinary()
		assert.NoError(t, err)

		t.Logf("get dump: %x", data)

		var p2 pos
		assert.NoError(t, p2.UnmarshalBinary(data))

		assert.Equal(t, int64(0), p2.Seek)
		assert.Equal(t, fmt.Appendf(nil, "data.%032d", 1234), p2.Name)

		t.Logf("pos: %s", p)
	})
}

func BenchmarkPosDump(b *T.B) {
	p := pos{
		Seek: 1024 * 1024 * 1024,
		Name: fmt.Appendf(nil, "data.%032d", 1234),
	}

	b.Run("binary", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			p.MarshalBinary()
		}
	})

	b.Run("json", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			p.dumpJSON()
		}
	})

	b.Run("force-dump", func(b *T.B) {
		p := pos{
			Seek:      1024 * 1024 * 1024,
			Name:      fmt.Appendf(nil, "data.%032d", 1234),
			dumpCount: 0,
		}

		for i := 0; i < b.N; i++ {
			p.dumpFile()
		}
	})

	b.Run("interval-dump", func(b *T.B) {
		p := pos{
			Seek:         1024 * 1024 * 1024,
			Name:         fmt.Appendf(nil, "data.%032d", 1234),
			dumpCount:    100,
			dumpInterval: 100 * time.Millisecond,
		}

		for i := 0; i < b.N; i++ {
			p.dumpFile()
		}
	})
}
