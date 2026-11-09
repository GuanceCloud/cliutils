// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync/atomic"
)

// WatermarkLevel represents the memory pressure level.
type WatermarkLevel int

const (
	// WatermarkGreen: normal resident in-memory retention.
	WatermarkGreen WatermarkLevel = iota
	// WatermarkYellow: proactive spilling of larger traces to disk.
	WatermarkYellow
	// WatermarkRed: backpressure rejection and urgent evictions.
	WatermarkRed
)

// MemoryBudget tracks resident payload bytes across the sampler and provides
// hard admission gates to prevent container OOMs.
type MemoryBudget struct {
	maxBytes      int64
	warningBytes  int64
	criticalBytes int64

	residentBytes atomic.Int64
	spilledBytes  atomic.Int64
	spilledTraces atomic.Int64
	rejections    atomic.Int64
}

// NewMemoryBudget creates a budget controller with specified maximum limit and watermarks.
func NewMemoryBudget(maxBytes int64, warningRatio, criticalRatio float64) *MemoryBudget {
	if maxBytes <= 0 {
		maxBytes = 2 << 30 // 2 GB default
	}
	if warningRatio <= 0 || warningRatio >= 1.0 {
		warningRatio = 0.70
	}
	if criticalRatio <= 0 || criticalRatio >= 1.0 || criticalRatio <= warningRatio {
		criticalRatio = 0.85
	}

	return &MemoryBudget{
		maxBytes:      maxBytes,
		warningBytes:  int64(float64(maxBytes) * warningRatio),
		criticalBytes: int64(float64(maxBytes) * criticalRatio),
	}
}

// FastCheck checks whether an incoming packet can be admitted into memory.
// Returns ErrBudgetExceeded if memory is at or above the critical watermark.
func (b *MemoryBudget) FastCheck(size int64) error {
	if b == nil || size <= 0 {
		return nil
	}
	current := b.residentBytes.Load()
	if current+size >= b.criticalBytes {
		b.rejections.Add(1)
		return ErrBudgetExceeded
	}
	return nil
}

// Level returns current watermark level.
func (b *MemoryBudget) Level() WatermarkLevel {
	if b == nil {
		return WatermarkGreen
	}
	cur := b.residentBytes.Load()
	if cur >= b.criticalBytes {
		return WatermarkRed
	}
	if cur >= b.warningBytes {
		return WatermarkYellow
	}
	return WatermarkGreen
}

// ShouldSpill reports whether a trace should be spilled based on size or memory pressure.
func (b *MemoryBudget) ShouldSpill(traceSize int64, spillThreshold int64) bool {
	if b == nil {
		return false
	}
	// Per-trace size threshold
	if spillThreshold > 0 && traceSize >= spillThreshold {
		return true
	}
	// Memory pressure threshold (yellow/red zone triggers spilling for any non-trivial trace)
	lvl := b.Level()
	if lvl >= WatermarkYellow && traceSize > 8<<10 { // > 8KB
		return true
	}
	return false
}

// AddResident records additional bytes in memory.
func (b *MemoryBudget) AddResident(delta int64) {
	if b != nil && delta > 0 {
		b.residentBytes.Add(delta)
	}
}

// SubResident decrements resident memory bytes.
func (b *MemoryBudget) SubResident(delta int64) {
	if b != nil && delta > 0 {
		b.residentBytes.Add(-delta)
	}
}

// AddSpilled tracks spilled bytes and trace count.
func (b *MemoryBudget) AddSpilled(delta int64) {
	if b != nil && delta > 0 {
		b.spilledBytes.Add(delta)
		b.spilledTraces.Add(1)
	}
}

// SubSpilled decrements spilled bytes and trace count.
func (b *MemoryBudget) SubSpilled(delta int64) {
	if b != nil && delta > 0 {
		b.spilledBytes.Add(-delta)
		b.spilledTraces.Add(-1)
	}
}

// ResidentBytes returns the current resident byte count.
func (b *MemoryBudget) ResidentBytes() int64 {
	if b == nil {
		return 0
	}
	return b.residentBytes.Load()
}

// SpilledBytes returns total bytes currently offloaded to disk.
func (b *MemoryBudget) SpilledBytes() int64 {
	if b == nil {
		return 0
	}
	return b.spilledBytes.Load()
}

// Rejections returns total rejection count.
func (b *MemoryBudget) Rejections() int64 {
	if b == nil {
		return 0
	}
	return b.rejections.Load()
}
