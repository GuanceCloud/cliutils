// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"

	common "github.com/GuanceCloud/tracing-protos/opentelemetry-gen-go/common/v1"
	"github.com/stretchr/testify/require"
)

func TestFindAttribute(t *testing.T) {
	attrs := []*common.KeyValue{
		{Key: "a"},
		nil,
		{Key: "b"},
	}

	attr, idx, ok := FindAttribute(attrs, "b")
	require.True(t, ok)
	require.Equal(t, 2, idx)
	require.Equal(t, "b", attr.GetKey())
}

func TestAnyValueConversion(t *testing.T) {
	t.Run("scalar", func(t *testing.T) {
		v := &common.AnyValue{
			Value: &common.AnyValue_IntValue{IntValue: 42},
		}
		got, ok := AnyValueToInterface(v)
		require.True(t, ok)
		require.EqualValues(t, 42, got)

		s, ok := AnyValueToString(v)
		require.True(t, ok)
		require.Equal(t, "42", s)
	})

	t.Run("complex", func(t *testing.T) {
		v := &common.AnyValue{
			Value: &common.AnyValue_ArrayValue{
				ArrayValue: &common.ArrayValue{
					Values: []*common.AnyValue{
						{
							Value: &common.AnyValue_StringValue{StringValue: "a"},
						},
					},
				},
			},
		}

		s, ok := AnyValueToString(v)
		require.True(t, ok)
		require.Contains(t, s, "stringValue")
	})
}

func TestAttributesToStringMapAndMerge(t *testing.T) {
	attrs := []*common.KeyValue{
		{Key: "service.name", Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: "svc"}}},
		{Key: "count", Value: &common.AnyValue{Value: &common.AnyValue_IntValue{IntValue: 7}}},
	}

	tags := AttributesToStringMap(attrs, StringMapOptions{MaxValueLen: 8})
	require.Equal(t, "svc", tags["service.name"])
	require.Equal(t, "7", tags["count"])

	tagKVs := MergeStringMapsAsTags(tags)
	require.ElementsMatch(t, []string{"service_name", "count"}, []string{tagKVs[0].Key, tagKVs[1].Key})

	fieldKVs := MergeStringMapsAsFields(tags)
	require.ElementsMatch(t, []string{"service_name", "count"}, []string{fieldKVs[0].Key, fieldKVs[1].Key})
}
