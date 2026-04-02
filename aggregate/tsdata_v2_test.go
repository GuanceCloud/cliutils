package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestDataPacketDecodePBPoints(t *testing.T) {
	now := time.Now()
	pt1 := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-1",
		"span_id":  "span-1",
		"resource": "GET /v1/ping",
	}), point.WithTime(now))
	pt2 := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-1",
		"span_id":  "span-2",
		"resource": "SELECT 1",
	}), point.WithTime(now.Add(time.Millisecond)))

	raw1, err := pt1.PBPoint().Marshal()
	assert.NoError(t, err)
	raw2, err := pt2.PBPoint().Marshal()
	assert.NoError(t, err)

	packet := &DataPacket{
		GroupIdHash:            11,
		RawGroupId:             "trace-1",
		Token:                  "tkn_123",
		Source:                 "ddtrace",
		DataType:               point.STracing,
		ConfigVersion:          2,
		GroupKey:               "trace_id",
		PointCount:             2,
		TraceStartTimeUnixNano: now.UnixNano(),
		TraceEndTimeUnixNano:   now.Add(time.Millisecond).UnixNano(),
		RawPoints:              [][]byte{raw1, raw2},
	}

	pbPoints, err := packet.DecodePBPoints()
	assert.NoError(t, err)
	assert.Len(t, pbPoints, 2)
	assert.Equal(t, "trace", pbPoints[0].Name)
	assert.Equal(t, "trace", pbPoints[1].Name)
}

func TestDataPacketDecodePoints(t *testing.T) {
	now := time.Now()
	pt := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-2",
		"span_id":  "span-1",
	}), point.WithTime(now))

	raw, err := pt.PBPoint().Marshal()
	assert.NoError(t, err)

	packet := &DataPacket{
		Token:     "tkn_decode",
		DataType:  point.STracing,
		RawPoints: [][]byte{raw},
	}

	decoded, err := packet.DecodePoints()
	assert.NoError(t, err)
	assert.Len(t, decoded, 1)
	assert.Equal(t, "trace", decoded[0].Name())
}

func TestDataPacketWalkPoints(t *testing.T) {
	now := time.Now()
	pt1 := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-walk",
		"span_id":  "span-1",
	}), point.WithTime(now))
	pt2 := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-walk",
		"span_id":  "span-2",
	}), point.WithTime(now.Add(time.Millisecond)))

	raw1, err := pt1.PBPoint().Marshal()
	assert.NoError(t, err)
	raw2, err := pt2.PBPoint().Marshal()
	assert.NoError(t, err)

	packet := &DataPacket{
		RawPoints: [][]byte{raw1, raw2},
	}

	visited := 0
	err = packet.WalkPoints(func(*point.Point) bool {
		visited++
		return visited < 2
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, visited)
}

func TestDataPacketProtoRoundTrip(t *testing.T) {
	now := time.Now()
	pt := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-compat",
		"span_id":  "span-compat",
	}), point.WithTime(now))

	raw, err := pt.PBPoint().Marshal()
	assert.NoError(t, err)

	original := &DataPacket{
		GroupIdHash: 1,
		RawGroupId:  "trace-compat",
		Token:       "tkn_compat",
		DataType:    point.STracing,
		GroupKey:    "trace_id",
		PointCount:  1,
		RawPoints:   [][]byte{raw},
	}

	body, err := proto.Marshal(original)
	assert.NoError(t, err)

	decoded := &DataPacket{}
	err = proto.Unmarshal(body, decoded)
	assert.NoError(t, err)
	assert.Len(t, decoded.RawPoints, 1)
	assert.Equal(t, int32(1), packetPointCount(decoded))
}
