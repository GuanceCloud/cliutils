package aggregate

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
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
		t.Logf("package=%s", p.RawGroupId)
	}
}

func TestSamplingPipeline_DoAction1(t *testing.T) {
	pip := &SamplingPipeline{
		Name: "keep resource",
		Type: PipelineTypeCondition,
		// Condition: "{ resource EQ \"/resource\" }",
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
		assert.Equal(t, int32(1), packetPointCount(td))
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
		td *DataPacket
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
				// Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ resource = "/resource" }`,
				Action:    PipelineActionKeep,
			},
			args: args{
				td: &DataPacket{
					GroupIdHash:   123,
					RawGroupId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					RawPoints:     MockTrace(),
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
				// Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ resource = "/client" }`,
				Action:    PipelineActionDrop,
			},
			args: args{
				td: &DataPacket{
					GroupIdHash:   123,
					RawGroupId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					RawPoints:     MockTrace(),
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
				td: &DataPacket{
					GroupIdHash:   123,
					RawGroupId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					RawPoints:     MockTrace(),
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
				// Condition: "{ resource EQ \"/resource\" }",
				Condition: `{ 1 = 1 }`,
				Rate:      0.01,
			},
			args: args{
				td: &DataPacket{
					GroupIdHash:   123123123123123,
					RawGroupId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					RawPoints:     MockTrace(),
				},
			},
			want:    true,
			tdIsNil: true,
		},
		{
			name: "test_drop_resource",
			fields: fields{
				Name:      "drop resource",
				Type:      PipelineTypeCondition,
				Condition: "{ resource = \"GET /tmall/**\" }",
				Action:    PipelineActionDrop,
			},
			args: args{
				td: &DataPacket{
					GroupIdHash:   123123123123123,
					RawGroupId:    "123456789",
					Source:        "ddtrace",
					ConfigVersion: 1,
					HasError:      false,
					RawPoints:     MockTrace(),
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

func MockTrace() [][]byte {
	var raws [][]byte
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

	pt5 := point.NewPoint("ddtrace", point.NewKVs(map[string]interface{}{
		"http.server.requests_bucket": float64(10),
		"resource":                    "GET /tmall/**",
		"trace_id":                    "1000000000",
		"span_id":                     "12345678912",
		"status":                      "ok",
		"start_time":                  time.Now().Unix(),
		"duration":                    1000,
	}), point.CommonLoggingOptions()...)
	pt5.SetTime(now)

	for _, p := range []*point.Point{pt1, pt2, pt3, pt4, pt5} {
		raw, err := p.PBPoint().Marshal()
		if err != nil {
			continue
		}
		raws = append(raws, raw)
	}

	return raws
}

// TestTailSamplingConfigs_Init 测试配置初始化
func TestTailSamplingConfigs_Init(t *testing.T) {
	tests := []struct {
		name    string
		config  *TailSamplingConfigs
		wantErr bool
	}{
		{
			name: "valid tracing config",
			config: &TailSamplingConfigs{
				Version: 1,
				Tracing: &TraceTailSampling{
					DataTTL:  5 * time.Minute,
					Version:  1,
					GroupKey: "trace_id",
					Pipelines: []*SamplingPipeline{
						{
							Name:      "keep_errors",
							Type:      PipelineTypeCondition,
							Condition: `{ error = true }`,
							Action:    PipelineActionKeep,
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid logging config with multiple dimensions",
			config: &TailSamplingConfigs{
				Version: 1,
				Logging: &LoggingTailSampling{
					DataTTL: 2 * time.Minute,
					Version: 1,
					GroupDimensions: []*LoggingGroupDimension{
						{
							GroupKey: "user_id",
							Pipelines: []*SamplingPipeline{
								{
									Name:      "sample_user_logs",
									Type:      PipelineTypeSampling,
									Rate:      0.1,
									Condition: `{ source = "user_action" }`,
								},
							},
						},
						{
							GroupKey: "order_id",
							Pipelines: []*SamplingPipeline{
								{
									Name:      "keep_order_errors",
									Type:      PipelineTypeCondition,
									Condition: `{ level = "error" AND order_id != "" }`,
									Action:    PipelineActionKeep,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with invalid pipeline condition",
			config: &TailSamplingConfigs{
				Version: 1,
				Tracing: &TraceTailSampling{
					DataTTL:  5 * time.Minute,
					Version:  1,
					GroupKey: "trace_id",
					Pipelines: []*SamplingPipeline{
						{
							Name:      "invalid_pipeline",
							Type:      PipelineTypeCondition,
							Condition: `{ invalid syntax }`, // 无效的条件语法
							Action:    PipelineActionKeep,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty config",
			config: &TailSamplingConfigs{
				Version: 1,
			},
			wantErr: false,
		},
		{
			name: "config with disabled logging dimension",
			config: &TailSamplingConfigs{
				Version: 1,
				Logging: &LoggingTailSampling{
					DataTTL: 2 * time.Minute,
					Version: 1,
					GroupDimensions: []*LoggingGroupDimension{
						{
							GroupKey: "user_id",
							Pipelines: []*SamplingPipeline{
								{
									Name: "sample_user_logs",
									Type: PipelineTypeSampling,
									Rate: 0.1,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "config with missing logging group key",
			config: &TailSamplingConfigs{
				Version: 1,
				Logging: &LoggingTailSampling{
					GroupDimensions: []*LoggingGroupDimension{
						{
							Pipelines: []*SamplingPipeline{
								{
									Name:      "keep_errors",
									Type:      PipelineTypeCondition,
									Condition: `{ level = "error" }`,
									Action:    PipelineActionKeep,
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "config with unsupported derived metrics",
			config: &TailSamplingConfigs{
				Version: 1,
				Tracing: &TraceTailSampling{
					DerivedMetrics: []*DerivedMetric{
						{
							Name: "trace_total_count",
							Type: SUM,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "config with mixed data types",
			config: &TailSamplingConfigs{
				Version: 1,
				Tracing: &TraceTailSampling{
					DataTTL:  5 * time.Minute,
					Version:  1,
					GroupKey: "trace_id",
					Pipelines: []*SamplingPipeline{
						{
							Name:      "keep_slow_traces",
							Type:      PipelineTypeCondition,
							Condition: `{ duration > 5000 }`,
							Action:    PipelineActionKeep,
						},
					},
				},
				Logging: &LoggingTailSampling{
					DataTTL: 2 * time.Minute,
					Version: 1,
					GroupDimensions: []*LoggingGroupDimension{
						{
							GroupKey: "session_id",
							Pipelines: []*SamplingPipeline{
								{
									Name: "sample_session_logs",
									Type: PipelineTypeSampling,
									Rate: 0.05,
								},
							},
						},
					},
				},
				RUM: &RUMTailSampling{
					DataTTL: 10 * time.Minute,
					Version: 1,
					GroupDimensions: []*RUMGroupDimension{
						{
							GroupKey: "session_id",
							Pipelines: []*SamplingPipeline{
								{
									Name:      "keep_slow_sessions",
									Type:      PipelineTypeCondition,
									Condition: `{ page_load_time > 3000 }`,
									Action:    PipelineActionKeep,
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 初始化日志以便测试
			SetLogging(logger.DefaultSLogger("test"))

			// 调用 Init 方法
			err := tt.config.Init()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// 验证管道是否已应用
			if tt.config.Tracing != nil {
				for _, pipeline := range tt.config.Tracing.Pipelines {
					if pipeline.Condition != "" && (!tt.wantErr || pipeline.Condition != `{ invalid syntax }`) {
						// 对于有效条件，验证 conds 是否已设置
						assert.NotNil(t, pipeline.conds, "Pipeline conditions should be parsed")
					}
				}
			}

			if tt.config.Logging != nil {
				for _, dimension := range tt.config.Logging.GroupDimensions {
					for _, pipeline := range dimension.Pipelines {
						if pipeline.Condition != "" {
							assert.NotNil(t, pipeline.conds, "Logging pipeline conditions should be parsed")
						}
					}
				}
			}

			if tt.config.RUM != nil {
				for _, dimension := range tt.config.RUM.GroupDimensions {
					for _, pipeline := range dimension.Pipelines {
						if pipeline.Condition != "" {
							assert.NotNil(t, pipeline.conds, "RUM pipeline conditions should be parsed")
						}
					}
				}
			}
		})
	}
}

// TestPickLogging 测试日志数据分组
func TestPickLogging(t *testing.T) {
	now := time.Now()

	// 创建测试数据点
	createLogPoint := func(name string, fields map[string]interface{}) *point.Point {
		pt := point.NewPoint("logging", point.NewKVs(fields), point.CommonLoggingOptions()...)
		pt.SetTime(now)
		return pt
	}

	tests := []struct {
		name           string
		dimension      *LoggingGroupDimension
		points         []*point.Point
		wantGroupCount int      // 期望的分组数量
		wantPassCount  int      // 期望的直接通过数量
		groupKeys      []string // 期望的分组键值
	}{
		{
			name: "group by user_id - all have key",
			dimension: &LoggingGroupDimension{
				GroupKey: "user_id",
			},
			points: []*point.Point{
				createLogPoint("user_action", map[string]interface{}{
					"message": "User logged in",
					"user_id": "user_123",
					"level":   "info",
					"status":  "ok",
				}),
				createLogPoint("user_action", map[string]interface{}{
					"message": "User viewed profile",
					"user_id": "user_123", // 同一个用户
					"level":   "info",
					"status":  "ok",
				}),
				createLogPoint("user_action", map[string]interface{}{
					"message": "Another user action",
					"user_id": "user_456", // 不同用户
					"level":   "info",
					"status":  "ok",
				}),
			},
			wantGroupCount: 2, // user_123 和 user_456 两个分组
			wantPassCount:  0, // 所有点都有 user_id，没有直通数据
			groupKeys:      []string{"user_123", "user_456"},
		},
		{
			name: "group by user_id - some missing key",
			dimension: &LoggingGroupDimension{
				GroupKey: "user_id",
			},
			points: []*point.Point{
				createLogPoint("system", map[string]interface{}{
					"message": "System started",
					"level":   "info",
					"status":  "ok",
					// 没有 user_id
				}),
				createLogPoint("user_action", map[string]interface{}{
					"message": "User logged in",
					"user_id": "user_123",
					"level":   "info",
					"status":  "ok",
				}),
				createLogPoint("system", map[string]interface{}{
					"message": "System error",
					"level":   "error",
					"status":  "error",
					// 没有 user_id
				}),
			},
			wantGroupCount: 1, // 只有 user_123 一个分组
			wantPassCount:  2, // 两个没有 user_id 的点直接通过
			groupKeys:      []string{"user_123"},
		},
		{
			name: "group by order_id - different value types",
			dimension: &LoggingGroupDimension{
				GroupKey: "order_id",
			},
			points: []*point.Point{
				createLogPoint("order", map[string]interface{}{
					"message":  "Order created",
					"order_id": "ORD-12345", // 字符串
					"level":    "info",
				}),
				createLogPoint("order", map[string]interface{}{
					"message":  "Order updated",
					"order_id": int64(12345), // int64
					"level":    "info",
				}),
				createLogPoint("order", map[string]interface{}{
					"message":  "Order completed",
					"order_id": float64(12345.0), // float64
					"level":    "info",
				}),
				createLogPoint("order", map[string]interface{}{
					"message":  "Invalid order",
					"order_id": true, // 不支持的类型
					"level":    "error",
				}),
			},
			wantGroupCount: 3, // 三个有效分组（字符串、int64、float64 都转换为 "12345"）
			wantPassCount:  0, // 一个不支持类型的点直接通过
			groupKeys:      []string{"ORD-12345", "12345", "true"},
		},
		{
			name: "group by session_id - empty values",
			dimension: &LoggingGroupDimension{
				GroupKey: "session_id",
			},
			points: []*point.Point{
				createLogPoint("session", map[string]interface{}{
					"message":    "Session started",
					"session_id": "sess_abc",
					"level":      "info",
				}),
				createLogPoint("session", map[string]interface{}{
					"message":    "Session activity",
					"session_id": "", // 空字符串
					"level":      "info",
				}),
				createLogPoint("session", map[string]interface{}{
					"message": "No session id",
					"level":   "info",
					// 没有 session_id
				}),
			},
			wantGroupCount: 1, // 只有 sess_abc 一个分组
			wantPassCount:  2, // 空字符串和缺失键的点都直接通过
			groupKeys:      []string{"sess_abc"},
		},
		{
			name: "group with error status detection",
			dimension: &LoggingGroupDimension{
				GroupKey: "user_id",
			},
			points: []*point.Point{
				createLogPoint("error", map[string]interface{}{
					"message": "Login failed",
					"user_id": "user_123",
					"level":   "error",
					"status":  "error", // 有错误状态
				}),
				createLogPoint("info", map[string]interface{}{
					"message": "Login successful",
					"user_id": "user_123",
					"level":   "info",
					"status":  "ok",
				}),
				createLogPoint("error", map[string]interface{}{
					"message": "Payment failed",
					"user_id": "user_456",
					"level":   "error",
					// 没有 status 标签，但有 level=error
				}),
			},
			wantGroupCount: 2,
			wantPassCount:  0,
			groupKeys:      []string{"user_123", "user_456"},
		},
		{
			name: "disabled dimension",
			dimension: &LoggingGroupDimension{
				GroupKey: "user_id",
			},
			points: []*point.Point{
				createLogPoint("test", map[string]interface{}{
					"message": "Test message",
					"user_id": "user_123",
					"level":   "info",
				}),
			},
			wantGroupCount: 1,
			wantPassCount:  0,
			groupKeys:      []string{},
		},
		{
			name: "large number of points",
			dimension: &LoggingGroupDimension{
				GroupKey: "request_id",
			},
			points: func() []*point.Point {
				var points []*point.Point
				for i := 0; i < 100; i++ {
					points = append(points, createLogPoint("request", map[string]interface{}{
						"message":    "Request processed",
						"request_id": fmt.Sprintf("req_%d", i%10), // 10个不同的请求ID
						"level":      "info",
					}))
				}
				return points
			}(),
			wantGroupCount: 10, // 10个不同的请求ID
			wantPassCount:  0,
			groupKeys:      []string{"req_0", "req_1", "req_2", "req_3", "req_4", "req_5", "req_6", "req_7", "req_8", "req_9"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 调用 PickLogging
			grouped, passedThrough := tt.dimension.PickLogging("test_source", tt.points)
			t.Logf("group %d pass=%d", len(grouped), len(passedThrough))
			// 验证分组数量
			assert.Equal(t, tt.wantGroupCount, len(grouped), "Group count mismatch")

			// 验证直通数据数量
			assert.Equal(t, tt.wantPassCount, len(passedThrough), "Pass-through count mismatch")

			// 验证分组键
			if len(tt.groupKeys) > 0 {
				actualKeys := make([]string, 0, len(grouped))
				for _, packet := range grouped {
					actualKeys = append(actualKeys, packet.RawGroupId)
				}

				// 排序以便比较
				sort.Strings(actualKeys)
				expectedKeys := make([]string, len(tt.groupKeys))
				copy(expectedKeys, tt.groupKeys)
				sort.Strings(expectedKeys)
				assert.Equal(t, expectedKeys, actualKeys, "Group keys mismatch")
			}
		})
	}
}
