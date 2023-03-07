// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"encoding/binary"
	"fmt"
	"time"
)

// Fn is the handler to eat cache from disk.
type Fn func([]byte) error

// Get fetch new data from disk cache, then passing to @fn
// if any error occurred during call @fn, the reading data is
// dropped, and will not read again.
//
// Get is safe to call concurrently with other operations and will
// block until all other operations finish.
func (c *DiskCache) Get(fn Fn) error {
	var nbytes int

	c.rlock.Lock()
	defer c.rlock.Unlock()

	start := time.Now()

	c.getCount++
	defer func() {
		c.getCost += int64(time.Since(start))
		if nbytes != EOFHint {
			c.getBytes += int64(nbytes)
		}
	}()

	// wakeup sleeping write file, rotate it for succession reading!
	if time.Since(c.wfdCreated) > c.wakeup && c.curBatchSize > 0 {
		l.Debugf("wakeup %s(%d bytes), global size: %d", c.curWriteFile, c.curBatchSize, c.size)

		if err := func() error {
			c.wlock.Lock()
			defer c.wlock.Unlock()
			return c.rotate()
		}(); err != nil {
			return err
		}
	}

	if c.rfd == nil {
		if err := c.switchNextFile(); err != nil {
			return err
		}
	}

retry:
	if c.rfd == nil {
		return ErrEOF
	}

	hdr := make([]byte, dataHeaderLen)
	if n, err := c.rfd.Read(hdr); err != nil {
		return fmt.Errorf("rfd.Read(%s): %w", c.curReadfile, err)
	} else if n != dataHeaderLen {
		return ErrBadHeader
	}

	nbytes = int(binary.LittleEndian.Uint32(hdr[0:]))

	if nbytes == EOFHint { // EOF
		if err := c.removeCurrentReadingFile(); err != nil {
			return fmt.Errorf("removeCurrentReadingFile: %w", err)
		}

		// clear .pos
		if !c.noPos {
			if err := c.pos.reset(); err != nil {
				l.Errorf("pos reset: %s", err)
				return err
			}
		}

		// reopen next file to read
		if err := c.switchNextFile(); err != nil {
			return err
		}

		goto retry // read next new file
	}

	databuf := make([]byte, nbytes)

	if n, err := c.rfd.Read(databuf); err != nil {
		return err
	} else if n != nbytes {
		return ErrUnexpectedReadSize
	}

	// update seek position
	if !c.noPos {
		c.pos.Seek += int64(dataHeaderLen + nbytes)
		if err := c.pos.dumpFile(); err != nil {
			return err
		}
	}

	if fn != nil {
		return fn(databuf)
	}

	return nil
}
