package aggregate

import (
	"github.com/GuanceCloud/cliutils/point"
)

// PickTrace 按找trace_id 聚合
// 返回值 key：hashed trace_id value：TraceData
func PickTrace(source string, pts []*point.Point) map[uint64]*TraceDataPacket {
	traceDatas := make(map[uint64]*TraceDataPacket)
	for _, pt := range pts {
		v := pt.Get("trace_id")
		if tid, ok := v.(string); ok {
			id := hashTraceID(tid)
			traceData, ok := traceDatas[id]
			if !ok {
				traceData = &TraceDataPacket{
					TraceIdHash:   id,
					RawTraceId:    tid,
					Token:         "",
					Source:        source,
					ConfigVersion: 0,
					Spans:         []*point.PBPoint{},
				}
			}
			traceData.Spans = append(traceData.Spans, pt.PBPoint())

			status := pt.GetTag("status")
			if status == "error" {
				traceData.HasError = true
			}
		}
	}

	return traceDatas
}

//TraceIDHash: id,
//RawTraceID:  tid,
//Spans:       []*point.Point{},
//Source:      source,
