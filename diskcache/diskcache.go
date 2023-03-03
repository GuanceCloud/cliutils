// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package diskcache is a simple local-disk cache implements.
package diskcache

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils/logger"
	"github.com/gofrs/flock"
)

const (
	dataHeaderLen = 4
	EOFHint       = 0xdeadbeef
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
	wakeup     time.Duration

	wlock, // used to exclude concurrent Put.
	rlock *sync.Mutex // used to exclude concurrent Get.
	rwlock *sync.Mutex // used to exclude switch/rotate/drop/Close

	flock *flock.Flock
	pos   *pos

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
	putBytes,
	getCost,
	putCost int64
}

func (c *DiskCache) String() string {
	return fmt.Sprintf("%s/[files: %d][maxDataSize: %d][batchSize: %d]{capacity: %d}",
		c.path, len(c.dataFiles), c.maxDataSize, c.batchSize, c.capacity,
	)
}
