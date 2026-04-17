// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTraceStatusFromCode(t *testing.T) {
	require.Equal(t, SpanStatusOK, TraceStatusFromCode(0))
	require.Equal(t, SpanStatusOK, TraceStatusFromCode(1))
	require.Equal(t, SpanStatusError, TraceStatusFromCode(2))
	require.Equal(t, SpanStatusInfo, TraceStatusFromCode(99))
}

func TestLogStatusFromSeverityNumber(t *testing.T) {
	require.Equal(t, "trace", LogStatusFromSeverityNumber(1, "fallback"))
	require.Equal(t, "debug", LogStatusFromSeverityNumber(7, "fallback"))
	require.Equal(t, "info", LogStatusFromSeverityNumber(12, "fallback"))
	require.Equal(t, "warn", LogStatusFromSeverityNumber(13, "fallback"))
	require.Equal(t, "error", LogStatusFromSeverityNumber(17, "fallback"))
	require.Equal(t, "fatal", LogStatusFromSeverityNumber(21, "fallback"))
	require.Equal(t, "unknown", LogStatusFromSeverityNumber(0, "fallback"))
	require.Equal(t, "fallback", LogStatusFromSeverityNumber(99, "fallback"))
}
