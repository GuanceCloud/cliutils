// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"encoding/binary"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A structurally complete zstd frame with one raw byte and an independently declared FCS.
func traceBudgetTestFrame(declared uint64) []byte {
	frame := []byte{0x28, 0xb5, 0x2f, 0xfd, 0xe0}
	frame = binary.LittleEndian.AppendUint64(frame, declared)
	return append(frame, 9, 0, 0, 'x')
}

func TestTraceMaterializationRejectsFalseSizeBeforeJoinedAllocation(t *testing.T) {
	const declared = 4 << 20 // Small, bounded probe; never attempts a real OOM.
	frame := traceBudgetTestFrame(declared)
	size, err := PointsPayloadDecodedSize(frame, PayloadCompressionZstd)
	require.NoError(t, err)
	require.EqualValues(t, declared, size, "header preflight does not validate decoded content")
	_, err = DecompressPointsPayload(traceBudgetTestFrame(1), PayloadCompressionZstd)
	require.NoError(t, err)
	dg := &DataGroup{packet: &DataPacket{PointsPayload: frame, PayloadCompression: PayloadCompressionZstd},
		fragments: []tracePayloadFragment{
			{payload: frame, compression: PayloadCompressionZstd, pointCount: 1},
			{payload: frame, compression: PayloadCompressionZstd, pointCount: 1},
		}}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	err = dg.materializeTraceFragments(false)
	runtime.ReadMemStats(&after)
	require.Error(t, err)
	allocated := after.TotalAlloc - before.TotalAlloc
	t.Logf("declared per fragment=%d, cumulative=%d, allocated=%d, error=%v", declared, 2*declared, allocated, err)
	assert.Less(t, allocated, uint64(declared+2<<20), "do not allocate the joined buffer before validating fragment contents")
	require.Len(t, dg.fragments, 2, "a failed materialization must preserve the existing group")
}
