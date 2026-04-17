// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChunkStringByRuneLength(t *testing.T) {
	require.Equal(t, []string{"ab", "cd", "ef"}, ChunkStringByRuneLength("abcdef", 2))
	require.Equal(t, []string{"你a", "好b"}, ChunkStringByRuneLength("你a好b", 2))
	require.Nil(t, ChunkStringByRuneLength("", 2))
	require.Nil(t, ChunkStringByRuneLength("abc", 0))
}
