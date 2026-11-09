// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync"
)

// Predicate bitmask flags.
const (
	PredFlagHasError  uint16 = 1 << 0
	PredFlagPredError uint16 = 1 << 1
	PredFlagHttpError uint16 = 1 << 2
	PredFlagBizError  uint16 = 1 << 3
	PredFlagTraceKeep uint16 = 1 << 4
)

// TraceMeta is a compact representation of an active trace or log group in the TimeWheel.
// It holds all decision-relevant metadata in scalar fields, completely decoupled from
// the large points payload bytes.
type TraceMeta struct {
	GroupIDHash   uint64
	RawGroupID    string
	Token         string
	DataType      string
	GroupKey      string
	Source        string
	ConfigVersion int64

	FirstSeenNano int64  // Unix nano timestamp when the first span arrived
	HardExpireSec uint32 // Absolute deadline after which the trace MUST be evaluated
	SoftExpireSec uint32 // Current slot expiry in the time wheel
	SlotIdx       int    // Current slot index in the time wheel array

	PointCount             int32
	TraceStartTimeUnixNano int64
	TraceEndTimeUnixNano   int64
	MaxPointTimeUnixNano   int64

	// Predicate summary flags & durations
	PredMask                uint16
	PredicateSummaryVersion uint32
	MaxSpanDurationUs       int64
	RootDurationUs          int64
	MaxNonrootDurationUs    int64

	// Payload handles bytes storage (memory pooled buffer or disk spill)
	Payload PayloadHandle
}

var traceMetaPool = sync.Pool{
	New: func() interface{} {
		return &TraceMeta{}
	},
}

// AllocTraceMeta borrows a TraceMeta from the pool.
func AllocTraceMeta(packet *DataPacket, hardExpireSec, softExpireSec uint32, slotIdx int, nowNano int64) *TraceMeta {
	meta := traceMetaPool.Get().(*TraceMeta)
	meta.Reset()

	meta.GroupIDHash = packet.GroupIdHash
	meta.RawGroupID = packet.RawGroupId
	meta.Token = packet.Token
	meta.DataType = packet.DataType
	meta.GroupKey = packet.GroupKey
	meta.Source = packet.Source
	meta.ConfigVersion = packet.ConfigVersion
	meta.FirstSeenNano = nowNano
	meta.HardExpireSec = hardExpireSec
	meta.SoftExpireSec = softExpireSec
	meta.SlotIdx = slotIdx
	meta.PointCount = packet.PointCount
	meta.TraceStartTimeUnixNano = packet.TraceStartTimeUnixNano
	meta.TraceEndTimeUnixNano = packet.TraceEndTimeUnixNano
	meta.MaxPointTimeUnixNano = packet.MaxPointTimeUnixNano
	meta.PredicateSummaryVersion = packet.PredicateSummaryVersion
	meta.MaxSpanDurationUs = packet.MaxSpanDurationUs
	meta.RootDurationUs = packet.RootDurationUs
	meta.MaxNonrootDurationUs = packet.MaxNonrootDurationUs

	meta.setPredFlags(packet)
	return meta
}

// FreeTraceMeta returns a TraceMeta to the pool after releasing its payload.
func FreeTraceMeta(m *TraceMeta) {
	if m == nil {
		return
	}
	if m.Payload != nil {
		m.Payload.Release()
		m.Payload = nil
	}
	m.Reset()
	traceMetaPool.Put(m)
}

// Reset clears all fields.
func (m *TraceMeta) Reset() {
	m.GroupIDHash = 0
	m.RawGroupID = ""
	m.Token = ""
	m.DataType = ""
	m.GroupKey = ""
	m.Source = ""
	m.ConfigVersion = 0
	m.FirstSeenNano = 0
	m.HardExpireSec = 0
	m.SoftExpireSec = 0
	m.SlotIdx = 0
	m.PointCount = 0
	m.TraceStartTimeUnixNano = 0
	m.TraceEndTimeUnixNano = 0
	m.MaxPointTimeUnixNano = 0
	m.PredMask = 0
	m.PredicateSummaryVersion = 0
	m.MaxSpanDurationUs = 0
	m.RootDurationUs = 0
	m.MaxNonrootDurationUs = 0
	m.Payload = nil
}

func (m *TraceMeta) setPredFlags(packet *DataPacket) {
	if packet.HasError {
		m.PredMask |= PredFlagHasError
	}
	if packet.PredError {
		m.PredMask |= PredFlagPredError
	}
	if packet.PredHttpError {
		m.PredMask |= PredFlagHttpError
	}
	if packet.PredBizError {
		m.PredMask |= PredFlagBizError
	}
	if packet.PredTraceKeep {
		m.PredMask |= PredFlagTraceKeep
	}
}

// Merge incoming packet's metadata into this TraceMeta.
func (m *TraceMeta) Merge(packet *DataPacket) {
	if packet == nil {
		return
	}
	m.PointCount += packet.PointCount
	m.setPredFlags(packet)

	if packet.PredicateSummaryVersion >= CurrentPredicateSummaryVersion &&
		m.PredicateSummaryVersion >= CurrentPredicateSummaryVersion {
		m.PredicateSummaryVersion = CurrentPredicateSummaryVersion
	} else {
		m.PredicateSummaryVersion = 0
	}

	if packet.MaxSpanDurationUs > m.MaxSpanDurationUs {
		m.MaxSpanDurationUs = packet.MaxSpanDurationUs
	}
	if packet.RootDurationUs > m.RootDurationUs {
		m.RootDurationUs = packet.RootDurationUs
	}
	if packet.MaxNonrootDurationUs > m.MaxNonrootDurationUs {
		m.MaxNonrootDurationUs = packet.MaxNonrootDurationUs
	}

	if packet.TraceStartTimeUnixNano > 0 {
		if m.TraceStartTimeUnixNano == 0 || packet.TraceStartTimeUnixNano < m.TraceStartTimeUnixNano {
			m.TraceStartTimeUnixNano = packet.TraceStartTimeUnixNano
		}
	}
	if packet.TraceEndTimeUnixNano > m.TraceEndTimeUnixNano {
		m.TraceEndTimeUnixNano = packet.TraceEndTimeUnixNano
	}
	if packet.MaxPointTimeUnixNano > m.MaxPointTimeUnixNano {
		m.MaxPointTimeUnixNano = packet.MaxPointTimeUnixNano
	}
	if m.RawGroupID == "" {
		m.RawGroupID = packet.RawGroupId
	}
	if packet.ConfigVersion > m.ConfigVersion {
		m.ConfigVersion = packet.ConfigVersion
	}
	if m.Source == "" {
		m.Source = packet.Source
	}
}

// HasError returns whether error flags are set.
func (m *TraceMeta) HasError() bool {
	return (m.PredMask & (PredFlagHasError | PredFlagPredError)) != 0
}

// HasHttpError returns whether http error flags are set.
func (m *TraceMeta) HasHttpError() bool {
	return (m.PredMask & PredFlagHttpError) != 0
}

// HasBizError returns whether business error flags are set.
func (m *TraceMeta) HasBizError() bool {
	return (m.PredMask & PredFlagBizError) != 0
}

// HasTraceKeep returns whether trace keep flag is set.
func (m *TraceMeta) HasTraceKeep() bool {
	return (m.PredMask & PredFlagTraceKeep) != 0
}
