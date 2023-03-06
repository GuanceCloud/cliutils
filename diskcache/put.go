// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"encoding/binary"
	"time"
)

// Put write @data to disk cache, if reached batch size, a new batch is rotated.
// Put is safe to call concurrently with other operations and will
// block until all other operations finish.
func (c *DiskCache) Put(data []byte) error {
	c.wlock.Lock()
	defer c.wlock.Unlock()

	start := time.Now()

	c.putCount++
	defer func() {
		c.putCost += int64(time.Since(start))
	}()

	if c.capacity > 0 && c.size+int64(len(data)) > c.capacity {
		if err := c.dropBatch(); err != nil {
			return err
		}
	}

	if c.maxDataSize > 0 && int32(len(data)) > c.maxDataSize {
		l.Warnf("too large data: %d > %d", len(data), c.maxDataSize)
		return ErrTooLargeData
	}

	hdr := make([]byte, dataHeaderLen)

	binary.LittleEndian.PutUint32(hdr, uint32(len(data)))
	if _, err := c.wfd.Write(hdr); err != nil {
		return err
	}

	if _, err := c.wfd.Write(data); err != nil {
		return err
	}

	if !c.noSync {
		if err := c.wfd.Sync(); err != nil {
			return err
		}
	}

	c.curBatchSize += int64(len(data) + dataHeaderLen)
	c.size += int64(len(data) + dataHeaderLen)

	// rotate new file
	if c.curBatchSize >= c.batchSize {
		if err := c.rotate(); err != nil {
			return err
		}
	}

	c.putBytes += int64(len(data))

	return nil
}
