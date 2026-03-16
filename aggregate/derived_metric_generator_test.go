// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate

import (
	"testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestMetricGenerator_Basic(t *testing.T) {
	mg := NewMetricGenerator()
	assert.NotNil(t, mg)
}

func TestGenerateDerivedMetrics_Basic(t *testing.T) {
	// 测试全局函数
	packet := &DataPacket{
		RawGroupId: "test-trace-1",
		Token:      "test-token",
		DataType:   point.STracing,
		Points:     []*point.PBPoint{},
	}

	metrics := []*DerivedMetric{
		{
			Name:      "test_metric",
			Condition: "",
			Groupby:   []string{},
			Algorithm: &AggregationAlgo{
				Method:      COUNT,
				SourceField: "$trace_id",
			},
		},
	}

	points := GenerateDerivedMetrics(packet, metrics)
	assert.NotNil(t, points)
}

// 简化测试，确保基本功能
func TestMetricGenerator_SimpleCount(t *testing.T) {
	mg := NewMetricGenerator()

	packet := &DataPacket{
		RawGroupId: "simple-test",
		Token:      "test-token",
		DataType:   point.STracing,
		HasError:   true,
		Points:     []*point.PBPoint{},
	}

	// 简单的trace计数
	metric := &DerivedMetric{
		Name:      "simple_count",
		Condition: "",
		Groupby:   []string{},
		Algorithm: &AggregationAlgo{
			Method:      COUNT,
			SourceField: "$trace_id",
		},
	}

	points := mg.GenerateFromDataPacket(packet, metric)
	assert.NotNil(t, points)
	assert.Greater(t, len(points), 0)
}

// 测试条件评估
func TestMetricGenerator_Condition(t *testing.T) {
	mg := NewMetricGenerator()

	// 测试无条件
	metric1 := &DerivedMetric{
		Name:      "metric1",
		Condition: "", // 无条件
		Algorithm: &AggregationAlgo{
			Method: COUNT,
		},
	}

	packet := &DataPacket{
		RawGroupId: "cond-test",
		Token:      "test-token",
		DataType:   point.STracing,
		Points:     []*point.PBPoint{},
	}

	points1 := mg.GenerateFromDataPacket(packet, metric1)
	assert.NotNil(t, points1)

	// 测试无效条件
	metric2 := &DerivedMetric{
		Name:      "metric2",
		Condition: "{invalid_field=123}", // 无效条件
		Algorithm: &AggregationAlgo{
			Method: COUNT,
		},
	}

	points2 := mg.GenerateFromDataPacket(packet, metric2)
	assert.Nil(t, points2) // 应该返回nil
}

// 测试特殊字段
func TestMetricGenerator_SpecialFields(t *testing.T) {
	mg := NewMetricGenerator()

	testCases := []struct {
		name         string
		sourceField  string
		packet       *DataPacket
		expectPoints bool
	}{
		{
			name:        "trace_id",
			sourceField: "$trace_id",
			packet: &DataPacket{
				RawGroupId: "trace-1",
				Token:      "test-token",
				DataType:   point.STracing,
				Points:     []*point.PBPoint{},
			},
			expectPoints: true, // trace_id应该生成点
		},
		{
			name:        "error_flag with error",
			sourceField: "$error_flag",
			packet: &DataPacket{
				RawGroupId: "trace-2",
				Token:      "test-token",
				DataType:   point.STracing,
				HasError:   true,
				Points:     []*point.PBPoint{},
			},
			expectPoints: true, // error_flag应该生成点
		},
		{
			name:        "error_flag without error",
			sourceField: "$error_flag",
			packet: &DataPacket{
				RawGroupId: "trace-3",
				Token:      "test-token",
				DataType:   point.STracing,
				HasError:   false,
				Points:     []*point.PBPoint{},
			},
			expectPoints: true, // 即使没有错误也应该生成点（值为0）
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			metric := &DerivedMetric{
				Name:      "test_" + tc.name,
				Condition: "",
				Groupby:   []string{},
				Algorithm: &AggregationAlgo{
					Method:      COUNT,
					SourceField: tc.sourceField,
				},
			}

			points := mg.GenerateFromDataPacket(tc.packet, metric)
			if tc.expectPoints {
				assert.NotNil(t, points)
				assert.Greater(t, len(points), 0)
			} else {
				// 允许为nil
				t.Logf("points: %v", points)
			}
		})
	}
}

func TestBuildDerivedMetricBatches_TraceTotalCount(t *testing.T) {
	packet := &DataPacket{
		RawGroupId: "1000000000",
		Token:      "test-token",
		DataType:   point.STracing,
		HasError:   true,
		Points:     MockTrace(),
	}

	batches := BuildDerivedMetricBatches(packet, []*DerivedMetric{TraceTotalCount}, 0)
	assert.Len(t, batches, 1)

	batch := batches[0]
	assert.NotNil(t, batch)
	assert.Equal(t, DefaultDerivedMetricWindowSeconds, batch.AggregationOpts["value"].Window)
	assert.Equal(t, SUM, batch.AggregationOpts["value"].Method)
	assert.Len(t, batch.Points.Arr, 1)

	pt := point.FromPB(batch.Points.Arr[0])
	assert.Equal(t, "trace_total_count", pt.Name())

	value, ok := pt.GetF("value")
	assert.True(t, ok)
	assert.Equal(t, 1.0, value)

	for _, batch := range batches {
		// t.Logf("batch =%s", batch.String())
		for _, pt := range batch.Points.Arr {
			t.Logf("point string=%s", pt.String())
		}
	}
}

func TestBuildDerivedMetricBatches_SpanTotalCount(t *testing.T) {
	packet := &DataPacket{
		RawGroupId: "1000000000",
		Token:      "test-token",
		DataType:   point.STracing,
		HasError:   true,
		Points:     MockTrace(),
	}

	batches := BuildDerivedMetricBatches(packet, []*DerivedMetric{SpanTotalCount}, 0)
	assert.Len(t, batches, 1)

	batch := batches[0]
	assert.NotNil(t, batch)
	assert.Equal(t, DefaultDerivedMetricWindowSeconds, batch.AggregationOpts["value"].Window)
	assert.Equal(t, SUM, batch.AggregationOpts["value"].Method)
	assert.Len(t, batch.Points.Arr, 1)

	pt := point.FromPB(batch.Points.Arr[0])
	assert.Equal(t, "span_total_count", pt.Name())

	value, ok := pt.GetF("value")
	assert.True(t, ok)
	assert.Equal(t, float64(len(packet.Points)), value)
}

func TestBuildDerivedMetricBatches_TraceErrorCount(t *testing.T) {
	packet := &DataPacket{
		RawGroupId: "1000000000",
		Token:      "test-token",
		DataType:   point.STracing,
		HasError:   true,
		Points:     MockTrace(),
	}

	batches := BuildDerivedMetricBatches(packet, []*DerivedMetric{TraceErrorCount}, 0)
	assert.Len(t, batches, 1)

	batch := batches[0]
	assert.NotNil(t, batch)
	assert.Equal(t, DefaultDerivedMetricWindowSeconds, batch.AggregationOpts["value"].Window)
	assert.Equal(t, SUM, batch.AggregationOpts["value"].Method)
	assert.Len(t, batch.Points.Arr, 1)

	pt := point.FromPB(batch.Points.Arr[0])
	assert.Equal(t, "trace_error_count", pt.Name())

	value, ok := pt.GetF("value")
	assert.True(t, ok)
	assert.Equal(t, 1.0, value)
}
