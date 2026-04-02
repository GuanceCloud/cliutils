package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

func TestDataPacketV2RoundTrip(t *testing.T) {
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

	v1 := &DataPacket{
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
		Points:                 []*point.PBPoint{pt1.PBPoint(), pt2.PBPoint()},
	}

	v2, err := NewDataPacketV2FromDataPacket(v1)
	assert.NoError(t, err)
	assert.NotNil(t, v2)
	assert.Len(t, v2.RawPoints, 2)

	decoded, err := v2.ToDataPacket()
	assert.NoError(t, err)
	assert.NotNil(t, decoded)
	assert.Equal(t, v1.Token, decoded.Token)
	assert.Equal(t, v1.DataType, decoded.DataType)
	assert.Equal(t, v1.RawGroupId, decoded.RawGroupId)
	assert.Equal(t, v1.PointCount, decoded.PointCount)
	assert.Len(t, decoded.Points, 2)

	assert.Equal(t, v1.Points[0].Name, decoded.Points[0].Name)
	assert.Equal(t, v1.Points[1].Name, decoded.Points[1].Name)
}

func TestDataPacketV2MetaUsesRawPointsLen(t *testing.T) {
	v2 := &DataPacketV2{
		Token:     "tkn_meta",
		DataType:  point.SLogging,
		RawPoints: [][]byte{{1}, {2}, {3}},
	}

	meta := v2.ToDataPacketMeta()
	assert.NotNil(t, meta)
	assert.Equal(t, int32(3), meta.PointCount)
	assert.Nil(t, meta.Points)
}

func TestDataPacketV2ProtoCompatibleWithDataPacket(t *testing.T) {
	now := time.Now()
	pt := point.NewPoint("trace", point.NewKVs(map[string]interface{}{
		"trace_id": "trace-compat",
		"span_id":  "span-compat",
	}), point.WithTime(now))

	legacy := &DataPacket{
		GroupIdHash: 1,
		RawGroupId:  "trace-compat",
		Token:       "tkn_compat",
		DataType:    point.STracing,
		GroupKey:    "trace_id",
		PointCount:  1,
		Points:      []*point.PBPoint{pt.PBPoint()},
	}

	body, err := proto.Marshal(legacy)
	assert.NoError(t, err)

	v2 := &DataPacketV2{}
	err = proto.Unmarshal(body, v2)
	assert.NoError(t, err)
	assert.Len(t, v2.RawPoints, 1)
	assert.Equal(t, int32(1), packetV2PointCount(v2))

	decoded, err := v2.ToDataPacket()
	assert.NoError(t, err)
	assert.Len(t, decoded.Points, 1)
	assert.Equal(t, legacy.Points[0].Name, decoded.Points[0].Name)
}
