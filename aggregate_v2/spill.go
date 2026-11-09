// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"sync"
)

// PayloadHandle defines the storage abstraction for points payload.
type PayloadHandle interface {
	Append(chunk []byte) error
	Bytes() ([]byte, error)
	Size() int64
	Release()
	IsSpilled() bool
	SpillTo(spiller PayloadSpiller) (PayloadHandle, error)
}

// MemoryPayloadHandle implements in-memory payload storage using pooled buffers.
type MemoryPayloadHandle struct {
	buf    *PooledBuffer
	budget *MemoryBudget
}

// NewMemoryPayloadHandle creates a new memory-backed payload handle.
func NewMemoryPayloadHandle(initialChunk []byte, pool *TieredBufferPool, budget *MemoryBudget) *MemoryPayloadHandle {
	if pool == nil {
		pool = GlobalTieredPool
	}
	buf := pool.Get(len(initialChunk))
	if len(initialChunk) > 0 {
		buf.Append(initialChunk)
		if budget != nil {
			budget.AddResident(int64(len(initialChunk)))
		}
	}
	return &MemoryPayloadHandle{
		buf:    buf,
		budget: budget,
	}
}

// Append appends a compressed chunk to the flat in-memory buffer.
func (h *MemoryPayloadHandle) Append(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	if h.buf == nil {
		h.buf = GlobalTieredPool.Get(len(chunk))
	}
	h.buf.Append(chunk)
	if h.budget != nil {
		h.budget.AddResident(int64(len(chunk)))
	}
	return nil
}

// Bytes returns the continuous byte payload (RFC 8878 concatenated Zstd frames).
func (h *MemoryPayloadHandle) Bytes() ([]byte, error) {
	if h.buf == nil {
		return nil, nil
	}
	return h.buf.Data, nil
}

// Size returns total resident payload size in bytes.
func (h *MemoryPayloadHandle) Size() int64 {
	if h.buf == nil {
		return 0
	}
	return int64(len(h.buf.Data))
}

// Release releases the pooled buffer and discounts resident memory.
func (h *MemoryPayloadHandle) Release() {
	if h.buf != nil {
		size := int64(len(h.buf.Data))
		h.buf.Release()
		h.buf = nil
		if h.budget != nil {
			h.budget.SubResident(size)
		}
	}
}

// IsSpilled returns false.
func (h *MemoryPayloadHandle) IsSpilled() bool {
	return false
}

// SpillTo offloads the payload to the provided spiller and returns a SpillPayloadHandle.
func (h *MemoryPayloadHandle) SpillTo(spiller PayloadSpiller) (PayloadHandle, error) {
	if spiller == nil || h.buf == nil || len(h.buf.Data) == 0 {
		return h, nil
	}

	size := int64(len(h.buf.Data))
	key, err := spiller.Put(h.buf.Data)
	if err != nil {
		return h, fmt.Errorf("%w: %v", ErrSpillFailed, err)
	}

	// Release memory buffer
	h.Release()

	if h.budget != nil {
		h.budget.AddSpilled(size)
	}

	return &SpillPayloadHandle{
		key:     key,
		size:    size,
		spiller: spiller,
		budget:  h.budget,
	}, nil
}

// SpillPayloadHandle represents a payload that has been offloaded to disk.
type SpillPayloadHandle struct {
	mu      sync.Mutex
	key     string
	size    int64
	spiller PayloadSpiller
	budget  *MemoryBudget
}

// Append appends a chunk to a spilled payload by re-hydrating or direct append.
func (h *SpillPayloadHandle) Append(chunk []byte) error {
	if len(chunk) == 0 {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := h.spiller.Get(h.key)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrHydrationFailed, err)
	}

	merged := append(data, chunk...)
	oldKey := h.key
	newKey, err := h.spiller.Put(merged)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSpillFailed, err)
	}

	h.key = newKey
	delta := int64(len(chunk))
	h.size += delta
	if h.budget != nil {
		h.budget.AddSpilled(delta)
	}

	_ = h.spiller.Delete(oldKey)
	return nil
}

// Bytes reads the payload back from disk storage.
func (h *SpillPayloadHandle) Bytes() ([]byte, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.key == "" || h.spiller == nil {
		return nil, nil
	}
	data, err := h.spiller.Get(h.key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrHydrationFailed, err)
	}
	return data, nil
}

// Size returns the size in bytes of the spilled payload.
func (h *SpillPayloadHandle) Size() int64 {
	return h.size
}

// Release deletes the spilled file from disk and discounts stats.
func (h *SpillPayloadHandle) Release() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.key != "" && h.spiller != nil {
		_ = h.spiller.Delete(h.key)
		h.key = ""
		if h.budget != nil {
			h.budget.SubSpilled(h.size)
		}
	}
}

// IsSpilled returns true.
func (h *SpillPayloadHandle) IsSpilled() bool {
	return true
}

// SpillTo is a no-op on already spilled handles.
func (h *SpillPayloadHandle) SpillTo(_ PayloadSpiller) (PayloadHandle, error) {
	return h, nil
}
