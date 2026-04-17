// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDHelpers(t *testing.T) {
	require.True(t, IsZeroID(nil))
	require.True(t, IsZeroID([]byte{0, 0, 0}))
	require.False(t, IsZeroID([]byte{0, 1}))

	require.Equal(t, "0", HexID(nil))
	require.Equal(t, "0102", HexID([]byte{1, 2}))
	require.Equal(t, "AQID", Base64RawID([]byte{1, 2, 3}))

	v, ok := DecimalIDFromLast8Bytes([]byte{0, 0, 0, 0, 0, 0, 0, 9})
	require.True(t, ok)
	require.Equal(t, "9", v)
}
