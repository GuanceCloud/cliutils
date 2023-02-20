// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"os"
	"path/filepath"
)

type CacheOption func(c *DiskCache)

// WithBatchSize file size, default 64MB.
func WithBatchSize(size int64) CacheOption {
	return func(c *DiskCache) {
		c.batchSize = size
	}
}

// WithMaxDataSize set max single data size, default 32MB.
func WithMaxDataSize(size int32) CacheOption {
	return func(c *DiskCache) {
		c.maxDataSize = size
	}
}

// WithCapacity set cache capacity, default unlimited.
func WithCapacity(size int64) CacheOption {
	return func(c *DiskCache) {
		c.capacity = size
	}
}

// WithNoSync enable/disable sync on cache write.
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
		// TODO: check path here
		c.path = filepath.Clean(x)
	}
}
