// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetDataPacketPayloadCompression(t *testing.T) {
	original := []byte(strings.Repeat("tail-sampling-payload-", 1024))
	packet := &DataPacket{PointsPayload: append([]byte(nil), original...)}

	require.NoError(t, SetDataPacketPayloadCompression(packet, PayloadCompressionZstd))
	assert.Equal(t, PayloadCompressionZstd, packet.PayloadCompression)
	assert.Less(t, len(packet.PointsPayload), len(original))

	require.NoError(t, SetDataPacketPayloadCompression(packet, PayloadCompressionNone))
	assert.Equal(t, PayloadCompressionNone, packet.PayloadCompression)
	assert.Equal(t, original, packet.PointsPayload)
}

func TestSetDataPacketPayloadCompressionRejectsUnknownMethods(t *testing.T) {
	packet := &DataPacket{PointsPayload: []byte("payload")}

	err := SetDataPacketPayloadCompression(packet, 99)
	require.ErrorIs(t, err, ErrUnsupportedPayloadCompression)
	assert.Equal(t, PayloadCompressionNone, packet.PayloadCompression)
	assert.Equal(t, []byte("payload"), packet.PointsPayload)

	packet.PayloadCompression = 99
	err = SetDataPacketPayloadCompression(packet, PayloadCompressionNone)
	require.ErrorIs(t, err, ErrUnsupportedPayloadCompression)
}

func TestTailSamplingPayloadProtocolConstants(t *testing.T) {
	assert.Equal(t, "Guance-Tail-Sampling-Payload-Compression", TailSamplingPayloadCompressionHeader)
	assert.Equal(t, "zstd", TailSamplingPayloadCompressionZstd)
}
