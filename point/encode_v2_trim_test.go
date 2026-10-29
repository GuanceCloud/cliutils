// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"
	T "testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV2EncodeTrimProgress(t *T.T) {
	const bufSize = 1 << 20
	var kvs KVs
	kvs = kvs.Add("m", "")
	small := NewPoint("x", kvs, WithTimestamp(1700000000000000000))
	tiny := NewPoint("x", nil, WithTimestamp(0))
	debugPoint := NewPoint("log", NewKVs(map[string]any{"message": "hello"}), WithTimestamp(1700000000000000000))
	for range 20 {
		debugPoint.AddDebug(&Debug{Info: "x"})
	}

	cases := []struct {
		name      string
		pts       []*Point
		bufSize   int
		approx    bool
		oversized bool
	}{
		{
			name: "one-mib-boundary",
			pts: []*Point{newBoundaryTestPoint(t, bufSize-20), small,
				NewPoint("y", kvs, WithTimestamp(1700000000000000001))},
			bufSize: bufSize, approx: true,
		},
		{
			name: "precise-size-with-envelope",
			pts: []*Point{tiny, NewPoint("y", nil, WithTimestamp(0)),
				NewPoint("z", nil, WithTimestamp(0))},
			bufSize: 3 * tiny.PBSize(),
		},
		{
			name:    "trim-exactly-remaining",
			pts:     []*Point{debugPoint, debugPoint, debugPoint},
			bufSize: 3 * debugPoint.Size(), approx: true,
		},
		{
			name:    "trim-more-than-remaining",
			pts:     []*Point{debugPoint, debugPoint, debugPoint, debugPoint, debugPoint},
			bufSize: 5 * debugPoint.Size(), approx: true,
		},
		{
			name:    "oversized-first-with-tiny-suffix",
			pts:     []*Point{newBoundaryTestPoint(t, bufSize+1), tiny, tiny},
			bufSize: bufSize, approx: true, oversized: true,
		},
	}

	for _, tc := range cases {
		for _, ignore := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/ignore=%t", tc.name, ignore), func(t *T.T) {
				// Check progress before Next: a regression must fail here rather than
				// leave the test stuck in the encoder's retry loop.
				if !t.Run("trim-progress", func(t *T.T) {
					enc := GetEncoder(WithIgnoreLargePoint(ignore))
					defer PutEncoder(enc)
					enc.EncodeV2(tc.pts)
					estimated := 0
					for _, pt := range tc.pts {
						enc.pbpts.Arr = append(enc.pbpts.Arr, pt.PBPoint())
						if tc.approx {
							estimated += pt.Size()
						} else {
							estimated += pt.PBSize()
						}
					}
					require.LessOrEqual(t, estimated, tc.bufSize)
					require.Greater(t, enc.pbpts.Size(), tc.bufSize)
					enc.lastPtsIdx = len(tc.pts)
					enc.totalPts = len(tc.pts)
					after, err := enc.tryTrim(estimated, estimated, tc.bufSize)
					require.Positive(t, enc.lastPtsIdx, "trimming must not rewind to the batch start")
					if tc.oversized {
						assert.Equal(t, 1, enc.SkippedPoints())
						assert.Empty(t, enc.pbpts.Arr)
						if !ignore {
							require.ErrorIs(t, err, errTooSmallBuffer)
							return
						}
					} else {
						require.Positive(t, after)
						assert.LessOrEqual(t, after, tc.bufSize)
						assert.NotEmpty(t, enc.pbpts.Arr)
					}
					require.NoError(t, err)
				}) {
					return
				}

				callbackPoints := 0
				enc := GetEncoder(WithEncEncoding(Protobuf), WithApproxSize(tc.approx), WithIgnoreLargePoint(ignore),
					WithEncFn(func(n int, _ []byte) error {
						callbackPoints += n
						return nil
					}))
				defer PutEncoder(enc)
				enc.EncodeV2(tc.pts)
				buf := make([]byte, tc.bufSize)
				var got []*PBPoint
				for rounds := 0; ; rounds++ {
					require.LessOrEqual(t, rounds, len(tc.pts)+1)
					payload, ok := enc.Next(buf)
					if !ok {
						break
					}
					var batch PBPoints
					require.NoError(t, batch.Unmarshal(payload))
					got = append(got, batch.Arr...)
				}

				want := tc.pts
				if tc.oversized {
					assert.Equal(t, 1, enc.SkippedPoints())
					want = want[1:]
					if !ignore {
						require.ErrorIs(t, enc.LastErr(), errTooSmallBuffer)
						assert.Empty(t, got)
						assert.Zero(t, callbackPoints)
						_, ok := enc.Next(buf)
						assert.False(t, ok)
						return
					}
				} else {
					assert.Zero(t, enc.SkippedPoints())
				}
				require.NoError(t, enc.LastErr())
				require.Len(t, got, len(want))
				assert.Equal(t, len(want), callbackPoints)
				assert.Equal(t, len(want), enc.TotalPoints())
				for i, pt := range want {
					expected, err := pt.PBPoint().Marshal()
					require.NoError(t, err)
					actual, err := got[i].Marshal()
					require.NoError(t, err)
					assert.Equal(t, expected, actual, "point %d changed or out of order", i)
				}
			})
		}
	}
}
