// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync"
)

const (
	tier8K   = 8 << 10
	tier32K  = 32 << 10
	tier128K = 128 << 10
	tier512K = 512 << 10
)

const (
	tierIndex8K   = 0
	tierIndex32K  = 1
	tierIndex128K = 2
	tierIndex512K = 3
	tierIndexNone = -1
)

// PooledBuffer wraps a pooled byte slice with its tier metadata.
type PooledBuffer struct {
	Data []byte
	tier int
	pool *TieredBufferPool
}

// Len returns the current written length.
func (b *PooledBuffer) Len() int {
	if b == nil {
		return 0
	}
	return len(b.Data)
}

// Cap returns the buffer capacity.
func (b *PooledBuffer) Cap() int {
	if b == nil {
		return 0
	}
	return cap(b.Data)
}

// Reset resets the buffer length to 0 while keeping capacity.
func (b *PooledBuffer) Reset() {
	if b != nil {
		b.Data = b.Data[:0]
	}
}

// Append appends a byte chunk. If capacity is exceeded, it seamlessly upgrades
// to a larger tier from the pool, copies content, and returns the old buffer to its pool.
func (b *PooledBuffer) Append(chunk []byte) {
	if len(chunk) == 0 {
		return
	}
	needed := len(b.Data) + len(chunk)
	if needed <= cap(b.Data) {
		b.Data = append(b.Data, chunk...)
		return
	}

	// Upgrade tier if pooled
	if b.pool != nil {
		newBuf := b.pool.Get(needed)
		newBuf.Data = append(newBuf.Data, b.Data...)
		newBuf.Data = append(newBuf.Data, chunk...)

		// Swap underlying buffer
		oldTier := b.tier
		oldPool := b.pool
		oldData := b.Data

		b.Data = newBuf.Data
		b.tier = newBuf.tier
		b.pool = newBuf.pool

		// Put back old data into old pool
		oldPool.putRaw(oldTier, oldData)
		return
	}

	// Unpooled fallback
	b.Data = append(b.Data, chunk...)
}

// Release returns the buffer to its parent tiered pool.
func (b *PooledBuffer) Release() {
	if b == nil || b.pool == nil || b.tier == tierIndexNone {
		return
	}
	b.pool.Put(b)
}

// TieredBufferPool manages tiered pre-allocated byte buffers.
type TieredBufferPool struct {
	pool8K   sync.Pool
	pool32K  sync.Pool
	pool128K sync.Pool
	pool512K sync.Pool
}

// NewTieredBufferPool creates a new tiered buffer pool.
func NewTieredBufferPool() *TieredBufferPool {
	p := &TieredBufferPool{}
	p.pool8K = sync.Pool{
		New: func() any {
			return make([]byte, 0, tier8K)
		},
	}
	p.pool32K = sync.Pool{
		New: func() any {
			return make([]byte, 0, tier32K)
		},
	}
	p.pool128K = sync.Pool{
		New: func() any {
			return make([]byte, 0, tier128K)
		},
	}
	p.pool512K = sync.Pool{
		New: func() any {
			return make([]byte, 0, tier512K)
		},
	}
	return p
}

// GlobalTieredPool is the default shared pool instance for resident buffers.
var GlobalTieredPool = NewTieredBufferPool()

// Get borrows a buffer with at least minCap capacity.
func (p *TieredBufferPool) Get(minCap int) *PooledBuffer {
	if minCap <= tier8K {
		buf := p.pool8K.Get().([]byte)
		return &PooledBuffer{Data: buf[:0], tier: tierIndex8K, pool: p}
	}
	if minCap <= tier32K {
		buf := p.pool32K.Get().([]byte)
		return &PooledBuffer{Data: buf[:0], tier: tierIndex32K, pool: p}
	}
	if minCap <= tier128K {
		buf := p.pool128K.Get().([]byte)
		return &PooledBuffer{Data: buf[:0], tier: tierIndex128K, pool: p}
	}
	if minCap <= tier512K {
		buf := p.pool512K.Get().([]byte)
		return &PooledBuffer{Data: buf[:0], tier: tierIndex512K, pool: p}
	}

	// Beyond highest tier: direct allocation without pool retention to avoid bloat
	return &PooledBuffer{Data: make([]byte, 0, minCap), tier: tierIndexNone, pool: p}
}

// Put returns the buffer to the appropriate pool based on its tier.
func (p *TieredBufferPool) Put(b *PooledBuffer) {
	if b == nil || b.tier == tierIndexNone {
		return
	}
	p.putRaw(b.tier, b.Data)
	b.Data = nil
	b.tier = tierIndexNone
	b.pool = nil
}

func (p *TieredBufferPool) putRaw(tier int, data []byte) {
	if data == nil {
		return
	}
	data = data[:0]
	switch tier {
	case tierIndex8K:
		if cap(data) == tier8K {
			p.pool8K.Put(data)
		}
	case tierIndex32K:
		if cap(data) == tier32K {
			p.pool32K.Put(data)
		}
	case tierIndex128K:
		if cap(data) == tier128K {
			p.pool128K.Put(data)
		}
	case tierIndex512K:
		if cap(data) == tier512K {
			p.pool512K.Put(data)
		}
	}
}
