package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDataPacketWalkRawPBPoints(t *testing.T) {
	now := time.Now()
	payload := point.AppendPointToPBPointsPayload(nil, point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-walk",
		"span_id":  "span-1",
	}), point.WithTime(now)))
	payload = point.AppendPointToPBPointsPayload(payload, point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-walk",
		"span_id":  "span-2",
	}), point.WithTime(now.Add(time.Millisecond))))

	packet := &DataPacket{
		PointCount:    2,
		PointsPayload: payload,
	}

	visited := 0
	err := packet.WalkRawPBPoints(func([]byte) bool {
		visited++
		return true
	})
	require.NoError(t, err)
	assert.Equal(t, 2, visited)
}

func TestDataPacketProtoRoundTrip(t *testing.T) {
	now := time.Now()
	payload := point.AppendPointToPBPointsPayload(nil, point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-compat",
		"span_id":  "span-compat",
	}), point.WithTime(now)))

	original := &DataPacket{
		GroupIdHash:          1,
		RawGroupId:           "trace-compat",
		Token:                "tkn_compat",
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           1,
		PointsPayload:        payload,
		MaxPointTimeUnixNano: now.UnixNano(),
	}

	body, err := proto.Marshal(original)
	require.NoError(t, err)

	decoded := &DataPacket{}
	err = proto.Unmarshal(body, decoded)
	require.NoError(t, err)
	assert.Equal(t, int32(1), packetPointCount(decoded))
	assert.Equal(t, now.UnixNano(), decoded.MaxPointTimeUnixNano)

	dec := point.GetDecoder(point.WithDecEncoding(point.Protobuf))
	defer point.PutDecoder(dec)

	pts, err := dec.Decode(decoded.PointsPayload)
	require.NoError(t, err)
	require.Len(t, pts, 1)
	assert.Equal(t, "trace", pts[0].Name())
}
