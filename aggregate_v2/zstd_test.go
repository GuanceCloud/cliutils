// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZstdFrameConcatenation(t *testing.T) {
	chunk1 := []byte("span-001: trace_id=123 service=auth duration=10ms; ")
	chunk2 := []byte("span-002: trace_id=123 service=order duration=25ms; ")
	chunk3 := []byte("span-003: trace_id=123 service=pay duration=80ms; ")

	frame1, err := CompressPayload(chunk1)
	require.NoError(t, err)
	frame2, err := CompressPayload(chunk2)
	require.NoError(t, err)
	frame3, err := CompressPayload(chunk3)
	require.NoError(t, err)

	// Direct binary concatenation without decompressing or recompressing!
	concatenated := ConcatZstdPayloads(nil, frame1)
	concatenated = ConcatZstdPayloads(concatenated, frame2)
	concatenated = ConcatZstdPayloads(concatenated, frame3)

	// Decompress concatenated stream
	decompressed, err := DecompressPayload(concatenated)
	require.NoError(t, err)

	expected := append(append(chunk1, chunk2...), chunk3...)
	assert.True(t, bytes.Equal(expected, decompressed), "decompressed payload must equal concatenation of original chunks")
}
