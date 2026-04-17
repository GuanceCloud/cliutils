// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package otlp

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/stretchr/testify/require"
)

func TestNormalizeContentType(t *testing.T) {
	require.Equal(t, ContentTypeJSON, NormalizeContentType("Application/JSON; charset=utf-8"))
	require.Equal(t, ContentTypeProtobuf, NormalizeContentType(" application/x-protobuf "))
}

func TestUnmarshal(t *testing.T) {
	req := buildStruct()

	t.Run("protobuf", func(t *testing.T) {
		body, err := proto.Marshal(req)
		require.NoError(t, err)

		got := &structpb.Struct{}
		require.NoError(t, Unmarshal(body, ContentTypeProtobuf, got))
		require.Equal(t, req.String(), got.String())
	})

	t.Run("json", func(t *testing.T) {
		body, err := protojson.Marshal(req)
		require.NoError(t, err)

		got := &structpb.Struct{}
		require.NoError(t, Unmarshal(body, ContentTypeJSON, got))
		require.Equal(t, req.String(), got.String())
	})
}

func buildStruct() *structpb.Struct {
	return &structpb.Struct{
		Fields: map[string]*structpb.Value{
			"name": {
				Kind: &structpb.Value_StringValue{StringValue: "op"},
			},
			"service": {
				Kind: &structpb.Value_StringValue{StringValue: "svc"},
			},
		},
	}
}
