// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync/atomic"
)

// SamplingHook provides optional observability and derived metric interceptors
// called at key lifecycle points in the sampling pipeline.
type SamplingHook interface {
	// OnIngest is called immediately after a packet is admitted into the sampler.
	OnIngest(packet *DataPacket)

	// OnDecision is called when a trace expires and a keep/drop decision is made.
	OnDecision(meta *TraceMeta, action PipelineAction, ruleName string)
}

// BuiltinMetricsHook is a thread-safe atomic counter hook providing production telemetry.
type BuiltinMetricsHook struct {
	TotalIngestedPackets atomic.Int64
	TotalIngestedSpans   atomic.Int64
	TotalKeptTraces      atomic.Int64
	TotalDroppedTraces   atomic.Int64
	TotalErrorTraces     atomic.Int64
	TotalSlowTraces      atomic.Int64
}

// NewBuiltinMetricsHook creates a new metrics hook.
func NewBuiltinMetricsHook() *BuiltinMetricsHook {
	return &BuiltinMetricsHook{}
}

// OnIngest increments ingest counters.
func (h *BuiltinMetricsHook) OnIngest(packet *DataPacket) {
	if packet == nil {
		return
	}
	h.TotalIngestedPackets.Add(1)
	if packet.PointCount > 0 {
		h.TotalIngestedSpans.Add(int64(packet.PointCount))
	}
}

// OnDecision records sampling outcomes.
func (h *BuiltinMetricsHook) OnDecision(meta *TraceMeta, action PipelineAction, _ string) {
	if meta == nil {
		return
	}
	if action == PipelineActionKeep {
		h.TotalKeptTraces.Add(1)
		if meta.HasError() {
			h.TotalErrorTraces.Add(1)
		}
		if meta.MaxSpanDurationUs >= 500000 { // >= 500ms
			h.TotalSlowTraces.Add(1)
		}
	} else {
		h.TotalDroppedTraces.Add(1)
	}
}
