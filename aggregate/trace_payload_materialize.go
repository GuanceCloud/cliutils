// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"errors"
	"fmt"
	"sync"

	"github.com/klauspost/compress/zstd"
)

const (
	// A materialized group must fit Dataway's existing 64 MiB send budget.
	maxTraceMaterializedBytes int64 = 64 << 20
	// Account conservatively for decoded fragments, joined output, encoding and compaction.
	traceMaterializationWorkFactor int64 = 4
	// DecodeAll can grow by a block before rejecting a false FCS. Reserve bounded
	// output slack separately; codec-owned history/tables are not payload buffers.
	traceDecodeBlockSlackBytes         int64 = 1 << 20
	maxTraceMaterializationBytes             = traceMaterializationWorkFactor*maxTraceMaterializedBytes + traceDecodeBlockSlackBytes
	maxTraceMaterializationConcurrency       = 4
)

// ErrTraceMaterializationBudgetExceeded indicates that a group cannot be safely materialized.
var ErrTraceMaterializationBudgetExceeded = errors.New("trace payload materialization budget exceeded")

// The work budget is shared by all samplers, shard/expiry and bounded-decode callers in this process.
var sharedTraceMaterializationBudget = newTraceMaterializationBudget(maxTraceMaterializationBytes, maxTraceMaterializationConcurrency)

type traceMaterializationBudget struct {
	mu          sync.Mutex
	changed     *sync.Cond
	capacity    int64
	used        int64
	active      int
	concurrency int
}

func newTraceMaterializationBudget(capacity int64, concurrency int) *traceMaterializationBudget {
	b := &traceMaterializationBudget{capacity: capacity, concurrency: concurrency}
	b.changed = sync.NewCond(&b.mu)
	return b
}

func (b *traceMaterializationBudget) reserve(bytes int64) error {
	if bytes < 0 || bytes > b.capacity || b.concurrency <= 0 {
		return ErrTraceMaterializationBudgetExceeded
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	for bytes > b.capacity-b.used || b.active >= b.concurrency {
		b.changed.Wait()
	}
	b.used += bytes
	b.active++
	return nil
}

func (b *traceMaterializationBudget) release(bytes int64) {
	b.mu.Lock()
	b.used -= bytes
	b.active--
	b.changed.Broadcast()
	b.mu.Unlock()
}

func tracePayloadWorkBytes(size int64) int64 {
	return traceMaterializationWorkFactor*size + traceDecodeBlockSlackBytes
}

// Use a maximum-sized encoding destination so output growth stays in the work reservation.
// The public compressor keeps its existing allocation behavior.
func compressMaterializedTracePayload(payload []byte) ([]byte, int32, error) {
	if len(payload) == 0 {
		return payload, PayloadCompressionNone, nil
	}
	enc := zstdEncoderPool.Get().(*zstd.Encoder)
	compressed := enc.EncodeAll(payload, make([]byte, 0, enc.MaxEncodedSize(len(payload))))
	zstdEncoderPool.Put(enc)
	if len(compressed) >= len(payload) {
		return payload, PayloadCompressionNone, nil
	}
	return compressed, PayloadCompressionZstd, nil
}

func traceMaterializedSize(fragments []tracePayloadFragment) (int64, error) {
	var total int64
	for _, fragment := range fragments {
		size, err := PointsPayloadDecodedSize(fragment.payload, fragment.compression)
		if err != nil {
			return 0, err
		}
		if size > maxTraceMaterializedBytes-total {
			return 0, fmt.Errorf("%w: decoded group exceeds %d bytes", ErrTraceMaterializationBudgetExceeded, maxTraceMaterializedBytes)
		}
		total += size
	}
	return total, nil
}

func (dg *DataGroup) materializeTraceFragments(compress bool) error {
	return dg.materializeTraceFragmentsWithBudget(compress, sharedTraceMaterializationBudget)
}

func (dg *DataGroup) materializeTraceFragmentsWithBudget(compress bool, budget *traceMaterializationBudget) error {
	if len(dg.fragments) == 0 {
		return nil
	}
	declared, err := traceMaterializedSize(dg.fragments)
	if err != nil {
		return err
	}
	workBytes := tracePayloadWorkBytes(declared)
	if err := budget.reserve(workBytes); err != nil {
		return err
	}
	defer budget.release(workBytes)

	decoded := make([][]byte, len(dg.fragments))
	actual := 0
	for i, fragment := range dg.fragments {
		size, err := PointsPayloadDecodedSize(fragment.payload, fragment.compression)
		if err != nil {
			return err
		}
		payload, err := decompressPointsPayloadWithLimit(fragment.payload, fragment.compression, size)
		if err != nil {
			return err
		}
		decoded[i] = payload
		actual += len(payload)
	}
	// The legacy single-packet result needs validation but no extra copy or encoding.
	if len(dg.fragments) == 1 {
		dg.fragments = nil
		dg.fragmentBytes = 0
		return nil
	}
	// Only validated lengths may determine the joined allocation.
	merged := make([]byte, 0, actual)
	for i, payload := range decoded {
		merged = append(merged, payload...)
		decoded[i] = nil
	}
	compression := PayloadCompressionNone
	if compress {
		merged, compression, err = compressMaterializedTracePayload(merged)
		if err != nil {
			return err
		}
	}
	dg.packet.PointsPayload = merged
	dg.packet.PayloadCompression = compression
	compactTracePayload(dg.packet)
	dg.fragments = nil
	dg.fragmentBytes = 0
	return nil
}

func mergeBoundedTracePayload(dg *DataGroup, packet *DataPacket) error {
	fragments := dg.fragments
	if len(fragments) == 0 {
		fragments = []tracePayloadFragment{{payload: dg.packet.PointsPayload,
			compression: dg.packet.PayloadCompression, pointCount: dg.packet.PointCount}}
	}
	fragments = append(append(make([]tracePayloadFragment, 0, len(fragments)+1), fragments...), tracePayloadFragment{
		payload: packet.PointsPayload, compression: packet.PayloadCompression, pointCount: packet.PointCount,
	})
	// Preserve the direct raw append path, while still checking its cumulative allocation.
	if len(dg.fragments) == 0 && dg.packet.PayloadCompression == PayloadCompressionNone && packet.PayloadCompression == PayloadCompressionNone {
		size, err := traceMaterializedSize(fragments)
		if err != nil {
			return err
		}
		workBytes := tracePayloadWorkBytes(size)
		if err := sharedTraceMaterializationBudget.reserve(workBytes); err != nil {
			return err
		}
		defer sharedTraceMaterializationBudget.release(workBytes)
		return mergePacketPayload(dg.packet, packet)
	}
	// Build a candidate, so any admission/decode error leaves every old fragment intact.
	metadata := *dg.packet
	candidate := &DataGroup{packet: &metadata, fragments: fragments}
	if err := candidate.materializeTraceFragments(true); err != nil {
		return err
	}
	dg.packet.PointsPayload = candidate.packet.PointsPayload
	dg.packet.PayloadCompression = candidate.packet.PayloadCompression
	dg.fragments = nil
	dg.fragmentBytes = 0
	return nil
}
