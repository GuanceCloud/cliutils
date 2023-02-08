// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"bytes"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPutGet(t *testing.T) {
	os.RemoveAll(".TestPutGet")
	c, err := Open(".TestPutGet", nil)
	if err != nil {
		t.Error(err)
	}

	defer c.Close()

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
}

func TestDropDuringGet(t *testing.T) {
	os.RemoveAll(".TestDropDuringGet")
	c, err := Open(".TestDropDuringGet", &Option{
		BatchSize: 1 * 1024 * 1024,
		Capacity:  2 * 1024 * 1024,
	})
	if err != nil {
		t.Error(err)
		return
	}

	defer c.Close()

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
			if i%32 == 0 {
				t.Logf(">>>>>>>>>> Get %d bytes", len(x))
			}
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
}

func TestDropBatch(t *testing.T) {
	os.RemoveAll(".TestDropBatch")
	c, err := Open(".TestDropBatch", &Option{
		BatchSize: 4 * 1024 * 1024,
		Capacity:  32 * 1024 * 1024,
	})
	if err != nil {
		t.Error(err)
		return
	}

	defer c.Close()

	sample := bytes.Repeat([]byte("hello"), 7351)
	for {
		if err := c.Put(sample); err != nil {
			t.Error(err)
		}

		if c.droppedBatch > 3 {
			return
		}
	}
}

func TestConcurrentPutGet(t *testing.T) {
	os.RemoveAll(".TestConcurrentPutGet")
	c, err := Open(".TestConcurrentPutGet", &Option{
		BatchSize: 4 * 1024 * 1024,
		Capacity:  1024 * 1024 * 1024,
	})
	if err != nil {
		t.Error(err)
		return
	}

	defer c.Close()

	t.Logf("files: %+#v", c.dataFiles)

	sample := bytes.Repeat([]byte("hello"), 7351)

	wg := sync.WaitGroup{}
	concurrency := 4
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()
			nput := 0
			exceed100ms := 0

			for {
				start := time.Now()
				if err := c.Put(sample); err != nil {
					t.Error(err)
				}

				cost := time.Since(start)
				if cost > 100*time.Millisecond {
					exceed100ms++
					t.Logf("[%d] <<<<<<<<<< Put %d(%dth) bytes cost: %s(%.3f%%)",
						idx, len(sample), idx, cost, 100.0*float64(exceed100ms)/float64(nput))
				}

				nput++

				if nput > 1000 {
					t.Logf("[%d] Put exit", idx)
					return
				}
			}
		}(i)
	}

	wg.Add(concurrency)
	eof := 0
	for i := 0; i < concurrency; i++ {
		go func(idx int) {
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
						t.Logf("[%d] >>>>>>>>>> Get %d(%dth) bytes cost: %s(%.3f%%)", idx, len(sample), idx, cost, 100.0*float64(exceed100ms)/float64(nget))
					}

					return nil
				}); err != nil {
					if errors.Is(err, ErrEOF) {
						t.Logf("[%d] read: %s(%dth)", idx, err, eof)
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
			t.Logf("[%d] total read bytes: %d", idx, readBytes)
		}(i)
	}

	wg.Wait()
}

func TestRotate(t *testing.T) {
	os.RemoveAll(".TestRotate")
	c, err := Open(".TestRotate", &Option{
		BatchSize: 1024 * 1024, // 1MB
	})
	if err != nil {
		t.Error(err)
		return
	}

	defer c.Close()

	origFileCnt := len(c.dataFiles)

	t.Logf("files: %+#v", c.dataFiles)

	maxRotate := 3
	sample := bytes.Repeat([]byte("hello"), 1000) // 5kb
	for {
		if err := c.Put(sample); err != nil {
			t.Error(err)
		}

		if c.rotateCount >= maxRotate {
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
			if errors.Is(err, ErrEOF) {
				t.Logf("read EOF")
				return
			} else {
				t.Error(err)
			}
			break
		}
	}

	t.Logf("total read bytes: %d", readBytes)
}
