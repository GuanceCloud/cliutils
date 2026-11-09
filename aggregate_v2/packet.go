// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"

	"github.com/VictoriaMetrics/easyproto"
)

// CurrentPredicateSummaryVersion is the active schema version for span predicate summaries.
const CurrentPredicateSummaryVersion uint32 = 1

var marshalerPool easyproto.MarshalerPool

// DataPacket is the wire-compatible data model representing a trace, log group, or RUM session.
// Field numbers 1..22 are byte-for-byte compatible with tsdata.proto without any proto dependencies.
type DataPacket struct {
	GroupIdHash             uint64 `json:"group_id_hash,omitempty"`
	RawGroupId              string `json:"raw_group_id,omitempty"`
	Token                   string `json:"token,omitempty"`
	Source                  string `json:"source,omitempty"`
	DataType                string `json:"data_type,omitempty"`
	ConfigVersion           int64  `json:"config_version,omitempty"`
	HasError                bool   `json:"has_error,omitempty"`
	GroupKey                string `json:"group_key,omitempty"`
	PointCount              int32  `json:"point_count,omitempty"`
	TraceStartTimeUnixNano  int64  `json:"trace_start_time_unix_nano,omitempty"`
	TraceEndTimeUnixNano    int64  `json:"trace_end_time_unix_nano,omitempty"`
	PointsPayload           []byte `json:"points_payload,omitempty"`
	MaxPointTimeUnixNano    int64  `json:"max_point_time_unix_nano,omitempty"`
	PayloadCompression      int32  `json:"payload_compression,omitempty"`
	PredError               bool   `json:"pred_error,omitempty"`
	PredHttpError           bool   `json:"pred_http_error,omitempty"`
	PredBizError            bool   `json:"pred_biz_error,omitempty"`
	PredTraceKeep           bool   `json:"pred_trace_keep,omitempty"`
	MaxSpanDurationUs       int64  `json:"max_span_duration_us,omitempty"`
	RootDurationUs          int64  `json:"root_duration_us,omitempty"`
	MaxNonrootDurationUs    int64  `json:"max_nonroot_duration_us,omitempty"`
	PredicateSummaryVersion uint32 `json:"predicate_summary_version,omitempty"`
}

// Reset resets the packet to its zero values.
func (m *DataPacket) Reset() { *m = DataPacket{} }

// String returns a readable string representation.
func (m *DataPacket) String() string {
	return fmt.Sprintf("DataPacket{group_id_hash=%d, raw_group_id=%q, points=%d, err=%t}",
		m.GroupIdHash, m.RawGroupId, m.PointCount, m.HasError)
}

// DataBatch represents a batch of DataPackets sent over HTTP.
type DataBatch struct {
	CollectorId string        `json:"collector_id,omitempty"`
	Packets     []*DataPacket `json:"packets,omitempty"`
}

// Reset resets the batch.
func (m *DataBatch) Reset() { *m = DataBatch{} }

// String returns a readable string representation.
func (m *DataBatch) String() string {
	return fmt.Sprintf("DataBatch{collector_id=%q, packets=%d}", m.CollectorId, len(m.Packets))
}

// MarshalDataPacket encodes a DataPacket to wire bytes using easyproto.
func MarshalDataPacket(p *DataPacket) ([]byte, error) {
	if p == nil {
		return nil, nil
	}
	m := marshalerPool.Get()
	defer marshalerPool.Put(m)

	mm := m.MessageMarshaler()
	appendPacketFields(mm, p)
	return m.Marshal(nil), nil
}

// MarshalDataBatch encodes a DataBatch to wire bytes using easyproto.
func MarshalDataBatch(b *DataBatch) ([]byte, error) {
	if b == nil {
		return nil, nil
	}
	m := marshalerPool.Get()
	defer marshalerPool.Put(m)

	mm := m.MessageMarshaler()
	if b.CollectorId != "" {
		mm.AppendString(1, b.CollectorId)
	}
	for _, p := range b.Packets {
		if p == nil {
			continue
		}
		pm := mm.AppendMessage(2)
		appendPacketFields(pm, p)
	}
	return m.Marshal(nil), nil
}

func appendPacketFields(mm *easyproto.MessageMarshaler, p *DataPacket) {
	if p.GroupIdHash != 0 {
		mm.AppendUint64(1, p.GroupIdHash)
	}
	if p.RawGroupId != "" {
		mm.AppendString(2, p.RawGroupId)
	}
	if p.Token != "" {
		mm.AppendString(3, p.Token)
	}
	if p.Source != "" {
		mm.AppendString(4, p.Source)
	}
	if p.DataType != "" {
		mm.AppendString(5, p.DataType)
	}
	if p.ConfigVersion != 0 {
		mm.AppendInt64(6, p.ConfigVersion)
	}
	if p.HasError {
		mm.AppendBool(7, p.HasError)
	}
	if p.GroupKey != "" {
		mm.AppendString(8, p.GroupKey)
	}
	if p.PointCount != 0 {
		mm.AppendInt32(9, p.PointCount)
	}
	if p.TraceStartTimeUnixNano != 0 {
		mm.AppendInt64(10, p.TraceStartTimeUnixNano)
	}
	if p.TraceEndTimeUnixNano != 0 {
		mm.AppendInt64(11, p.TraceEndTimeUnixNano)
	}
	if len(p.PointsPayload) > 0 {
		mm.AppendBytes(12, p.PointsPayload)
	}
	if p.MaxPointTimeUnixNano != 0 {
		mm.AppendInt64(13, p.MaxPointTimeUnixNano)
	}
	if p.PayloadCompression != 0 {
		mm.AppendInt32(14, p.PayloadCompression)
	}
	if p.PredError {
		mm.AppendBool(15, p.PredError)
	}
	if p.PredHttpError {
		mm.AppendBool(16, p.PredHttpError)
	}
	if p.PredBizError {
		mm.AppendBool(17, p.PredBizError)
	}
	if p.PredTraceKeep {
		mm.AppendBool(18, p.PredTraceKeep)
	}
	if p.MaxSpanDurationUs != 0 {
		mm.AppendInt64(19, p.MaxSpanDurationUs)
	}
	if p.RootDurationUs != 0 {
		mm.AppendInt64(20, p.RootDurationUs)
	}
	if p.MaxNonrootDurationUs != 0 {
		mm.AppendInt64(21, p.MaxNonrootDurationUs)
	}
	if p.PredicateSummaryVersion != 0 {
		mm.AppendUint32(22, p.PredicateSummaryVersion)
	}
}
