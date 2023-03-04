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
)

func TestDropBatch(t *T.T) {
	p := t.TempDir()
	c, err := Open(WithPath(p),
		WithBatchSize(4*1024*1024),
		WithCapacity(32*1024*1024))
	if err != nil {
		t.Error(err)
		return
	}

	sample := bytes.Repeat([]byte("hello"), 7351)
	for {
		if err := c.Put(sample); err != nil {
			t.Error(err)
		}

		if c.droppedBatch > 3 {
			break
		}
	}

	t.Logf("metric: %s", c.Metrics().LineProto())
	t.Cleanup(func() {
		assert.NoError(t, c.Close())
	})
}

func TestDropDuringGet(t *T.T) {
	p := t.TempDir()
	c, err := Open(WithPath(p), WithBatchSize(1*1024*1024), WithCapacity(2*1024*1024))
	assert.NoError(t, err)

	sample := bytes.Repeat([]byte("hello"), 7351)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() { // fast put
		defer wg.Done()
		for {
			if err := c.Put(sample); err != nil {
				t.Error(err)
				return
			}

			if c.droppedBatch > 2 {
				return
			}
		}
	}()

	time.Sleep(time.Second) // wait new data write

	// slow get
	i := 0
	eof := 0
	for {
		time.Sleep(time.Millisecond * 100)
		if err := c.Get(func(x []byte) error {
			assert.Equal(t, sample, x)
			return nil
		}); err != nil {
			if errors.Is(err, ErrEOF) {
				t.Logf("[%s] read: %s", time.Now(), err)
				time.Sleep(time.Second)
				eof++
				if eof > 5 {
					break
				}
			} else {
				t.Logf("%s", err)
				time.Sleep(time.Second)
			}
		} else {
			eof = 0 // reset EOF if put ok
		}
		i++
	}

	wg.Wait()

	t.Logf("metric: %s", c.Metrics().LineProto())

	t.Cleanup(func() {
		assert.NoError(t, c.Close())
	})
}
