// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"
	T "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBoundaryTestPoint(t *T.T, wireSize int) *Point {
	t.Helper()
	newPoint := func(messageSize int) *Point {
		var kvs KVs
		kvs = kvs.Add("message", strings.Repeat("x", messageSize))
		return NewPoint("boundary", kvs, WithTimestamp(1700000000000000000))
	}

	pt := newPoint(wireSize)
	pbpts := PBPoints{Arr: []*PBPoint{pt.PBPoint()}}
	pt = newPoint(2*wireSize - pbpts.Size())
	pbpts.Arr[0] = pt.PBPoint()
	require.Equal(t, wireSize, pbpts.Size())
	return pt
}

func TestV2EncodeProtobufBoundary(t *T.T) {
	const bufSize = 1 << 20
	below := newBoundaryTestPoint(t, bufSize-1)
	exact := newBoundaryTestPoint(t, bufSize)
	oversized := newBoundaryTestPoint(t, bufSize+1)
	huge := newBoundaryTestPoint(t, bufSize+64)
	// Exercise the trim path in both approximate and precise size modes.
	require.LessOrEqual(t, oversized.Size(), bufSize)
	require.LessOrEqual(t, oversized.PBSize(), bufSize)

	normal := make([]*Point, 205)
	for i := range normal {
		var kvs KVs
		kvs = kvs.Add("message", fmt.Sprintf("normal-%d", i))
		normal[i] = NewPoint("normal", kvs, WithTimestamp(1700000000000000000+int64(i)))
	}
	consecutive := make([]*Point, 4096)
	for i := range consecutive {
		consecutive[i] = oversized
	}

	cases := []struct {
		name    string
		pts     []*Point
		want    []*Point
		skipped int
	}{
		{name: "empty"},
		{name: "nil-only", pts: []*Point{nil, nil}},
		{name: "normal-only", pts: normal, want: normal},
		{
			name: "below-boundary",
			pts:  slices.Concat([]*Point{below}, normal[:200]),
			want: slices.Concat([]*Point{below}, normal[:200]),
		},
		{
			name: "exact-fit",
			pts:  slices.Concat([]*Point{exact}, normal[:200]),
			want: slices.Concat([]*Point{exact}, normal[:200]),
		},
		{
			name: "oversized-first", pts: slices.Concat([]*Point{oversized}, normal[:200]),
			want: normal[:200], skipped: 1,
		},
		{
			name: "oversized-middle", pts: slices.Concat(normal[:5], []*Point{oversized}, normal[5:]),
			want: normal, skipped: 1,
		},
		{
			name: "oversized-last", pts: slices.Concat(normal[:200], []*Point{oversized}),
			want: normal[:200], skipped: 1,
		},
		{
			name: "nil-interleaved", pts: []*Point{nil, oversized, nil, normal[0], nil},
			want: normal[:1], skipped: 1,
		},
		{
			name: "consecutive-oversized", pts: slices.Concat(consecutive, normal[:200]),
			want: normal[:200], skipped: len(consecutive),
		},
		{name: "all-oversized", pts: consecutive, skipped: len(consecutive)},
		{
			name: "oversized-before-trim", pts: slices.Concat([]*Point{huge}, normal[:200]),
			want: normal[:200], skipped: 1,
		},
	}

	for _, approx := range []bool{true, false} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("approx=%t/%s", approx, tc.name), func(t *T.T) {
				callbackPoints := 0
				enc := GetEncoder(WithEncEncoding(Protobuf), WithApproxSize(approx), WithIgnoreLargePoint(true),
					WithEncFn(func(n int, payload []byte) error {
						assert.Positive(t, n)
						assert.NotEmpty(t, payload)
						callbackPoints += n
						return nil
					}))
				defer PutEncoder(enc)
				enc.EncodeV2(tc.pts)

				var got []*PBPoint
				buf := make([]byte, bufSize)
				for rounds := 0; ; rounds++ {
					require.LessOrEqual(t, rounds, len(tc.pts)+1, "encoder failed to terminate")
					payload, ok := enc.Next(buf)
					if !ok {
						break
					}
					require.LessOrEqual(t, len(payload), bufSize)
					var decoded PBPoints
					require.NoError(t, decoded.Unmarshal(payload))
					got = append(got, decoded.Arr...)
				}

				require.NoError(t, enc.LastErr())
				assert.Equal(t, tc.skipped, enc.SkippedPoints())
				assert.Equal(t, len(tc.want), enc.TotalPoints())
				assert.Equal(t, len(tc.want), callbackPoints)
				require.Len(t, got, len(tc.want))
				for i, pt := range got {
					actual, err := pt.Marshal()
					require.NoError(t, err)
					expected, err := tc.want[i].PBPoint().Marshal()
					require.NoError(t, err)
					assert.True(t, bytes.Equal(expected, actual), "point %d changed or out of order", i)
				}

				_, ok := enc.Next(buf)
				assert.False(t, ok, "encoder returned data after exhaustion")
			})
		}
	}
}

func TestV2EncodeBoundaryErrors(t *T.T) {
	const bufSize = 1 << 20
	oversized := newBoundaryTestPoint(t, bufSize+1)
	var kvs KVs
	kvs = kvs.Add("message", "normal")
	normal := NewPoint("normal", kvs, WithTimestamp(1700000000000000000))

	for _, approx := range []bool{true, false} {
		t.Run(fmt.Sprintf("strict/approx=%t", approx), func(t *T.T) {
			enc := GetEncoder(WithEncEncoding(Protobuf), WithApproxSize(approx), WithIgnoreLargePoint(false))
			defer PutEncoder(enc)
			enc.EncodeV2([]*Point{oversized, normal})
			buf := make([]byte, bufSize)
			for range 2 {
				_, ok := enc.Next(buf)
				assert.False(t, ok)
				assert.ErrorIs(t, enc.LastErr(), errTooSmallBuffer)
			}
		})
	}

	t.Run("callback-error-after-skip", func(t *T.T) {
		wantErr := errors.New("callback failed")
		calls := 0
		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true), WithEncFn(func(_ int, _ []byte) error {
			calls++
			return wantErr
		}))
		defer PutEncoder(enc)
		enc.EncodeV2([]*Point{oversized, normal})
		buf := make([]byte, bufSize)
		for range 2 {
			_, ok := enc.Next(buf)
			assert.False(t, ok)
			assert.ErrorIs(t, enc.LastErr(), wantErr)
		}
		assert.Equal(t, 1, calls)
	})
}
