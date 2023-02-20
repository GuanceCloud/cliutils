// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package diskcache is a simple local-disk cache implements.
package diskcache

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils/logger"
)

const (
	dataHeaderLen = 4
	eofHint       = 0xdeadbeef
)

var (
	ErrNoData                    = errors.New("no data")
	ErrUnexpectedReadSize        = errors.New("unexpected read size")
	ErrTooLargeData              = errors.New("too large data")
	ErrEOF                       = errors.New("EOF")
	ErrInvalidDataFileName       = errors.New("invalid datafile name")
	ErrInvalidDataFileNameSuffix = errors.New("invalid datafile name suffix")
	ErrBadHeader                 = errors.New("bad header")

	l = logger.DefaultSLogger("diskcache")
)

// DiskCache is the representation of a disk cache.
// A DiskCache is safe for concurrent use by multiple goroutines.
type DiskCache struct {
	path string

	dataFiles []string

	curWriteFile,
	curReadfile string

	wfd, // write fd
	rfd *os.File // read fd

	wfdCreated time.Time

	wlock, // used to exclude concurrent Put.
	rlock *sync.Mutex // used to exclude concurrent Get.
	rwlock *sync.Mutex // used to exclude switch/rotate/drop/Close

	curBatchSize,
	batchSize,
	capacity int64
	maxDataSize int32

	// File permission, default 0750/0640
	dirPerms, filePerms os.FileMode

	// NoSync if enabled, may cause data missing, default false
	noSync bool

	// metrics related
	rotateCount,
	droppedBatch,
	getCount,
	putCount int
	size int64
	getBytes,
	putBytes int64
}

// Open init and create a new disk cache. We can set other options with various options.
func Open(opts ...CacheOption) (*DiskCache, error) {
	c := defaultInstance()

	// apply extra options
	for _, x := range opts {
		if x != nil {
			x(c)
		}
	}

	if err := c.doOpen(); err != nil {
		return nil, err
	}

	return c, nil
}

func defaultInstance() *DiskCache {
	return &DiskCache{
		noSync: false,

		batchSize:   20 * 1024 * 1024,
		maxDataSize: 0, // not set

		dirPerms:  0o750,
		filePerms: 0o640,
	}
}

func (c *DiskCache) doOpen() error {
	l = logger.SLogger("diskcache")

	c.curWriteFile = filepath.Join(c.path, "data")

	c.wlock = &sync.Mutex{}
	c.rlock = &sync.Mutex{}
	c.rwlock = &sync.Mutex{}

	if c.dirPerms == 0 {
		c.dirPerms = 0o755
	}

	if c.filePerms == 0 {
		c.filePerms = 0o640
	}

	if c.batchSize == 0 {
		c.batchSize = 20 * 1024 * 1024
	}

	if int64(c.maxDataSize) > c.batchSize {
		l.Warnf("reset MaxDataSize from %d to %d",
			c.maxDataSize, c.batchSize/2)

		// reset max-data-size to half of batch size
		c.maxDataSize = int32(c.batchSize / 2)
	}

	if err := os.MkdirAll(c.path, c.dirPerms); err != nil {
		return err
	}

	c.syncEnv()

	// write append fd, always write to the same-name file
	if err := c.openWriteFile(); err != nil {
		return err
	}

	// list files under @path
	arr := []string{}
	if err := filepath.Walk(c.path, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			return nil
		}

		c.size += fi.Size()

		arr = append(arr, path)
		return nil
	}); err != nil {
		return err
	}

	sort.Strings(arr)
	if len(arr) > 1 && arr[0] == c.curWriteFile {
		c.dataFiles = arr[1:] // ignore first writing file, we do not read file `data` if data.000001/0000002/... exists
	}

	l.Infof("init %d datafiles", len(c.dataFiles))

	return nil
}

// Close reclame fd resources.
// Close is safe to call concurrently with other operations and will
// block until all other operations finish.
func (c *DiskCache) Close() error {
	c.rwlock.Lock()
	defer c.rwlock.Unlock()

	if c.rfd != nil {
		if err := c.rfd.Close(); err != nil {
			return err
		}
	}

	if c.wfd != nil {
		if err := c.wfd.Close(); err != nil {
			return err
		}
	}
	return nil
}

// Put write @data to disk cache, if reached batch size, a new batch is rotated.
// Put is safe to call concurrently with other operations and will
// block until all other operations finish.
func (c *DiskCache) Put(data []byte) error {
	c.wlock.Lock()
	defer c.wlock.Unlock()

	c.putCount++

	if c.capacity > 0 && c.size+int64(len(data)) > c.capacity {
		if err := c.dropBatch(); err != nil {
			return err
		}
	}

	if int32(len(data)) > c.maxDataSize && c.maxDataSize > 0 {
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

// Fn is the handler to eat cache from disk.
type Fn func([]byte) error

// Get fetch new data from disk cache, then passing to @fn
// if any error occurred during call @fn, the reading data is
// ignored, and will not read again.
// Gut is safe to call concurrently with other operations and will
// block until all other operations finish.
func (c *DiskCache) Get(fn Fn) error {
	c.rlock.Lock()
	defer c.rlock.Unlock()

	c.getCount++

	// wakeup sleeping write file, rotate it for succession reading!
	if time.Since(c.wfdCreated) > time.Second*3 && c.curBatchSize > 0 {
		l.Debugf("wakeup %s(%d bytes), global size: %d",
			c.curWriteFile, c.curBatchSize, c.size)
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
	n, err := c.rfd.Read(hdr)
	if err != nil {
		pos, _err := c.rfd.Seek(0, 1)
		if _err != nil {
			return fmt.Errorf("rfd.Seek: %w", _err)
		}
		return fmt.Errorf("rfd.Read(%s/pos: %d): %w", c.curReadfile, pos, err)
	}

	if n != dataHeaderLen {
		return ErrBadHeader
	}

	nbytes := binary.LittleEndian.Uint32(hdr[0:])

	if nbytes == eofHint { // EOF
		if err := c.removeCurrentReadingFile(); err != nil {
			return fmt.Errorf("removeCurrentReadingFile: %w", err)
		}

		// reopen next file to read
		if err := c.switchNextFile(); err != nil {
			return err
		}

		goto retry // read next new file
	}

	databuf := make([]byte, nbytes)

	n, err = c.rfd.Read(databuf)
	if err != nil {
		return err
	}

	if n != int(nbytes) {
		return ErrUnexpectedReadSize
	}

	c.getBytes += int64(n)

	// NOTE: if @fn failed, c.rfd never seek back, data dropped
	return fn(databuf)
}
