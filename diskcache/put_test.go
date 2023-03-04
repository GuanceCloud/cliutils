// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"errors"
	"sync"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func BenchmarkPutGet(b *T.B) {
	p := b.TempDir()
	c, err := Open(WithPath(p), WithBatchSize(1024*1024*4), WithCapacity(4*1024*1024*1024))
	require.NoError(b, err)

	_1mb := make([]byte, 1024*1024)
	_1kb := make([]byte, 1024)
	_512kb := make([]byte, 1024*512)

	b.Run(`put-1mb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_1mb)
		}
	})

	b.Run(`put-1kb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_1kb)
		}
	})

	b.Run(`put-512kb`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Put(_512kb)
		}
	})

	b.Run(`get`, func(b *T.B) {
		for i := 0; i < b.N; i++ {
			c.Get(func(_ []byte) error { return nil })
		}
	})

	m := c.Metrics()
	b.Logf(m.LineProto())

	b.Cleanup(func() {
		assert.NoError(b, c.Close())
	})
}

func TestConcurrentPutGet(t *T.T) {
	p := t.TempDir()
	c, err := Open(WithPath(p), WithBatchSize(4*1024*1024), WithCapacity(1024*1024*1024))
	assert.NoError(t, err)

	defer c.Close()

	t.Logf("files: %+#v", c.dataFiles)

	sample := bytes.Repeat([]byte("hello"), 7351)

	wg := sync.WaitGroup{}
	concurrency := 4

	fnPut := func(idx int) {
		defer wg.Done()
		nput := 0
		exceed100ms := 0

		for {
			start := time.Now()
			assert.NoError(t, c.Put(sample))

			cost := time.Since(start)
			if cost > 100*time.Millisecond {
				exceed100ms++
			}

			nput++

			if nput > 1000 {
				t.Logf("[%d] Put exit", idx)
				return
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go fnPut(i)
	}

	eof := 0
	fnGet := func(idx int) {
		defer wg.Done()
		nget := 0
		readBytes := 0
		exceed100ms := 0

		for {
			nget++
			start := time.Now()
			if err := c.Get(func(x []byte) error {
				assert.Equal(t, sample, x)
				readBytes += len(x)

				cost := time.Since(start)
				if cost > 100*time.Millisecond {
					exceed100ms++
				}

				return nil
			}); err != nil {
				if errors.Is(err, ErrEOF) {
					time.Sleep(time.Second)
					eof++
					if eof >= 10 {
						break
					}
				} else {
					t.Logf("[%d]: %s", idx, err)
					time.Sleep(time.Second)
				}
			} else {
				eof = 0 // reset eof if Get ok
			}
		}
	}

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go fnGet(i)
	}

	wg.Wait()

	t.Logf("metric: %s", c.Metrics().LineProto())
	t.Cleanup(func() {
		assert.NoError(t, c.Close())
	})
}
