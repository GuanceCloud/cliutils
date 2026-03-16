package aggregate

import (
	"hash/fnv"
	"math"
	"strconv"
	"time"

	fp "github.com/GuanceCloud/cliutils/filter"
	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/point"
)

const (
	PipelineTypeCondition = "condition"
	PipelineTypeSampling  = "probabilistic"

	PipelineActionKeep = "keep"
	PipelineActionDrop = "drop"
	sampleRange        = 10000
)

type (
	PipelineType   string
	PipelineAction string
)

type DerivedMetric struct {
	Name      string           `toml:"name" json:"name"`
	Algorithm *AggregationAlgo `toml:"aggregate" json:"aggregate"`
	Condition string           `toml:"condition" json:"condition"`
	Groupby   []string         `toml:"group_by" json:"group_by"`
}

type BuiltinDerivedMetricConfig struct {
	Name    string `toml:"name" json:"name"`
	Enabled bool   `toml:"enabled" json:"enabled"`
}

type SamplingPipeline struct {
	Name      string         `toml:"name" json:"name"`
	Type      PipelineType   `toml:"type" json:"type"`
	Condition string         `toml:"condition,omitempty" json:"condition,omitempty"`
	Action    PipelineAction `toml:"action,omitempty" json:"action,omitempty"`
	Rate      float64        `toml:"rate,omitempty" json:"rate,omitempty"`
	HashKeys  []string       `toml:"hash_keys" json:"hash_keys"`

	conds fp.WhereConditions
}

func (sp *SamplingPipeline) Apply() error {
	if ast, err := fp.GetConds(sp.Condition); err != nil {
		return err
	} else {
		sp.conds = ast
		return nil
	}
}

func (sp *SamplingPipeline) DoAction(td *DataPacket) (bool, *DataPacket) {
	if sp.conds == nil { // condition are required to do actions.
		return false, td
	}
	ptw := &ptWrap{}
	if len(sp.HashKeys) > 0 {
		for _, key := range sp.HashKeys {
			for _, span := range td.Points {
				ptw.Point = point.FromPB(span)
				if _, has := ptw.Get(key); has {
					l.Debugf("matched 'hasKey' has key =%s", key)
					return true, td
				}
			}
		}
	}

	matched := false

	for _, span := range td.Points {
		ptw.Point = point.FromPB(span)
		if x := sp.conds.Eval(ptw); x < 0 {
			continue
		} // else: matched, fall through...
		l.Debugf("matched condition =%s", sp.Condition)
		matched = true

		if sp.Type == PipelineTypeSampling {
			if sp.Rate > 0.0 {
				if td.GroupIdHash%sampleRange < uint64(math.Floor(sp.Rate*float64(sampleRange))) {
					return true, td // keep
				}
				// sp.dropped++
				return true, nil
			}
		}

		if sp.Type == PipelineTypeCondition {
			// check on action
			switch sp.Action {
			case PipelineActionDrop:
				// r.dropped++
				return true, nil
			case PipelineActionKeep:
				return true, td
			}
		}
	}

	return matched, td
}

func PickTrace(source string, pts []*point.Point, version int64) map[uint64]*DataPacket {
	traceDatas := make(map[uint64]*DataPacket)
	for _, pt := range pts {
		v := pt.Get("trace_id")
		if tid, ok := v.(string); ok {
			id := hashTraceID(tid)
			traceData, ok := traceDatas[id]
			if !ok {
				traceData = &DataPacket{
					GroupIdHash:   id,
					RawGroupId:    tid,
					Token:         "", // 在pick调用处添加。
					DataType:      point.Tracing.String(),
					Source:        source,
					ConfigVersion: version,
					Points:        []*point.PBPoint{},
				}
				traceDatas[id] = traceData
			}
			traceData.Points = append(traceData.Points, pt.PBPoint())

			status := pt.GetTag("status")
			if status == "error" {
				traceData.HasError = true
			}
			traceData.PointCount++
			start, duration := getTime(pt)
			if traceData.TraceStartTimeUnixNano == 0 {
				traceData.TraceStartTimeUnixNano = start
			}
			if traceData.TraceStartTimeUnixNano > start {
				traceData.TraceStartTimeUnixNano = start
			}
			if traceData.TraceEndTimeUnixNano == 0 {
				traceData.TraceEndTimeUnixNano = start + duration
			}
			if traceData.TraceEndTimeUnixNano < start+duration {
				traceData.TraceEndTimeUnixNano = start + duration
			}
		} else {
			l.Errorf("invalid trace_id:%v", v)
		}
	}

	return traceDatas
}

type TraceTailSampling struct {
	DataTTL               time.Duration                 `toml:"data_ttl" json:"data_ttl"`
	BuiltinDerivedMetrics []*BuiltinDerivedMetricConfig `toml:"builtin_derived_metrics" json:"builtin_derived_metrics"`
	DerivedMetrics        []*DerivedMetric              `toml:"derived_metrics" json:"derived_metrics"`
	Pipelines             []*SamplingPipeline           `toml:"sampling_pipeline" json:"pipelines"`
	Version               int64                         `toml:"version" json:"version"`

	// 链路特有配置
	GroupKey string `toml:"group_key" json:"group_key"` // 链路固定为 "trace_id"
}

type TailSamplingConfigs struct {
	Tracing *TraceTailSampling   `toml:"trace" json:"trace"`
	Logging *LoggingTailSampling `toml:"logging" json:"logging"`
	RUM     *RUMTailSampling     `toml:"rum" json:"rum"`
	Version int64                `toml:"version" json:"version"`
}

func (t *TailSamplingConfigs) Init() {
	if t.Tracing != nil {
		if t.Tracing.DataTTL == 0 {
			t.Tracing.DataTTL = 5 * time.Minute
		}
		if t.Tracing.GroupKey == "" {
			t.Tracing.GroupKey = "trace_id"
		}
		resolveBuiltinDerivedMetrics(point.STracing, t.Tracing.BuiltinDerivedMetrics)
		for _, pipeline := range t.Tracing.Pipelines {
			if err := pipeline.Apply(); err != nil {
				l.Errorf("failed to apply sampling pipeline: %s", err)
			}
		}
	}

	if t.Logging != nil {
		if t.Logging.DataTTL == 0 {
			t.Logging.DataTTL = 1 * time.Minute
		}
		for _, group := range t.Logging.GroupDimensions {
			resolveBuiltinDerivedMetrics(point.SLogging, group.BuiltinDerivedMetrics)
			for _, pipeline := range group.Pipelines {
				if err := pipeline.Apply(); err != nil {
					l.Errorf("failed to apply sampling pipeline: %s", err)
				}
			}
		}
	}

	if t.RUM != nil {
		if t.RUM.DataTTL == 0 {
			t.RUM.DataTTL = 1 * time.Minute
		}
		for _, group := range t.RUM.GroupDimensions {
			resolveBuiltinDerivedMetrics(point.SRUM, group.BuiltinDerivedMetrics)
			for _, pipeline := range group.Pipelines {
				if err := pipeline.Apply(); err != nil {
					l.Errorf("failed to apply sampling pipeline: %s", err)
				}
			}
		}
	}
}

type LoggingTailSampling struct {
	DataTTL time.Duration `toml:"data_ttl" json:"data_ttl"`
	Version int64         `toml:"version" json:"version"`

	// 按分组维度配置（不再是全局管道）
	GroupDimensions []*LoggingGroupDimension `toml:"group_dimensions" json:"group_dimensions"`
}

type LoggingGroupDimension struct {
	// 分组键（如 user_id, order_id, session_id）
	GroupKey string `toml:"group_key" json:"group_key"`

	// 该分组维度下的采样管道
	Pipelines []*SamplingPipeline `toml:"pipelines" json:"pipelines"`

	// 显式启用的内置派生指标
	BuiltinDerivedMetrics []*BuiltinDerivedMetricConfig `toml:"builtin_derived_metrics" json:"builtin_derived_metrics"`

	// 该分组特有的派生指标
	DerivedMetrics []*DerivedMetric `toml:"derived_metrics" json:"derived_metrics"`
}

func (logGroup *LoggingGroupDimension) PickLogging(source string, pts []*point.Point) (map[uint64]*DataPacket, []*point.Point) {
	return pickByGroupKey(logGroup.GroupKey, source, pts, point.Logging)
}

func pickByGroupKey(groupKey string, source string, pts []*point.Point, category point.Category) (map[uint64]*DataPacket, []*point.Point) {
	traceDatas := make(map[uint64]*DataPacket)
	passedThrough := make([]*point.Point, 0)
	for _, pt := range pts {
		v := pt.Get(groupKey) // string float int64...
		if v == nil {
			passedThrough = append(passedThrough, pt)
			continue
		}

		tid := fieldToString(v)
		if tid == "" {
			passedThrough = append(passedThrough, pt)
			continue
		}
		l.Debugf("group key=%s  tid=%s", groupKey, tid)
		id := hashTraceID(tid)
		traceData, ok := traceDatas[id]
		if !ok {
			traceData = &DataPacket{
				GroupIdHash: id,
				RawGroupId:  tid,
				Token:       "",
				Source:      source,
				DataType:    category.String(),
				//	ConfigVersion: version,
				Points: []*point.PBPoint{},
			}
			traceDatas[id] = traceData
		}
		traceData.Points = append(traceData.Points, pt.PBPoint())

		status := pt.GetTag("status")
		if status == "error" {
			traceData.HasError = true
		}
	}

	return traceDatas, passedThrough
}

// RUM尾采样配置.
type RUMTailSampling struct {
	DataTTL time.Duration `toml:"data_ttl" json:"data_ttl"`
	Version int64         `toml:"version" json:"version"`

	// RUM可能也有多个分组维度
	GroupDimensions []*RUMGroupDimension `toml:"group_dimensions" json:"group_dimensions"`
}

type RUMGroupDimension struct {
	GroupKey              string                        `toml:"group_key" json:"group_key"` // session_id, user_id, page_id
	Pipelines             []*SamplingPipeline           `toml:"pipelines" json:"pipelines"`
	BuiltinDerivedMetrics []*BuiltinDerivedMetricConfig `toml:"builtin_derived_metrics" json:"builtin_derived_metrics"`
	DerivedMetrics        []*DerivedMetric              `toml:"derived_metrics" json:"derived_metrics"`
}

func (rumGroup *RUMGroupDimension) PickRUM(source string, pts []*point.Point) (map[uint64]*DataPacket, []*point.Point) {
	return pickByGroupKey(rumGroup.GroupKey, source, pts, point.RUM)
}

func SetLogging(log *logger.Logger) {
	l = log
}

// hashTraceID 将字符串 TraceID 转换为 uint64.
func hashTraceID(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

func fieldToString(field any) string {
	switch x := field.(type) {
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case string:
		return x
	case []byte:
		return string(x)
	case bool:
		return strconv.FormatBool(x)
	default: // other types are ignored
		return ""
	}
}
