// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"
	"time"

	common "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/common/v1"
	logs "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/logs/v1"
	metrics "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/metrics/v1"
	resource "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/resource/v1"
	trace "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/trace/v1"
	"github.com/stretchr/testify/require"
)

func TestParseResourceMetricsV2(t *testing.T) {
	ts := uint64(time.Unix(0, 123).UnixNano())
	pts := ParseResourceMetricsV2([]*metrics.ResourceMetrics{
		{
			Resource: &resource.Resource{
				Attributes: []*common.KeyValue{
					{Key: "service.name", Value: stringValue("svc")},
				},
			},
			ScopeMetrics: []*metrics.ScopeMetrics{
				{
					Metrics: []*metrics.Metric{
						{
							Name:        "cpu_usage",
							Unit:        "percent",
							Description: "CPU usage",
							Data: &metrics.Metric_Sum{
								Sum: &metrics.Sum{
									AggregationTemporality: metrics.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
									DataPoints: []*metrics.NumberDataPoint{
										{
											Attributes: []*common.KeyValue{
												{Key: "namespace", Value: stringValue("custom_ns")},
												{Key: "metric_name", Value: stringValue("custom_metric")},
											},
											TimeUnixNano: ts,
											Value:        &metrics.NumberDataPoint_AsInt{AsInt: 10},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, MetricsParserOptions{
		CollectorSourceIP: "127.0.0.1",
		MeasurementFromPointAttr: func(_ *metrics.Metric, tags map[string]string) string {
			return MetricMeasurementFromAttribute(tags, DefaultMetricMeasurement)
		},
		FieldName: func(metric *metrics.Metric, tags map[string]string) string {
			return MetricNameFromAttribute(tags, metric.GetName())
		},
	})

	require.Len(t, pts, 1)
	require.Equal(t, "custom_ns", pts[0].Name())
	require.EqualValues(t, int64(10), pts[0].Get("custom_metric"))
	require.Equal(t, "cumulative", pts[0].GetTag(DefaultMetricTemporality))
	require.Equal(t, "127.0.0.1", pts[0].GetTag(DefaultCollectorSourceTag))
}

func TestParseLogRequest(t *testing.T) {
	pts := ParseLogRequest([]*logs.ResourceLogs{
		{
			Resource: &resource.Resource{
				Attributes: []*common.KeyValue{
					{Key: AttrServiceName, Value: stringValue("svc")},
					{Key: "host.name", Value: stringValue("host-a")},
					{Key: "log.source", Value: stringValue("source-a")},
				},
			},
			ScopeLogs: []*logs.ScopeLogs{
				{
					LogRecords: []*logs.LogRecord{
						{
							TimeUnixNano:   uint64(time.Unix(0, 456).UnixNano()),
							SeverityNumber: logs.SeverityNumber_SEVERITY_NUMBER_WARN,
							Body:           stringValue("hello world"),
						},
					},
				},
			},
		},
	}, LogsParserOptions{
		CollectorSourceIP: "10.0.0.1",
		DKFingerprint:     "dk-1",
	})

	require.Len(t, pts, 1)
	require.Equal(t, "source-a", pts[0].Name())
	require.Equal(t, "hello world", pts[0].Get(DefaultMessageField))
	require.Equal(t, "warn", pts[0].GetTag(DefaultStatusTag))
	require.Equal(t, "svc", pts[0].GetTag(DefaultServiceTag))
	require.Equal(t, "host-a", pts[0].GetTag(DefaultHostTag))
	require.Equal(t, "10.0.0.1", pts[0].GetTag(DefaultCollectorSourceTag))
	require.Equal(t, "dk-1", pts[0].GetTag(DefaultFingerprintTag))
}

func TestParseResourceSpans(t *testing.T) {
	ts := uint64(time.Unix(0, 789).UnixNano())
	batches := ParseResourceSpans([]*trace.ResourceSpans{
		{
			Resource: &resource.Resource{
				Attributes: []*common.KeyValue{
					{Key: AttrServiceName, Value: stringValue("svc")},
				},
			},
			ScopeSpans: []*trace.ScopeSpans{
				{
					Spans: []*trace.Span{
						{
							TraceId:           []byte{1, 2, 3, 4},
							SpanId:            []byte{5, 6, 7, 8},
							Name:              "GET /health",
							StartTimeUnixNano: ts,
							EndTimeUnixNano:   ts + uint64(time.Millisecond),
							Kind:              trace.Span_SPAN_KIND_SERVER,
							Status:            &trace.Status{Code: trace.Status_STATUS_CODE_OK},
							Attributes: []*common.KeyValue{
								{Key: "http.method", Value: stringValue("GET")},
							},
							Events: []*trace.Span_Event{
								{
									Name: EventException,
									Attributes: []*common.KeyValue{
										{Key: AttrExceptionMessage, Value: stringValue("boom")},
									},
								},
							},
						},
					},
				},
			},
		},
	}, TracesParserOptions{
		CollectorSourceIP: "192.0.2.1",
		DKFingerprint:     "dk-2",
		SpanType: func(_, _ string, _, _ map[string]bool) string {
			return "entry"
		},
	})

	require.Len(t, batches, 1)
	require.Len(t, batches[0], 1)
	pt := batches[0][0]
	require.Equal(t, DefaultTracePointName, pt.Name())
	require.Equal(t, "svc", pt.GetTag(DefaultTraceServiceTag))
	require.Equal(t, "web", pt.GetTag(DefaultTraceSourceTypeTag))
	require.Equal(t, "entry", pt.GetTag(DefaultTraceSpanTypeTag))
	require.Equal(t, "ok", pt.GetTag(DefaultTraceStatusTag))
	require.Equal(t, "server", pt.GetTag(DefaultTraceSpanKindTag))
	require.Equal(t, "boom", pt.Get("error_message"))
	require.Equal(t, "192.0.2.1", pt.GetTag(DefaultCollectorSourceTag))
	require.Equal(t, "dk-2", pt.GetTag(DefaultTraceFingerprintTag))
	require.NotEmpty(t, pt.Get(DefaultTraceMessageField))
}

func stringValue(v string) *common.AnyValue {
	return &common.AnyValue{
		Value: &common.AnyValue_StringValue{StringValue: v},
	}
}
