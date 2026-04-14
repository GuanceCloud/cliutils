// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	metrics "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/metrics/v1"

	"github.com/stretchr/testify/require"
)

func TestNumberDataPointHelpers(t *testing.T) {
	pt := &metrics.NumberDataPoint{
		TimeUnixNano: uint64(time.Unix(0, 123).UnixNano()),
		Value:        &metrics.NumberDataPoint_AsInt{AsInt: 10},
	}

	value, ok := NumberDataPointValue(pt)
	require.True(t, ok)
	require.EqualValues(t, 10, value)

	out := NumberDataPointToPoint("otel", "count", nil, pt, nil)
	require.Equal(t, "otel", out.Name())
	require.EqualValues(t, int64(10), out.Get("count"))
}

func TestSummaryDataPointToPoint(t *testing.T) {
	summary := &metrics.SummaryDataPoint{
		TimeUnixNano: uint64(time.Unix(0, 456).UnixNano()),
		Count:        3,
		Sum:          12.5,
	}

	out := SummaryDataPointToPoint("otel", "latency", point.KVs{}, summary, nil)
	require.EqualValues(t, uint64(3), out.Get("latency_count"))
	require.EqualValues(t, 12.5, out.Get("latency_sum"))
}
