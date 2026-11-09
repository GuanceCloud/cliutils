// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync"
	"time"
)

const timeWheelSlots = 3600

// Shard manages an isolated partition of active traces with an array-based ring time wheel.
type Shard struct {
	mu         sync.Mutex
	currentPos int
	slots      [timeWheelSlots][]uint64
	activeMap  map[uint64]*TraceMeta

	budget         *MemoryBudget
	spiller        PayloadSpiller
	spillThreshold int64
}

// NewShard creates an initialized time wheel shard.
func NewShard(budget *MemoryBudget, spiller PayloadSpiller, spillThreshold int64) *Shard {
	s := &Shard{
		activeMap:      make(map[uint64]*TraceMeta),
		budget:         budget,
		spiller:        spiller,
		spillThreshold: spillThreshold,
	}
	for i := range s.slots {
		s.slots[i] = make([]uint64, 0, 16)
	}
	return s
}

// Ingest routes and merges an incoming packet into the shard's time wheel.
func (s *Shard) Ingest(packet *DataPacket, ttlSec, maxHoldSec int, now time.Time) error {
	if packet == nil {
		return ErrPacketNil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	nowUnix := uint32(now.Unix())
	nowNano := now.UnixNano()
	key := packet.GroupIdHash

	if meta, exists := s.activeMap[key]; exists {
		// Existing trace: merge metadata and payload
		meta.Merge(packet)

		// Append points payload
		if len(packet.PointsPayload) > 0 {
			if err := meta.Payload.Append(packet.PointsPayload); err != nil {
				return err
			}
		}

		// Check if payload should be spilled to disk based on threshold or watermark
		if !meta.Payload.IsSpilled() && s.spiller != nil {
			if s.budget.ShouldSpill(meta.Payload.Size(), s.spillThreshold) {
				spilledHandle, err := meta.Payload.SpillTo(s.spiller)
				if err == nil {
					meta.Payload = spilledHandle
				}
			}
		}

		// Bounded Leases: candidate new expiry bounded by HardExpireSec
		remHard := int64(meta.HardExpireSec) - int64(nowUnix)
		if remHard > 0 {
			deltaSec := ttlSec
			if deltaSec > int(remHard) {
				deltaSec = int(remHard)
			}
			newSlot := (s.currentPos + deltaSec) % timeWheelSlots
			if newSlot != meta.SlotIdx {
				s.removeFromSlot(meta.SlotIdx, key)
				meta.SlotIdx = newSlot
				meta.SoftExpireSec = nowUnix + uint32(deltaSec)
				s.slots[newSlot] = append(s.slots[newSlot], key)
			}
		}
		return nil
	}

	// New trace: calculate hard deadline and relative soft slot
	hardExpireSec := nowUnix + uint32(maxHoldSec)
	deltaSec := ttlSec
	if deltaSec > maxHoldSec {
		deltaSec = maxHoldSec
	}
	softExpireSec := nowUnix + uint32(deltaSec)
	slotIdx := (s.currentPos + deltaSec) % timeWheelSlots

	meta := AllocTraceMeta(packet, hardExpireSec, softExpireSec, slotIdx, nowNano)

	// Create memory payload handle using pooled buffers
	memHandle := NewMemoryPayloadHandle(packet.PointsPayload, GlobalTieredPool, s.budget)
	meta.Payload = memHandle

	// Check if new trace should be immediately spilled (e.g. jumbo packet or yellow watermark)
	if s.spiller != nil && s.budget.ShouldSpill(meta.Payload.Size(), s.spillThreshold) {
		spilledHandle, err := meta.Payload.SpillTo(s.spiller)
		if err == nil {
			meta.Payload = spilledHandle
		}
	}

	// Register in time wheel and activeMap
	s.slots[slotIdx] = append(s.slots[slotIdx], key)
	s.activeMap[key] = meta
	return nil
}

// AdvanceSlot ticks the shard forward by 1 second and drains expired traces.
func (s *Shard) AdvanceSlot() []*TraceMeta {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentPos = (s.currentPos + 1) % timeWheelSlots
	currKeys := s.slots[s.currentPos]
	if len(currKeys) == 0 {
		return nil
	}

	expired := make([]*TraceMeta, 0, len(currKeys))
	for _, key := range currKeys {
		if meta, ok := s.activeMap[key]; ok {
			expired = append(expired, meta)
			delete(s.activeMap, key)
		}
	}

	// Reset slot slice without reallocating backing array
	s.slots[s.currentPos] = s.slots[s.currentPos][:0]
	return expired
}

func (s *Shard) removeFromSlot(slot int, key uint64) {
	if slot < 0 || slot >= timeWheelSlots {
		return
	}
	keys := s.slots[slot]
	for i, k := range keys {
		if k == key {
			// Swap with last element and trim
			last := len(keys) - 1
			keys[i] = keys[last]
			s.slots[slot] = keys[:last]
			return
		}
	}
}

// ActiveCount returns current active traces in this shard.
func (s *Shard) ActiveCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.activeMap)
}

// Close releases all active traces and buffers in this shard.
func (s *Shard) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, meta := range s.activeMap {
		FreeTraceMeta(meta)
		delete(s.activeMap, key)
	}
	for i := range s.slots {
		s.slots[i] = s.slots[i][:0]
	}
}
