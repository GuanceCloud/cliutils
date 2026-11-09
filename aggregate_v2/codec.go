// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"sync"

	"github.com/VictoriaMetrics/easyproto"
)

var fcPool = sync.Pool{
	New: func() any {
		return &easyproto.FieldContext{}
	},
}

var batchPacketsPool = sync.Pool{
	New: func() any {
		return &BatchPackets{
			Packets: make([]*DataPacket, 0, 64),
		}
	},
}

// BatchPackets provides pooled, zero-allocation unmarshaling for DataBatch and DataPacket streams
// based on VictoriaMetrics/easyproto (inspired by point.BatchPoints in PR #149).
type BatchPackets struct {
	CollectorID string
	Packets     []*DataPacket
}

// NewBatchPackets borrows an initialized BatchPackets from the pool.
func NewBatchPackets() *BatchPackets {
	return batchPacketsPool.Get().(*BatchPackets)
}

// Reset clears packet states while preserving slice capacity.
func (bp *BatchPackets) Reset() {
	bp.CollectorID = ""
	for _, p := range bp.Packets {
		p.Reset()
	}
	bp.Packets = bp.Packets[:0]
}

// Release resets and returns the BatchPackets to the pool.
func (bp *BatchPackets) Release() {
	bp.Reset()
	batchPacketsPool.Put(bp)
}

// Unmarshal parses either a DataBatch (field 2 repeated) or a standalone DataPacket
// seamlessly with zero allocations.
func (bp *BatchPackets) Unmarshal(src []byte) error {
	if len(src) == 0 {
		return nil
	}

	// Fast detection: if first byte is 0x08, it's field 1 varint (GroupIdHash) of DataPacket
	if src[0] == 0x08 {
		packet := bp.allocPacket()
		return UnmarshalSinglePacket(src, packet)
	}

	// Otherwise, attempt unmarshaling as DataBatch
	if err := bp.UnmarshalBatch(src); err == nil && len(bp.Packets) > 0 {
		return nil
	}

	// Fallback: attempt unmarshaling as single DataPacket
	bp.Reset()
	packet := bp.allocPacket()
	return UnmarshalSinglePacket(src, packet)
}

// UnmarshalBatch parses a DataBatch (repeated DataPacket in field 2) using easyproto with zero allocations.
func (bp *BatchPackets) UnmarshalBatch(src []byte) error {
	fc := fcPool.Get().(*easyproto.FieldContext)
	defer fcPool.Put(fc)

	var err error
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for batch failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			s, ok := fc.String()
			if ok {
				bp.CollectorID = s
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read DataPacket message data")
			}
			packet := bp.allocPacket()
			if err := unmarshalDataPacketFields(fc, packet, data); err != nil {
				return err
			}
		}
	}
	return nil
}

// UnmarshalSinglePacket unmarshals a standalone DataPacket using easyproto without allocating if p is reused.
func UnmarshalSinglePacket(src []byte, p *DataPacket) error {
	if p == nil {
		return ErrPacketNil
	}
	p.Reset()

	fc := fcPool.Get().(*easyproto.FieldContext)
	defer fcPool.Put(fc)

	return unmarshalDataPacketFields(fc, p, src)
}

func (bp *BatchPackets) allocPacket() *DataPacket {
	if cap(bp.Packets) > len(bp.Packets) {
		bp.Packets = bp.Packets[:len(bp.Packets)+1]
		if bp.Packets[len(bp.Packets)-1] == nil {
			bp.Packets[len(bp.Packets)-1] = &DataPacket{}
		}
	} else {
		bp.Packets = append(bp.Packets, &DataPacket{})
	}
	packet := bp.Packets[len(bp.Packets)-1]
	packet.Reset()
	return packet
}

func unmarshalDataPacketFields(fc *easyproto.FieldContext, p *DataPacket, src []byte) (err error) {
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for DataPacket failed: %w", err)
		}
		applyField(fc, p, fc.FieldNum)
	}
	return nil
}

func applyField(fc *easyproto.FieldContext, p *DataPacket, fieldNum uint32) {
	switch fieldNum {
	case 1:
		v, ok := fc.Uint64()
		if ok {
			p.GroupIdHash = v
		}
	case 2:
		s, ok := fc.String()
		if ok {
			p.RawGroupId = s
		}
	case 3:
		s, ok := fc.String()
		if ok {
			p.Token = s
		}
	case 4:
		s, ok := fc.String()
		if ok {
			p.Source = s
		}
	case 5:
		s, ok := fc.String()
		if ok {
			p.DataType = s
		}
	case 6:
		v, ok := fc.Int64()
		if ok {
			p.ConfigVersion = v
		}
	case 7:
		v, ok := fc.Bool()
		if ok {
			p.HasError = v
		}
	case 8:
		s, ok := fc.String()
		if ok {
			p.GroupKey = s
		}
	case 9:
		v, ok := fc.Int32()
		if ok {
			p.PointCount = v
		}
	case 10:
		v, ok := fc.Int64()
		if ok {
			p.TraceStartTimeUnixNano = v
		}
	case 11:
		v, ok := fc.Int64()
		if ok {
			p.TraceEndTimeUnixNano = v
		}
	case 12:
		// PointsPayload: zero-copy reference to message slice
		b, ok := fc.Bytes()
		if ok {
			p.PointsPayload = b
		}
	case 13:
		v, ok := fc.Int64()
		if ok {
			p.MaxPointTimeUnixNano = v
		}
	case 14:
		v, ok := fc.Int32()
		if ok {
			p.PayloadCompression = v
		}
	case 15:
		v, ok := fc.Bool()
		if ok {
			p.PredError = v
		}
	case 16:
		v, ok := fc.Bool()
		if ok {
			p.PredHttpError = v
		}
	case 17:
		v, ok := fc.Bool()
		if ok {
			p.PredBizError = v
		}
	case 18:
		v, ok := fc.Bool()
		if ok {
			p.PredTraceKeep = v
		}
	case 19:
		v, ok := fc.Int64()
		if ok {
			p.MaxSpanDurationUs = v
		}
	case 20:
		v, ok := fc.Int64()
		if ok {
			p.RootDurationUs = v
		}
	case 21:
		v, ok := fc.Int64()
		if ok {
			p.MaxNonrootDurationUs = v
		}
	case 22:
		v, ok := fc.Uint32()
		if ok {
			p.PredicateSummaryVersion = v
		}
	}
}
