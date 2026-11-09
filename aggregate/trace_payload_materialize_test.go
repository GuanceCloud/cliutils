// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"bytes"
	"encoding/binary"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func traceBudgetTestGroup(fragments []tracePayloadFragment) *DataGroup {
	return &DataGroup{dataType: point.STracing,
		packet: &DataPacket{DataType: point.STracing, PointsPayload: fragments[0].payload,
			PayloadCompression: fragments[0].compression, PointCount: int32(len(fragments))},
		fragments: fragments}
}

func TestTraceMaterializedSizeRejectsUntrustedClaims(t *testing.T) {
	for _, tc := range []struct {
		name      string
		claims    []uint64
		want      int64
		wantError bool
	}{
		{"single-boundary", []uint64{64 << 20}, 64 << 20, false},
		{"cumulative-boundary", []uint64{32 << 20, 32 << 20}, 64 << 20, false},
		{"single-over-limit", []uint64{64<<20 + 1}, 0, true},
		{"cumulative-over-limit", []uint64{32 << 20, 32<<20 + 1}, 0, true},
		{"terabyte-claim-header-only", []uint64{1 << 40}, 0, true},
		{"int64-overflow-header-only", []uint64{^uint64(0)}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fragments := make([]tracePayloadFragment, 0, len(tc.claims))
			for _, size := range tc.claims {
				fragments = append(fragments, tracePayloadFragment{payload: traceBudgetTestFrame(size), compression: PayloadCompressionZstd})
			}
			// Only exercise header admission; giant claims are never passed to an allocator.
			size, err := traceMaterializedSize(fragments)
			if tc.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, size)
		})
	}
}

func TestTraceMaterializationAdmissionAndErrorRelease(t *testing.T) {
	frame := traceBudgetTestFrame(1)
	fragments := []tracePayloadFragment{{payload: frame, compression: PayloadCompressionZstd}, {payload: frame, compression: PayloadCompressionZstd}}
	dg := traceBudgetTestGroup(fragments)
	budget := newTraceMaterializationBudget(tracePayloadWorkBytes(2)-1, 1)
	err := dg.materializeTraceFragmentsWithBudget(true, budget)
	require.ErrorIs(t, err, ErrTraceMaterializationBudgetExceeded)
	assert.Equal(t, int64(0), budget.used)
	assert.Zero(t, budget.active)
	assert.Len(t, dg.fragments, 2)
	assert.Equal(t, frame, dg.packet.PointsPayload)

	budget = newTraceMaterializationBudget(64<<20, 1)
	bad := traceBudgetTestFrame(4 << 20)
	dg = traceBudgetTestGroup([]tracePayloadFragment{{payload: bad, compression: PayloadCompressionZstd}, {payload: frame, compression: PayloadCompressionZstd}})
	err = dg.materializeTraceFragmentsWithBudget(true, budget)
	require.Error(t, err)
	assert.Zero(t, budget.used)
	assert.Zero(t, budget.active)
	assert.Len(t, dg.fragments, 2)
	assert.Equal(t, bad, dg.packet.PointsPayload)
	good := traceBudgetTestGroup(fragments)
	require.NoError(t, good.materializeTraceFragmentsWithBudget(true, budget))
	assert.Equal(t, []byte("xx"), good.packet.PointsPayload)
	assert.Zero(t, budget.used)
}

func TestTraceMaterializationWaitsForSharedAdmission(t *testing.T) {
	workBytes := tracePayloadWorkBytes(2)
	budget := newTraceMaterializationBudget(workBytes, 1)
	require.NoError(t, budget.reserve(workBytes))
	var once sync.Once
	release := func() { once.Do(func() { budget.release(workBytes) }) }
	defer release()
	frame := traceBudgetTestFrame(1)
	dg := traceBudgetTestGroup([]tracePayloadFragment{{payload: frame, compression: PayloadCompressionZstd}, {payload: frame, compression: PayloadCompressionZstd}})
	done := make(chan error, 1)
	go func() { done <- dg.materializeTraceFragmentsWithBudget(true, budget) }()
	select {
	case err := <-done:
		t.Fatalf("materialized without budget: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("materialization did not resume after admission")
	}
	assert.Zero(t, budget.used)
	assert.Zero(t, budget.active)
	assert.Equal(t, []byte("xx"), dg.packet.PointsPayload)
}

func TestBoundedDecodeAndMaterializationShareWorkBudget(t *testing.T) {
	budget := sharedTraceMaterializationBudget
	require.NoError(t, budget.reserve(budget.capacity))
	var once sync.Once
	release := func() { once.Do(func() { budget.release(budget.capacity) }) }
	defer release()
	done := make(chan error, 1)
	go func() {
		_, err := DecompressPointsPayloadWithLimit(traceBudgetTestFrame(4<<20), PayloadCompressionZstd, 4<<20)
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("public decode bypassed work admission: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	release()
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("public decode did not resume")
	}
	budget.mu.Lock()
	assert.Zero(t, budget.used, "decode errors must release the shared reservation")
	assert.Zero(t, budget.active)
	budget.mu.Unlock()

	// Leave exactly enough room for the group, but not a nested public decode.
	workBytes := tracePayloadWorkBytes(2)
	held := budget.capacity - workBytes
	require.NoError(t, budget.reserve(held))
	defer budget.release(held)
	frame := traceBudgetTestFrame(1)
	dg := traceBudgetTestGroup([]tracePayloadFragment{{payload: frame, compression: PayloadCompressionZstd}, {payload: frame, compression: PayloadCompressionZstd}})
	go func() { done <- dg.materializeTraceFragments(true) }()
	select {
	case err := <-done:
		require.NoError(t, err)
		assert.Equal(t, []byte("xx"), dg.packet.PointsPayload)
	case <-time.After(time.Second):
		t.Fatal("materialization recursively acquired its work budget")
	}
}

func TestTraceFallbackValidatesBeforeAllocatingJoinedPayload(t *testing.T) {
	for _, mode := range []string{"fragment-limit", "mixed-compression", "already-folded"} {
		t.Run(mode, func(t *testing.T) {
			bad := traceBudgetTestFrame(4 << 20)
			good := traceBudgetTestFrame(1)
			count := 2
			if mode == "fragment-limit" {
				count = maxTracePayloadFragments
			}
			fragments := make([]tracePayloadFragment, count)
			for i := range fragments {
				fragments[i] = tracePayloadFragment{payload: good, compression: PayloadCompressionZstd, pointCount: 1}
			}
			fragments[0].payload = bad
			dg := traceBudgetTestGroup(fragments)
			if mode == "already-folded" {
				dg.fragments = nil
			}
			beforePacket := *dg.packet
			beforeCount := len(dg.fragments)
			s := NewGlobalSampler(1, time.Second)
			s.EnableTracePayloadFragments()
			incoming := &DataPacket{DataType: point.STracing, PointsPayload: good, PayloadCompression: PayloadCompressionZstd,
				PredicateSummaryVersion: CurrentPredicateSummaryVersion, PointCount: 1}
			if mode == "mixed-compression" {
				incoming.PointsPayload = []byte("y")
				incoming.PayloadCompression = PayloadCompressionNone
			}
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			err := s.mergeTraceGroupPayload(dg, incoming)
			runtime.ReadMemStats(&after)
			require.Error(t, err)
			assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(6<<20), "fallback must validate before joined allocation")
			assert.Equal(t, beforePacket, *dg.packet)
			assert.Len(t, dg.fragments, beforeCount)
		})
	}
}

func TestTraceMaterializationFailureIsNotSamplingDrop(t *testing.T) {
	s := fragmentTestSampler(t, true, []*SamplingPipeline{{Name: "complex", Type: PipelineTypeCondition,
		Condition: `{ status = "error" AND duration > 1000 }`, Action: PipelineActionKeep}})
	frame := traceBudgetTestFrame(4 << 20)
	for range 2 {
		s.Ingest(&DataPacket{Token: "token", DataType: point.STracing, GroupKey: "trace_id", GroupIdHash: 42,
			PointCount: 1, PredicateSummaryVersion: CurrentPredicateSummaryVersion, PointsPayload: frame, PayloadCompression: PayloadCompressionZstd})
	}
	outcomes := s.TailSamplingFragmentOutcomes(s.AdvanceTime())
	require.Len(t, outcomes, 1)
	for _, outcome := range outcomes {
		assert.Equal(t, DerivedMetricDecisionUnknown, outcome.Decision)
		assert.Nil(t, outcome.Packet)
		assert.Empty(t, outcome.Packets)
		assert.Nil(t, outcome.MatchedRule)
	}
}

func TestDecompressPointsPayloadWithLimit(t *testing.T) {
	frame := traceBudgetTestFrame(1)
	decoded, err := DecompressPointsPayloadWithLimit(frame, PayloadCompressionZstd, 1)
	require.NoError(t, err)
	assert.Equal(t, []byte("x"), decoded)
	assert.Equal(t, 1, cap(decoded))
	_, err = DecompressPointsPayloadWithLimit(frame, PayloadCompressionZstd, 0)
	require.ErrorIs(t, err, ErrPayloadDecodedSizeExceeded)
	_, err = DecompressPointsPayloadWithLimit([]byte("xy"), PayloadCompressionNone, 1)
	require.ErrorIs(t, err, ErrPayloadDecodedSizeExceeded)
	_, err = DecompressPointsPayloadWithLimit(frame, PayloadCompressionZstd, -1)
	require.ErrorIs(t, err, ErrPayloadDecodedSizeExceeded)
	// A tiny declared output cannot authorize an oversized decoder window.
	window := []byte{0x28, 0xb5, 0x2f, 0xfd, 0xc0, 136} // 128 MiB window, eight-byte FCS.
	window = binary.LittleEndian.AppendUint64(window, 1)
	window = append(window, 9, 0, 0, 'x')
	_, err = DecompressPointsPayloadWithLimit(window, PayloadCompressionZstd, 1)
	require.ErrorIs(t, err, ErrPayloadDecodedSizeExceeded)
	// Structurally valid frame with no content-size field.
	unknown := []byte{0x28, 0xb5, 0x2f, 0xfd, 0, 0, 9, 0, 0, 'x'}
	_, err = DecompressPointsPayloadWithLimit(unknown, PayloadCompressionZstd, 1)
	require.ErrorIs(t, err, ErrPayloadDecodedSizeUnknown)
	legacy, err := DecompressPointsPayload(unknown, PayloadCompressionZstd)
	require.NoError(t, err, "legacy decoder retains unknown-size support")
	assert.Equal(t, []byte("x"), legacy)
	// FCS is one byte, but the raw block contains much more data.
	underdeclared := []byte{0x28, 0xb5, 0x2f, 0xfd, 0xc0, 40} // Non-single-segment, 32 KiB window.
	underdeclared = binary.LittleEndian.AppendUint64(underdeclared, 1)
	block := uint32(32<<10)<<3 | 1
	underdeclared = append(underdeclared, byte(block), byte(block>>8), byte(block>>16))
	underdeclared = append(underdeclared, bytes.Repeat([]byte("x"), 32<<10)...)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = DecompressPointsPayloadWithLimit(underdeclared, PayloadCompressionZstd, 1)
	runtime.ReadMemStats(&after)
	require.Error(t, err)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(traceDecodeBlockSlackBytes), "false FCS block growth stays within reserved slack")
	// A bad checksum also has to fail before a group can be materialized.
	checksum := append([]byte(nil), frame...)
	checksum[4] |= 4
	checksum = binary.LittleEndian.AppendUint32(checksum, 0)
	_, err = DecompressPointsPayloadWithLimit(checksum, PayloadCompressionZstd, 1)
	require.Error(t, err)
}
