package aggregate

import (
	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestPickTrace(t *testing.T) {
	now := time.Now()
	pt1 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "/resource",
		"trace_id":                    "1000000000",
		"span_id":                     "123456789",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt1.SetTime(now)

	pt2 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "/client",
		"trace_id":                    "2000000000",
		"span_id":                     "123456789",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt2.SetTime(now)

	packages := PickTrace("ddtrace", []*point.Point{pt1, pt2}, 1)
	t.Logf("package len=%d", len(packages))
	assert.Len(t, packages, 2)
	for _, p := range packages {
		t.Logf("package=%s", p.RawTraceId)
	}
}

func TestSamplingPipeline_DoAction1(t *testing.T) {
	pip := &SamplingPipeline{
		Name: "keep resource",
		Type: PipelineTypeCondition,
		//Condition: "{ resource EQ \"/resource\" }",
		Condition: `{ resource = "/resource" }`,
		Action:    PipelineActionKeep,
	}
	err := pip.Apply()
	assert.NoError(t, err)
	now := time.Now()
	pt1 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "/resource",
		"trace_id":                    "1000000000",
		"span_id":                     "123456789",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt1.SetTime(now)
	packages := PickTrace("ddtrace", []*point.Point{pt1}, 1)
	for _, packet := range packages {
		isKeep, td := pip.DoAction(packet)
		assert.True(t, isKeep)
		assert.Len(t, td.Spans, 1)
	}
}

func TestSamplingPipeline_DoAction(t *testing.T) {
	type fields struct {
		Name      string
		Type      PipelineType
		Condition string
		Action    PipelineAction
		Rate      float64
		HashKeys  []string
	}
	type args struct {
		td *TraceDataPacket
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		tdIsNil bool
	}{
		{
			name: "test_resource",
			fields: fields{
				Name: "keep resource",
				Type: PipelineTypeCondition,
				//Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ resource = "/resource" }`,
				Action:    PipelineActionKeep,
			},
			args: args{
				td: &TraceDataPacket{
					TraceIdHash:   123,
					RawTraceId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					Spans:         MockTrace(),
				},
			},
			want:    true,
			tdIsNil: false,
		},
		{
			name: "test_client",
			fields: fields{
				Name: "drop client",
				Type: PipelineTypeCondition,
				//Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ resource = "/client" }`,
				Action:    PipelineActionDrop,
			},
			args: args{
				td: &TraceDataPacket{
					TraceIdHash:   123,
					RawTraceId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					Spans:         MockTrace(),
				},
			},
			want:    true,
			tdIsNil: true,
		},
		{
			name: "test_haskey",
			fields: fields{
				Name: "keep resource",
				Type: PipelineTypeCondition,
				// Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ 1 = 1 }`,
				Action:    PipelineActionKeep,
				HashKeys:  []string{"db_host"},
			},
			args: args{
				td: &TraceDataPacket{
					TraceIdHash:   123,
					RawTraceId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					Spans:         MockTrace(),
				},
			},
			want:    true,
			tdIsNil: false,
		},
		{
			name: "test_rate",
			fields: fields{
				Name: "keep resource",
				Type: PipelineTypeSampling,
				//Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ 1 = 1 }`,
				Rate:      0.01,
			},
			args: args{
				td: &TraceDataPacket{
					TraceIdHash:   123123123123123,
					RawTraceId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					Spans:         MockTrace(),
				},
			},
			want:    true,
			tdIsNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sp := &SamplingPipeline{
				Name:      tt.fields.Name,
				Type:      tt.fields.Type,
				Condition: tt.fields.Condition,
				Action:    tt.fields.Action,
				Rate:      tt.fields.Rate,
				HashKeys:  tt.fields.HashKeys,
			}
			err := sp.Apply()
			assert.NoError(t, err)
			got, got1 := sp.DoAction(tt.args.td)
			assert.Equalf(t, tt.want, got, "DoAction(%v)", tt.args.td)
			if !(tt.tdIsNil == (got1 == nil)) {
				t.Errorf("DoAction() got1 = %v, want %v", got1, tt.tdIsNil)
			}
		})
	}
}

func MockTrace() []*point.PBPoint {
	var pbs []*point.PBPoint
	now := time.Now()
	pt1 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "/resource",
		"trace_id":                    "1000000000",
		"span_id":                     "123456789",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt1.SetTime(now)

	pt2 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "/client",
		"trace_id":                    "1000000000",
		"span_id":                     "1234567892",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt2.SetTime(now)

	pt3 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "select",
		"trace_id":                    "1000000000",
		"span_id":                     "1234567891",
		"db_host":                     "mysql",
		"status":                      "ok",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt3.SetTime(now)

	pt4 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "mysql",
		"trace_id":                    "1000000000",
		"span_id":                     "1234567891",
		"error_message":               "error",
		"status":                      "error",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt4.SetTime(now)

	for _, p := range []*point.Point{pt1, pt2, pt3, pt4} {
		pbs = append(pbs, p.PBPoint())
	}

	return pbs
}
