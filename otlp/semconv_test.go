// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpanKindName(t *testing.T) {
	require.Equal(t, "server", SpanKindName(2))
	require.Equal(t, "unspecified", SpanKindName(99))
}

func TestPublicAttributeAlias(t *testing.T) {
	alias, ok := PublicAttributeAlias("http.request.method")
	require.True(t, ok)
	require.Equal(t, "http_method", alias)

	_, ok = PublicAttributeAlias("unknown")
	require.False(t, ok)
}

func TestShouldDropMetricAttribute(t *testing.T) {
	require.True(t, ShouldDropMetricAttribute("telemetry.sdk.name"))
	require.False(t, ShouldDropMetricAttribute("service.name"))
}

func TestSystemNameForService(t *testing.T) {
	require.Equal(t, "mysql", SystemNameForService(map[string]string{
		AttrDBSystem:        "mysql",
		AttrMessagingSystem: "kafka",
	}))
	require.Equal(t, "grpc", SystemNameForService(map[string]string{
		AttrRPCSystem: "grpc",
	}))
	require.Empty(t, SystemNameForService(nil))
}
