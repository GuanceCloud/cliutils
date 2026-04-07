package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGlobalSamplerIngestBranchesAndTailSamplingData(t *testing.T) {
	sampler := NewGlobalSampler(2, time.Second)
	err := sampler.UpdateConfig("token-a", &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL:  time.Hour,
			GroupKey: "trace_id",
			Pipelines: []*SamplingPipeline{
				{Name: "keep_all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
		Logging: &LoggingTailSampling{
			DataTTL: time.Hour,
			GroupDimensions: []*LoggingGroupDimension{
				{GroupKey: "fields.user", Pipelines: []*SamplingPipeline{{Name: "keep_all", Type: PipelineTypeSampling, Rate: 1}}},
			},
		},
		RUM: &RUMTailSampling{
			DataTTL: time.Hour,
			GroupDimensions: []*RUMGroupDimension{
				{GroupKey: "fields.session", Pipelines: []*SamplingPipeline{{Name: "drop_all", Type: PipelineTypeCondition, Condition: `{ status = "drop" }`, Action: PipelineActionDrop}}},
			},
		},
	})
	require.NoError(t, err)

	payload := point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{Name: "event"})
	trace := &DataPacket{
		GroupIdHash:            1,
		RawGroupId:             "trace-1",
		Token:                  "token-a",
		DataType:               point.STracing,
		GroupKey:               "trace_id",
		PointCount:             1,
		TraceStartTimeUnixNano: 10,
		TraceEndTimeUnixNano:   20,
		PointsPayload:          payload,
	}
	sampler.Ingest(nil)
	sampler.Ingest(trace)
	sampler.Ingest(&DataPacket{
		GroupIdHash:            1,
		RawGroupId:             "trace-1-new",
		Token:                  "token-a",
		DataType:               point.STracing,
		GroupKey:               "trace_id",
		PointCount:             2,
		ConfigVersion:          2,
		Source:                 "ddtrace",
		HasError:               true,
		TraceStartTimeUnixNano: 5,
		TraceEndTimeUnixNano:   30,
		MaxPointTimeUnixNano:   40,
		PointsPayload:          payload,
	})

	shard := sampler.shards[1]
	key := tailSamplingGroupMapKey(trace)
	require.NotNil(t, shard.activeMap[key])
	assert.Equal(t, int32(3), shard.activeMap[key].packet.PointCount)
	assert.True(t, shard.activeMap[key].packet.HasError)
	assert.Equal(t, int64(5), shard.activeMap[key].packet.TraceStartTimeUnixNano)
	assert.Equal(t, int64(30), shard.activeMap[key].packet.TraceEndTimeUnixNano)
	assert.Equal(t, int64(40), shard.activeMap[key].packet.MaxPointTimeUnixNano)
	assert.Equal(t, int64(2), shard.activeMap[key].packet.ConfigVersion)
	assert.Equal(t, "ddtrace", shard.activeMap[key].packet.Source)

	logging := &DataPacket{
		GroupIdHash:   2,
		Token:         "token-a",
		DataType:      point.SLogging,
		GroupKey:      "fields.user",
		PointsPayload: payload,
		PointCount:    1,
	}
	rum := &DataPacket{
		GroupIdHash: 3,
		Token:       "token-a",
		DataType:    point.SRUM,
		GroupKey:    "fields.session",
		PointsPayload: point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{
			Name: "rum",
			Fields: []*point.Field{
				{Key: "status", Val: &point.Field_S{S: "drop"}},
			},
		}),
		PointCount: 1,
	}
	sampler.Ingest(logging)
	sampler.Ingest(rum)
	sampler.Ingest(&DataPacket{GroupIdHash: 4, Token: "token-a", DataType: "unknown"})
	sampler.Ingest(&DataPacket{GroupIdHash: 5, Token: "missing", DataType: point.STracing})
	assert.Zero(t, tailSamplingGroupMapKey(nil))

	dataGroups := map[uint64]*DataGroup{
		tailSamplingGroupMapKey(logging): {dataType: point.SLogging, packet: logging},
		tailSamplingGroupMapKey(rum):     {dataType: point.SRUM, packet: rum},
		99:                               {dataType: "unknown", packet: &DataPacket{Token: "token-a", DataType: "unknown"}},
		100:                              nil,
	}
	kept := sampler.TailSamplingData(dataGroups)
	require.Len(t, kept, 1)
	assert.Same(t, logging, kept[tailSamplingGroupMapKey(logging)])

	assert.Nil(t, sampler.GetRUMConfig("missing"))
}

func TestGlobalSamplerUpdateConfigNoopAndZeroTTL(t *testing.T) {
	sampler := NewGlobalSampler(1, time.Second)
	assert.NoError(t, sampler.UpdateConfig("token-a", nil))
	assert.Nil(t, sampler.GetTraceConfig("token-a"))

	err := sampler.UpdateConfig("token-a", &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{DataTTL: time.Second, GroupKey: "trace_id"},
	})
	require.NoError(t, err)
	first := sampler.GetTraceConfig("token-a")
	err = sampler.UpdateConfig("token-a", &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{DataTTL: 2 * time.Second, GroupKey: "trace_id"},
	})
	require.NoError(t, err)
	assert.Same(t, first, sampler.GetTraceConfig("token-a"), "same version should keep existing config")

	sampler.configMap["zero"] = &TailSamplingConfigs{Tracing: &TraceTailSampling{}}
	sampler.Ingest(&DataPacket{GroupIdHash: 1, Token: "zero", DataType: point.STracing})
	assert.Empty(t, sampler.shards[0].activeMap)
}
