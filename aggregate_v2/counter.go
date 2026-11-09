// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync"

	"github.com/cespare/xxhash/v2"
)

// UniqueTraceCounter counts unique trace IDs in a time window with zero string pointer overhead.
type UniqueTraceCounter struct {
	mu  sync.Mutex
	set map[uint64]struct{}
}

// NewUniqueTraceCounter creates a new trace counter.
func NewUniqueTraceCounter(estimatedCapacity int) *UniqueTraceCounter {
	if estimatedCapacity <= 0 {
		estimatedCapacity = 1024
	}
	return &UniqueTraceCounter{
		set: make(map[uint64]struct{}, estimatedCapacity),
	}
}

// Add records a trace ID into the set.
func (c *UniqueTraceCounter) Add(traceID string) {
	if len(traceID) == 0 {
		return
	}
	hash := xxhash.Sum64String(traceID)
	c.mu.Lock()
	c.set[hash] = struct{}{}
	c.mu.Unlock()
}

// AddHash records a pre-hashed 64-bit trace ID.
func (c *UniqueTraceCounter) AddHash(hash uint64) {
	c.mu.Lock()
	c.set[hash] = struct{}{}
	c.mu.Unlock()
}

// Count returns the number of distinct trace IDs recorded.
func (c *UniqueTraceCounter) Count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.set)
}

// Reset clears the counter while preserving underlying map capacity.
func (c *UniqueTraceCounter) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	clear(c.set)
}

// FlushAndReset returns the current count and resets the counter.
func (c *UniqueTraceCounter) FlushAndReset() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	count := len(c.set)
	clear(c.set)
	return count
}
