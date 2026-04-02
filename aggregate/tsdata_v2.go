package aggregate

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/point"
)

func packetV2PointCount(packet *DataPacketV2) int32 {
	if packet == nil {
		return 0
	}

	if packet.PointCount > 0 {
		return packet.PointCount
	}

	return int32(len(packet.RawPoints))
}

func (packet *DataPacketV2) ToDataPacketMeta() *DataPacket {
	if packet == nil {
		return nil
	}

	return &DataPacket{
		GroupIdHash:            packet.GroupIdHash,
		RawGroupId:             packet.RawGroupId,
		Token:                  packet.Token,
		Source:                 packet.Source,
		DataType:               packet.DataType,
		ConfigVersion:          packet.ConfigVersion,
		HasError:               packet.HasError,
		GroupKey:               packet.GroupKey,
		PointCount:             packetV2PointCount(packet),
		TraceStartTimeUnixNano: packet.TraceStartTimeUnixNano,
		TraceEndTimeUnixNano:   packet.TraceEndTimeUnixNano,
	}
}

func (packet *DataPacketV2) ToDataPacket() (*DataPacket, error) {
	meta := packet.ToDataPacketMeta()
	if meta == nil {
		return nil, nil
	}

	if len(packet.RawPoints) == 0 {
		return meta, nil
	}

	meta.Points = make([]*point.PBPoint, 0, len(packet.RawPoints))
	for idx, raw := range packet.RawPoints {
		if len(raw) == 0 {
			continue
		}

		pb := &point.PBPoint{}
		if err := pb.Unmarshal(raw); err != nil {
			return nil, fmt.Errorf("decode raw_points[%d]: %w", idx, err)
		}
		meta.Points = append(meta.Points, pb)
	}

	if meta.PointCount == 0 {
		meta.PointCount = int32(len(meta.Points))
	}

	return meta, nil
}

func NewDataPacketV2FromDataPacket(packet *DataPacket) (*DataPacketV2, error) {
	if packet == nil {
		return nil, nil
	}

	rawPoints := make([][]byte, 0, len(packet.Points))
	for idx, pb := range packet.Points {
		if pb == nil {
			continue
		}

		raw, err := pb.Marshal()
		if err != nil {
			return nil, fmt.Errorf("encode points[%d]: %w", idx, err)
		}
		rawPoints = append(rawPoints, raw)
	}

	pointCount := packet.PointCount
	if pointCount == 0 {
		pointCount = int32(len(rawPoints))
	}

	return &DataPacketV2{
		GroupIdHash:            packet.GroupIdHash,
		RawGroupId:             packet.RawGroupId,
		Token:                  packet.Token,
		Source:                 packet.Source,
		DataType:               packet.DataType,
		ConfigVersion:          packet.ConfigVersion,
		HasError:               packet.HasError,
		GroupKey:               packet.GroupKey,
		PointCount:             pointCount,
		TraceStartTimeUnixNano: packet.TraceStartTimeUnixNano,
		TraceEndTimeUnixNano:   packet.TraceEndTimeUnixNano,
		RawPoints:              rawPoints,
	}, nil
}
