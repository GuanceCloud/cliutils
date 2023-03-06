// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"os"
	"path/filepath"
	"time"
)

type CacheOption func(c *DiskCache)

// WithNoLock set .lock on or off.
// .lock used to exclude Open() on same path.
func WithNoLock(on bool) CacheOption {
	return func(c *DiskCache) {
		c.noLock = on
	}
}

// WithNoPos set .pos on or off.
// .pos used to remember last Get position of cache, without
// .pos, if process stop and restart, some already-Get data will
// re-Get in the new process.
func WithNoPos(on bool) CacheOption {
	return func(c *DiskCache) {
		c.noPos = on
	}
}

// WithWakeup set duration on wakeup(default 3s), this wakeup time
// used to shift current-writing-file to ready-to-reading-file.
// NOTE: without wakeup, current-writing-file maybe not read-available
// for a long time.
func WithWakeup(wakeup time.Duration) CacheOption {
	return func(c *DiskCache) {
		if int64(wakeup) > 0 {
			c.wakeup = wakeup
		}
	}
}

// WithBatchSize set file size, default 64MB.
func WithBatchSize(size int64) CacheOption {
	return func(c *DiskCache) {
		if size > 0 {
			c.batchSize = size
		}
	}
}

// WithMaxDataSize set max single data size, default 32MB.
func WithMaxDataSize(size int32) CacheOption {
	return func(c *DiskCache) {
		if size > 0 {
			c.maxDataSize = size
		}
	}
}

// WithCapacity set cache capacity, default unlimited.
func WithCapacity(size int64) CacheOption {
	return func(c *DiskCache) {
		if size > 0 {
			c.capacity = size
		}
	}
}

// WithExtraCapacity add capacity to existing cache.
func WithExtraCapacity(size int64) CacheOption {
	return func(c *DiskCache) {
		if c.capacity+size > 0 {
			c.capacity += size
		}
	}
}

// WithNoSync enable/disable sync on cache write.
//
// without sync, the write performance 60~80 times faster for 512KB/1MB put,
// for smaller put will get more faster(1kb for 4000+ times).
func WithNoSync(on bool) CacheOption {
	return func(c *DiskCache) {
		c.noSync = on
	}
}

// WithDirPermission set disk dir permission mode.
func WithDirPermission(perms os.FileMode) CacheOption {
	return func(c *DiskCache) {
		c.dirPerms = perms
	}
}

// WithFilePermission set cache file permission mode.
func WithFilePermission(perms os.FileMode) CacheOption {
	return func(c *DiskCache) {
		c.filePerms = perms
	}
}

// WithPath set disk dirname.
func WithPath(x string) CacheOption {
	return func(c *DiskCache) {
		c.path = filepath.Clean(x)
	}
}
