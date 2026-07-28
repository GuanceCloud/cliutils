// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package http

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPayloadObfuscationGzipCaesarV1SingleByte(t *testing.T) {
	assert.Equal(t, "X-Guance-Payload-Obfuscation", PayloadObfuscationHeader)
	assert.Equal(t, "gzip-caesar-v1", PayloadObfuscationGzipCaesarV1)

	body := []byte{250}

	require.NoError(t, ObfuscatePayload(PayloadObfuscationGzipCaesarV1, body))
	assert.Equal(t, []byte{4}, body)

	require.NoError(t, DeobfuscatePayload(PayloadObfuscationGzipCaesarV1, body))
	assert.Equal(t, []byte{250}, body)
}

func TestPayloadObfuscationGzipCaesarV1RoundTripBoundaries(t *testing.T) {
	for _, size := range []int{0, 1, 255, 256, 257, 1 << 20} {
		t.Run(fmt.Sprintf("length_%d", size), func(t *testing.T) {
			body := make([]byte, size)
			for i := range body {
				body[i] = byte(i)
			}
			original := append([]byte(nil), body...)

			require.NoError(t, ObfuscatePayload(PayloadObfuscationGzipCaesarV1, body))
			if len(body) > 256 {
				assert.True(t, bytes.Equal(original[256:], body[256:]), "bytes after the first 256 changed")
			}

			require.NoError(t, DeobfuscatePayload(PayloadObfuscationGzipCaesarV1, body))
			assert.True(t, bytes.Equal(original, body), "round trip changed a payload of length %d", size)
		})
	}
}

func TestPayloadObfuscationRejectsUnsupportedModeWithoutMutation(t *testing.T) {
	tests := []struct {
		name      string
		transform func(string, []byte) error
	}{
		{
			name:      "obfuscate",
			transform: ObfuscatePayload,
		},
		{
			name:      "deobfuscate",
			transform: DeobfuscatePayload,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := []byte{0, 1, 127, 128, 254, 255}
			original := append([]byte(nil), body...)

			err := test.transform("unknown-mode", body)

			require.ErrorIs(t, err, ErrUnsupportedPayloadObfuscation)
			assert.Equal(t, original, body)
		})
	}
}

func TestPayloadObfuscationGzipCaesarV1WrapsAllByteValues(t *testing.T) {
	original := make([]byte, 256)
	for i := range original {
		original[i] = byte(i)
	}

	t.Run("obfuscate", func(t *testing.T) {
		body := append([]byte(nil), original...)
		expected := append(append([]byte(nil), original[10:]...), original[:10]...)

		require.NoError(t, ObfuscatePayload(PayloadObfuscationGzipCaesarV1, body))
		assert.Equal(t, expected, body)
	})

	t.Run("deobfuscate", func(t *testing.T) {
		body := append([]byte(nil), original...)
		expected := append(append([]byte(nil), original[256-10:]...), original[:256-10]...)

		require.NoError(t, DeobfuscatePayload(PayloadObfuscationGzipCaesarV1, body))
		assert.Equal(t, expected, body)
	})
}

func TestPayloadObfuscationGzipCaesarV1DoesNotAllocate(t *testing.T) {
	tests := []struct {
		name      string
		transform func(string, []byte) error
	}{
		{
			name:      "obfuscate",
			transform: ObfuscatePayload,
		},
		{
			name:      "deobfuscate",
			transform: DeobfuscatePayload,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			body := make([]byte, 1024)
			var transformErr error

			allocations := testing.AllocsPerRun(1000, func() {
				transformErr = test.transform(PayloadObfuscationGzipCaesarV1, body)
			})

			require.NoError(t, transformErr)
			assert.Zero(t, allocations)
		})
	}
}
