// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"math"
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
			name:  "ok-32#3",
			pts:   RandPoints(32),
			batch: 3,
		},

		{
			name:  "ok-32#1",
			pts:   RandPoints(32),
			batch: 1,
		},

		{
			name:  "ok-32#0",
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

			p1, err := enc.Encode(tc.pts)
			assert.NoError(t, err)
			PutEncoder(enc)

			enc = GetEncoder(WithEncBatchSize(tc.batch))
			p2, err := enc.Encode(tc.pts)
			assert.NoError(t, err)
			PutEncoder(enc)

			assert.Equal(t, p1, p2)
		})

		// test encode pb
		t.Run(tc.name+"-pb", func(t *T.T) {
			enc := GetEncoder(WithEncBatchSize(tc.batch), WithEncEncoding(Protobuf))

			p1, err := enc.Encode(tc.pts)
			assert.NoError(t, err)
			PutEncoder(enc)

			enc = GetEncoder(WithEncBatchSize(tc.batch), WithEncEncoding(Protobuf))
			p2, err := enc.Encode(tc.pts)
			assert.NoError(t, err)
			PutEncoder(enc)

			assert.Equal(t, p1, p2)
		})
	}
}

// TestEncodeEqualty test equality on
//   - multi-part encode: multiple points splited into multiple parts
//   - line-protocol/protobuf: points encode between line-protocol and protobuf are equal
func TestEncodeEqualty(t *T.T) {
	r := NewRander(WithKVSorted(true), WithRandFields(1), WithRandTags(1))

	nrand := 6
	randBsize := 3

	var (
		randPts = r.Rand(nrand)

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
			bsize: randBsize,
			pts:   randPts,
			expect: func() [][]byte {
				enc := GetEncoder(WithEncBatchSize(randBsize))
				defer PutEncoder(enc)

				x, err := enc.Encode(randPts)
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:  "random-point-with-callback",
			bsize: randBsize,
			pts:   randPts,
			fn:    __fn,
			expect: func() [][]byte {
				enc := GetEncoder(WithEncBatchSize(randBsize))
				defer PutEncoder(enc)

				bufs, err := enc.Encode(randPts)
				assert.NoError(t, err)

				if len(randPts)%randBsize == 0 {
					assert.Equal(t, len(randPts)/randBsize, len(bufs))
				} else {
					assert.Equal(t, len(randPts)/randBsize+1, len(bufs), "randPts: %d", len(randPts))
				}

				for i, buf := range bufs {
					t.Logf("get %dth batch:\n%s", i, buf)
					if i != len(bufs)-1 {
						assert.Equal(t, randBsize, len(bytes.Split(buf, []byte("\n"))))
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
					t.Logf("get %dth batch:\n%s", i, buf)
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

			assert.Len(t, enc.pbpts.Arr, 0)

			payloads, err := enc.Encode(tc.pts)

			assert.NoError(t, err)

			assert.Equal(t, len(tc.expect), len(payloads))

			// check PB unmarshal
			for idx := range tc.expect {
				var pbpts PBPoints
				assert.NoError(t, proto.Unmarshal(payloads[idx], &pbpts))
			}

			// convert PB to line-protocol, check equality
			for idx, p := range payloads {
				lp, err := PB2LP(p)
				assert.NoError(t, err)
				t.Logf("pb -> lp:\n%s", lp)

				assert.Equal(t, string(tc.expect[idx]), string(lp))
			}
		})
	}

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
		assert.Equal(t, 1, len(data1), "encoder: %s", enc.String())

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

	buf := make([]byte, 1<<20)

	b.ResetTimer()

	b.Run("bench-encode-json", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(JSON))
			enc.Encode(pts)
			PutEncoder(enc)
		}
	})

	b.Run("bench-encode-lp", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder()
			enc.Encode(pts)
			PutEncoder(enc)
		}
	})

	b.Run("bench-encode-pb", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(Protobuf), WithEncBatchBytes(1<<20))
			enc.Encode(pts)
			PutEncoder(enc)
		}
	})

	b.Run("v2-encode-pb", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(Protobuf))
			enc.EncodeV2(pts)

			for {
				if _, ok := enc.Next(buf); ok {
				} else {
					break
				}
			}

			PutEncoder(enc)
		}
	})

	b.Run("v2-encode-lp", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(LineProtocol))
			enc.EncodeV2(pts)

			for {
				if _, ok := enc.Next(buf); ok {
				} else {
					break
				}
			}

			PutEncoder(enc)
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

func BenchmarkV2Encode(b *T.B) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	randPts := r.Rand(10000)

	buf := make([]byte, 1<<20)

	b.Logf("start...")

	b.ResetTimer()

	b.Run("encode-v1", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(Protobuf))
			enc.Encode(randPts)

			assert.NoError(b, enc.LastErr())
			PutEncoder(enc)
		}
	})

	b.Run("Next", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(Protobuf))
			enc.EncodeV2(randPts)

			for {
				if _, ok := enc.Next(buf); ok {
					buf = buf[0:]
				} else {
					break
				}
			}

			assert.NoError(b, enc.LastErr())
			PutEncoder(enc)
		}
	})
}

func TestV2Encode(t *T.T) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	randPts := r.Rand(10000)

	t.Run("encode-pb", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		dec := GetDecoder(WithDecEncoding(Protobuf))
		defer PutDecoder(dec)

		var (
			decodePts []*Point
			round     int
			buf       = make([]byte, 1<<20) // KB
		)

		for {
			if x, ok := enc.Next(buf); ok {
				decPts, err := dec.Decode(x)
				assert.NoErrorf(t, err, "decode %s failed", x)

				t.Logf("encoded %d(%d remain) bytes, %d points, encoder: %s",
					len(x), (len(buf) - len(x)), len(decPts), enc.String())
				decodePts = append(decodePts, decPts...)
				round++
				assert.Equal(t, round, enc.parts)
			} else {
				break
			}
		}

		assert.NoError(t, enc.LastErr())

		for i, pt := range decodePts {
			assert.Equal(t, randPts[i].Pretty(), pt.Pretty())
		}
	})

	t.Run("encode-lp", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(LineProtocol))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		dec := GetDecoder(WithDecEncoding(LineProtocol))
		defer PutDecoder(dec)

		var (
			decodePts []*Point
			round     int
			buf       = make([]byte, 1<<20)
		)

		for {
			if x, ok := enc.Next(buf); ok {
				decPts, err := dec.Decode(x)
				assert.NoErrorf(t, err, "decode %s failed", x)

				t.Logf("encoded %d(%d remain) bytes, %d points, encoder: %s",
					len(x), (len(buf) - len(x)), len(decPts), enc.String())

				decodePts = append(decodePts, decPts...)
				round++
				assert.Equal(t, round, enc.parts)
			} else {
				break
			}
		}

		assert.NoError(t, enc.LastErr())

		for i, pt := range decodePts {
			assert.Equal(t, randPts[i].Pretty(), pt.Pretty())
		}
	})

	t.Run("too-small-buffer-lp", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(LineProtocol))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		buf := make([]byte, 4) // too small

		for {
			_, ok := enc.Next(buf)
			require.False(t, ok)
			break
		}

		assert.Error(t, enc.LastErr())
		t.Logf("go error: %s", enc.LastErr())
	})

	t.Run("too-small-buffer-pb", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		buf := make([]byte, 4) // too small

		for {
			_, ok := enc.Next(buf)
			require.False(t, ok)
			break
		}

		assert.Error(t, enc.LastErr())
		t.Logf("go error: %s", enc.LastErr())
	})

	t.Run("with-encode-callback-line-proto", func(t *T.T) {
		fn := func(n int, buf []byte) error {
			assert.Equal(t, 2, n)
			assert.True(t, len(buf) > 0)

			t.Logf("buf: %q", buf)
			return nil
		}

		buf := make([]byte, 1<<20)
		randPts := r.Rand(2)
		enc := GetEncoder(WithEncFn(fn), WithEncEncoding(LineProtocol))
		enc.EncodeV2(randPts)
		for {
			if _, ok := enc.Next(buf); !ok {
				break
			}
		}
		PutEncoder(enc)
	})

	t.Run("with-encode-callback-protobuf", func(t *T.T) {
		fn := func(n int, buf []byte) error {
			assert.Equal(t, 2, n)
			assert.NotNil(t, buf)

			t.Logf("buf: %q", buf)
			return nil
		}

		randPts := r.Rand(2)

		enc := GetEncoder(WithEncFn(fn), WithEncEncoding(Protobuf))
		enc.EncodeV2(randPts)
		buf := make([]byte, 1<<20)

		for {
			if _, ok := enc.Next(buf); !ok {
				break
			}
		}
		PutEncoder(enc)
	})
}

func BenchmarkPointsSize(b *T.B) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	randPts := r.Rand(1)

	b.ResetTimer()
	b.Run("pt.pt.size", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			randPts[0].pt.Size()
		}
	})

	b.Run("pt.size", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			randPts[0].Size()
		}
	})
}

func TestPointsSize(t *T.T) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	randPts := r.Rand(1)

	t.Logf("pt.pt.size: %d, pt.size: %d",
		randPts[0].pt.Size(),
		randPts[0].Size())
}

func TestEncodePayloadSize(t *T.T) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	randPts := r.Rand(1000)

	enc := GetEncoder(WithEncEncoding(Protobuf))
	arr, err := enc.Encode(randPts)
	assert.NoError(t, err)
	assert.Len(t, arr, 1)

	pbPayload := arr[0]
	PutEncoder(enc)

	enc = GetEncoder(WithEncEncoding(LineProtocol))
	arr, err = enc.Encode(randPts)
	assert.NoError(t, err)
	assert.Len(t, arr, 1)
	lpPayload := arr[0]
	PutEncoder(enc)

	// gzip compression
	gzSize := func(payload []byte) int {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		_, err := gw.Write(payload)
		assert.NoError(t, err)
		assert.NoError(t, gw.Close())
		return buf.Len()
	}

	t.Logf("pbsize: %d, lpsize: %d, gz pb: %d, gz lp: %d",
		len(pbPayload), len(lpPayload), gzSize(pbPayload), gzSize(lpPayload))
}

func TestEncodeInfField(t *T.T) {

	var kvs KVs
	kvs = kvs.AddV2("f1", math.Inf(1), true)
	kvs = kvs.AddV2("f2", math.Inf(-1), true)
	kvs = kvs.AddV2("f3", 123, true)

	pt := NewPointV2("some", kvs)

	t.Logf("point: %s", pt.Pretty())

	t.Run("inf-lineproto", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(LineProtocol))
		defer PutEncoder(enc)

		enc.EncodeV2([]*Point{pt})

		buf := make([]byte, 1<<20)

		for {
			if res, ok := enc.Next(buf); ok {
				t.Logf("res: %s", string(res))
			} else {
				break
			}
		}

		assert.NoError(t, enc.LastErr())
	})

	t.Run("inf-protobuf", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)

		enc.EncodeV2([]*Point{pt})

		buf := make([]byte, 1<<20)

		for {
			if res, ok := enc.Next(buf); ok {
				dec := GetDecoder(WithDecEncoding(Protobuf))
				defer PutDecoder(dec)

				pts, err := dec.Decode(res)
				assert.NoError(t, err)

				t.Logf("decode point: %s", pts[0].Pretty())

				assert.Equal(t, uint64(math.MaxUint64), uint64(pts[0].Get("f1").(float64)))
				assert.Equal(t, int64(math.MinInt64), int64(pts[0].Get("f2").(float64)))
				assert.Equal(t, int64(123), pts[0].Get("f3"))

			} else {
				break
			}
		}

		assert.NoError(t, enc.LastErr())
	})

	t.Run("inf-pbjson", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(JSON))
		defer PutEncoder(enc)

		pt.SetFlag(Ppb)

		arr, err := enc.Encode([]*Point{pt})
		assert.NoError(t, err)

		t.Logf("res: %s", string(arr[0]))
	})
}
