// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import "github.com/GuanceCloud/cliutils/point"

const (
	maxTracePayloadFragments = 256
	maxTraceFragmentBytes    = 8 << 20
)

type tracePayloadFragment struct {
	payload     []byte
	compression int32
	pointCount  int32
}

// EnableTracePayloadFragments buffers compressed trace fragments until the group expires.
// Call before ingestion starts. Ingest takes ownership of packet payload bytes.
// Groups exceeding the fragment bounds or mixing protocols use budgeted materialization.
// Payload spilling keeps the legacy merge path.
func (s *GlobalSampler) EnableTracePayloadFragments() {
	if s != nil {
		s.tracePayloadFragments = true
	}
}

func (s *GlobalSampler) canBufferTracePacket(packet *DataPacket) bool {
	return s.tracePayloadFragments && s.spiller == nil && packet.DataType == point.STracing &&
		packet.PayloadCompression == PayloadCompressionZstd && len(packet.PointsPayload) > 0 &&
		packet.PredicateSummaryVersion == CurrentPredicateSummaryVersion && len(packet.PointsPayload) <= maxTraceFragmentBytes
}

// compactTracePayload detaches significantly oversized backing arrays before long-lived retention.
// Protobuf-decoded v2 packets are already compact and do not need another copy.
func compactTracePayload(packet *DataPacket) {
	if packet.DataType != point.STracing || len(packet.PointsPayload) == 0 ||
		cap(packet.PointsPayload)-len(packet.PointsPayload) <= len(packet.PointsPayload) {
		return
	}
	compact := make([]byte, len(packet.PointsPayload))
	copy(compact, packet.PointsPayload)
	packet.PointsPayload = compact
}

func (dg *DataGroup) appendTraceFragment(packet *DataPacket) {
	dg.fragments = append(dg.fragments, tracePayloadFragment{
		payload: packet.PointsPayload, compression: packet.PayloadCompression, pointCount: packet.PointCount,
	})
	dg.fragmentBytes += len(packet.PointsPayload)
}

func (s *GlobalSampler) mergeTraceGroupPayload(dg *DataGroup, packet *DataPacket) error {
	if len(dg.fragments) > 0 && s.canBufferTracePacket(packet) && len(dg.fragments) < maxTracePayloadFragments &&
		len(packet.PointsPayload) <= maxTraceFragmentBytes-dg.fragmentBytes {
		compactTracePayload(packet)
		dg.appendTraceFragment(packet)
		return nil
	}
	if len(dg.fragments) == 0 && (!s.tracePayloadFragments || dg.dataType != point.STracing || s.spiller != nil) {
		return mergePacketPayload(dg.packet, packet)
	}
	return mergeBoundedTracePayload(dg, packet)
}

func (s *GlobalSampler) traceFragmentOutcome(dg *DataGroup, fragmented bool) (*TailSamplingOutcome, error) {
	source := dg.packet
	plan := s.decisionPlanFor(dg.dataType, source.Token, source.GroupKey)
	if !fragmented || plan == nil || !plan.fast || !packetHasSpanPredicates(source) {
		// Unsupported rules keep the existing evaluator and rule priority unchanged.
		return nil, dg.materializeTraceFragments(true)
	}
	matched, index, kept := decideWithPredicates(source, plan)
	outcome := &TailSamplingOutcome{SourcePacket: source, Decision: DerivedMetricDecisionDropped}
	if matched {
		outcome.MatchedRule = tailSamplingMatchedRule(plan, index)
	}
	if kept == nil {
		return outcome, nil
	}
	outcome.Decision = DerivedMetricDecisionKept
	outcome.Packets = make([]*DataPacket, 0, len(dg.fragments))
	for _, fragment := range dg.fragments {
		packet := *source
		packet.PointsPayload = fragment.payload
		packet.PayloadCompression = fragment.compression
		packet.PointCount = fragment.pointCount
		outcome.Packets = append(outcome.Packets, &packet)
	}
	return outcome, nil
}
