// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"bytes"
	"encoding/json"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/point/gogopb"
	proto "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test if encode change points' payload
func TestIdempotent(t *T.T) {
	cases := []struct {
		name  string
		pts   []*Point
		batch int
	}{
		{
			name:  "ok-32/3",
			pts:   RandPoints(32),
			batch: 3,
		},

		{
			name:  "ok-32/1",
			pts:   RandPoints(32),
			batch: 1,
		},

		{
			name:  "ok-32/0",
			pts:   RandPoints(32),
			batch: 0,
		},

		{
			name:  "nothing",
			pts:   RandPoints(0),
			batch: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			enc := GetEncoder(WithEncBatchSize(tc.batch))
			defer PutEncoder(enc)

			p1, err := enc.Encode(tc.pts)
			assert.NoError(t, err)

			p2, err := enc.Encode(tc.pts)
			assert.NoError(t, err)

			assert.Equal(t, p1, p2)
		})

		// test encode pb
		t.Run(tc.name+"-pb", func(t *T.T) {
			enc := GetEncoder(WithEncBatchSize(tc.batch), WithEncEncoding(Protobuf))
			defer PutEncoder(enc)

			p1, err := enc.Encode(tc.pts)
			assert.NoError(t, err)

			p2, err := enc.Encode(tc.pts)
			assert.NoError(t, err)

			assert.Equal(t, p1, p2)
		})
	}
}

func TestEncode(t *T.T) {
	r := NewRander(WithKVSorted(true))

	var (
		randPts = r.Rand(1000)

		simplePts = []*Point{
			NewPointV2(`abc`, NewKVs(map[string]interface{}{"f1": "fv1", "f2": "fv2", "f3": "fv3"}).
				AddTag(`t1`, `tv1`).
				AddTag(`t2`, `tv2`).
				AddTag(`t3`, `tv3`), WithTime(time.Unix(0, 123))),

			NewPointV2(`def`, NewKVs(map[string]interface{}{"f1": "fv1", "f2": "fv2", "f3": "fv3"}).
				AddTag(`t1`, `tv1`).
				AddTag(`t2`, `tv2`).
				AddTag(`t3`, `tv3`), WithTime(time.Unix(0, 123))),

			NewPointV2(`xyz`, NewKVs(map[string]interface{}{"f1": "fv1", "f2": "fv2", "f3": "fv3"}).
				AddTag(`t1`, `tv1`).
				AddTag(`t2`, `tv2`).
				AddTag(`t3`, `tv3`), WithTime(time.Unix(0, 123))),
		}

		__fn = func(n int, data []byte) error {
			t.Logf("batch size: %d, payload: %d", n, len(data))
			return nil
		}
	)

	cases := []struct {
		name   string
		pts    []*Point
		bsize  int
		fn     EncodeFn
		gz     bool
		expect [][]byte
	}{
		{
			name:  "single-point",
			bsize: 10,
			expect: [][]byte{
				[]byte(`abc,t1=tv1,t2=tv2,t3=tv3 f1="fv1",f2="fv2",f3="fv3" 123`),
			},

			pts: func() []*Point {
				x, err := NewPoint("abc", map[string]string{
					"t1": "tv1",
					"t2": "tv2",
					"t3": "tv3",
				}, map[string]interface{}{
					"f1": "fv1",
					"f2": "fv2",
					"f3": "fv3",
				}, WithTime(time.Unix(0, 123)), WithKeySorted(true))

				require.NoError(t, err)

				t.Logf("pt: %s", x.Pretty())
				return []*Point{x}
			}(),
		},

		{
			name:  "random-point",
			bsize: 256,
			pts:   randPts,
			expect: func() [][]byte {
				enc := GetEncoder(WithEncBatchSize(256))
				defer PutEncoder(enc)

				x, err := enc.Encode(randPts)
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:  "random-point-with-callback",
			bsize: 256,
			pts:   randPts,
			fn:    __fn,
			expect: func() [][]byte {
				enc := GetEncoder(WithEncBatchSize(256))
				defer PutEncoder(enc)

				bufs, err := enc.Encode(randPts)
				assert.NoError(t, err)

				if len(randPts)%256 == 0 {
					assert.Equal(t, len(randPts)/256, len(bufs))
				} else {
					assert.Equal(t, len(randPts)/256+1, len(bufs), "randPts: %d", len(randPts))
				}

				for i, buf := range bufs {
					t.Logf("get %dth %q", i, buf)
					if i != len(bufs)-1 {
						assert.Equal(t, 256, len(bytes.Split(buf, []byte("\n"))))
					}
				}

				return bufs
			}(),
		},

		{
			name:  "simple-point-with-callback",
			bsize: 1,
			pts:   simplePts,
			fn:    __fn,
			expect: func() [][]byte {
				enc := GetEncoder(WithEncBatchSize(1))
				defer PutEncoder(enc)

				bufs, err := enc.Encode(simplePts)
				assert.NoError(t, err)

				assert.Equal(t, len(simplePts), len(bufs))

				for i, buf := range bufs {
					t.Logf("get %dth %q", i, buf)
					assert.Equal(t, 1, len(bytes.Split(buf, []byte("\n"))))
				}

				return bufs
			}(),
			// expect: simplePtsExpect,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			enc := GetEncoder(WithEncBatchSize(tc.bsize), WithEncFn(tc.fn))
			defer PutEncoder(enc)

			payloads, err := enc.Encode(tc.pts)
			assert.NoError(t, err)

			assert.Equal(t, len(tc.expect), len(payloads))

			for idx, ex := range tc.expect {
				assert.Equal(t, len(ex), len(payloads[idx]), "not equal at index: %d, gz: %q, fn: %q", idx, tc.gz, tc.fn)
				assert.Equal(t, ex, payloads[idx], "[%d] expect %s, get %s", idx, string(ex), string(payloads[idx]))
			}
		})

		t.Run(tc.name+"-pb", func(t *T.T) {
			enc := GetEncoder(WithEncBatchSize(tc.bsize),
				WithEncFn(tc.fn),
				WithEncEncoding(Protobuf))
			defer PutEncoder(enc)

			payloads, err := enc.Encode(tc.pts)

			assert.NoError(t, err)

			assert.Equal(t, len(tc.expect), len(payloads))

			// check PB unmarshal and compress ratio
			for idx := range tc.expect {
				var pbpts PBPoints
				assert.NoError(t, proto.Unmarshal(payloads[idx], &pbpts))
			}

			var lps [][]byte
			// convert PB to line-protocol, check equality
			for _, p := range payloads {
				lp, err := PB2LP(p)
				assert.NoError(t, err)
				lps = append(lps, lp)
			}

			lpbody := string(bytes.Join(lps, []byte("\n")))
			assert.Equal(t, string(bytes.Join(tc.expect, []byte("\n"))), lpbody)
		})
	}
}

func TestEncodeWithBytesLimit(t *T.T) {
	t.Run(`bytes-limite`, func(t *T.T) {
		r := NewRander(WithFixedTags(true), WithRandText(3))
		pts := r.Rand(1000)

		// add anypb data
		for _, pt := range pts {
			pt.MustAdd("s-arr", []string{"s1", "s2"})
			pt.MustAdd("i-arr", []int{1, 2})
			pt.MustAdd("b-arr", []bool{true, false})
			pt.MustAdd("f-arr", []float64{1.414, 3.14})
		}

		bytesBatchSize := 128 * 1024
		enc := GetEncoder(WithEncBatchBytes(bytesBatchSize), WithEncFn(func(n int, payload []byte) error {
			t.Logf("points: %d, payload: %dbytes", n, len(payload))
			return nil
		}))
		defer PutEncoder(enc)

		batches, err := enc.Encode(pts)
		assert.NoError(t, err)
		for idx, b := range batches {
			t.Logf("[%d] batch: %d", idx, len(b))
		}
	})
}

func TestEncodeTags(t *T.T) {
	t.Run("tag-value-begins-with-slash", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(LineProtocol))
		defer PutEncoder(enc)

		arr := func() []*Point {
			x, err := NewPoint("abc", map[string]string{
				"service": "/sf-webproxy/api/online_status",
			}, map[string]interface{}{
				"f3": "fv3",
			}, WithTime(time.Unix(0, 123)))

			require.NoError(t, err)

			t.Logf("pt: %s", x.Pretty())
			return []*Point{x}
		}()

		res, err := enc.Encode(arr)
		assert.NoError(t, err)
		t.Logf("%q", res[0])

		dec := GetDecoder(WithDecEncoding(LineProtocol))
		defer PutDecoder(dec)

		pts, err := dec.Decode([]byte(`abc,service=/sf-webproxy/api/online_status f3="fv3" 123`))
		assert.NoError(t, err)
		t.Logf("%s", pts[0].LineProto())
	})
}

func TestEncodeLen(t *T.T) {
	t.Run("encode-len", func(t *T.T) {
		r := NewRander(WithFixedTags(true), WithRandText(3))
		pts := r.Rand(1000)

		ptsTotalSize := 0
		for _, pt := range pts {
			ptsTotalSize += pt.Size()
		}

		enc := GetEncoder()
		defer PutEncoder(enc)

		data1, err := enc.Encode(pts)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(data1))

		gzData1 := cliutils.MustGZip(data1[0])
		t.Logf("lp data: %d bytes, gz: %d, pts size: %d", len(data1[0]), len(gzData1), ptsTotalSize)

		// setup pb
		WithEncEncoding(Protobuf)(enc)

		data2, err := enc.Encode(pts)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(data2))

		gzData2 := cliutils.MustGZip(data2[0])
		t.Logf("pb data: %d bytes, gz: %d, pts size: %d", len(data2[0]), len(gzData2), ptsTotalSize)

		t.Logf("ratio: %f, gz ration: %f",
			100*float64(len(data2[0]))/float64(len(data1[0])),
			100*float64(len(gzData2))/float64(len(gzData1)))
	})
}

func BenchmarkEncode(b *T.B) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	pts := r.Rand(1000)

	b.Run("bench-encode-lp", func(b *T.B) {
		enc := GetEncoder()
		defer PutEncoder(enc)

		for i := 0; i < b.N; i++ {
			_, err := enc.Encode(pts)
			assert.NoError(b, err)
		}
	})

	b.Run("bench-encode-pb", func(b *T.B) {
		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)

		for i := 0; i < b.N; i++ {
			_, err := enc.Encode(pts)
			assert.NoError(b, err)
		}
	})

	b.Run("bench-encode-json", func(b *T.B) {
		enc := GetEncoder(WithEncEncoding(JSON))
		defer PutEncoder(enc)

		for i := 0; i < b.N; i++ {
			_, err := enc.Encode(pts)
			assert.NoError(b, err)
		}
	})
}

func TestGoGoPBDecodePB(t *T.T) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	pts := r.Rand(3)

	enc := GetEncoder(WithEncEncoding(Protobuf))
	defer PutEncoder(enc)

	arr, err := enc.Encode(pts)
	assert.NoError(t, err)

	var gogopts gogopb.PBPoints
	assert.NoError(t, gogopts.Unmarshal(arr[0]))

	j, err := json.MarshalIndent(gogopts, "", "  ")
	assert.NoError(t, err)

	t.Logf("gogopts:\n%s", string(j))
}
