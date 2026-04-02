package aggregate

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/point"
)

func packetPointCount(packet *DataPacket) int32 {
	if packet == nil {
		return 0
	}

	if packet.PointCount > 0 {
		return packet.PointCount
	}

	return int32(len(packet.RawPoints))
}

func decodeRawPBPoint(raw []byte) (*point.PBPoint, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	pb := &point.PBPoint{}
	if err := pb.Unmarshal(raw); err != nil {
		return nil, err
	}

	return pb, nil
}

func (packet *DataPacket) DecodePBPoints() ([]*point.PBPoint, error) {
	if packet == nil {
		return nil, nil
	}

	if len(packet.RawPoints) == 0 {
		return nil, nil
	}

	points := make([]*point.PBPoint, 0, len(packet.RawPoints))
	for idx, raw := range packet.RawPoints {
		pb, err := decodeRawPBPoint(raw)
		if err != nil {
			return nil, fmt.Errorf("decode raw_points[%d]: %w", idx, err)
		}
		if pb == nil {
			continue
		}
		points = append(points, pb)
	}

	return points, nil
}

func (packet *DataPacket) DecodePoints() ([]*point.Point, error) {
	pbPoints, err := packet.DecodePBPoints()
	if err != nil {
		return nil, err
	}
	if len(pbPoints) == 0 {
		return nil, nil
	}

	pts := make([]*point.Point, 0, len(pbPoints))
	for idx, pb := range pbPoints {
		if pb == nil {
			continue
		}

		pt := point.FromPB(pb)
		if pt == nil {
			return nil, fmt.Errorf("decode raw_points[%d]: convert to point failed", idx)
		}
		pts = append(pts, pt)
	}

	return pts, nil
}

func (packet *DataPacket) WalkPoints(fn func(*point.Point) bool) error {
	if packet == nil || fn == nil {
		return nil
	}

	for idx, raw := range packet.RawPoints {
		pb, err := decodeRawPBPoint(raw)
		if err != nil {
			return fmt.Errorf("decode raw_points[%d]: %w", idx, err)
		}
		if pb == nil {
			continue
		}

		pt := point.FromPB(pb)
		if pt == nil {
			continue
		}
		if !fn(pt) {
			return nil
		}
	}

	return nil
}
