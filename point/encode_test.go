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
	"strings"
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
			NewPoint(`abc`, NewKVs(map[string]interface{}{"f1": "fv1", "f2": "fv2", "f3": "fv3"}).
				AddTag(`t1`, `tv1`).
				AddTag(`t2`, `tv2`).
				AddTag(`t3`, `tv3`), WithTime(time.Unix(0, 123))),

			NewPoint(`def`, NewKVs(map[string]interface{}{"f1": "fv1", "f2": "fv2", "f3": "fv3"}).
				AddTag(`t1`, `tv1`).
				AddTag(`t2`, `tv2`).
				AddTag(`t3`, `tv3`), WithTime(time.Unix(0, 123))),

			NewPoint(`xyz`, NewKVs(map[string]interface{}{"f1": "fv1", "f2": "fv2", "f3": "fv3"}).
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
				x, err := NewPointDeprecated("abc", map[string]string{
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
}

func TestEscapeEncode(t *T.T) {
	t.Run("escaped-lineproto", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f1=2=3=", 3.14)
		kvs = kvs.Add("f2\tnr", 2)
		kvs = kvs.Add("f3,", "some-string\nanother-line")
		kvs = kvs.Add("f4,", false)
		kvs = kvs.Add("f\nnext-line,", []byte("hello"))
		kvs = kvs.Add(`f\other`, []byte("hello")) // nolint:misspell

		kvs = kvs.AddTag("tag=1", "value")
		kvs = kvs.AddTag("tag 2", "value")
		kvs = kvs.AddTag("tag\t3", "value")
		kvs = kvs.AddTag("tag=1", "value=1")
		kvs = kvs.AddTag("tag 2", "value 2")
		kvs = kvs.AddTag("tag\t3", "value \t3")
		kvs = kvs.AddTag("tag4", "value \n3")
		kvs = kvs.AddTag("tag5", `value \`)         // tag-value got tail \
		kvs = kvs.AddTag("tag\nnext-line", `value`) // tag key get \n

		pt := NewPoint("some,=abc\"", kvs)

		lp := pt.LineProto()
		t.Logf("line-protocol: %s", lp)
		t.Logf("pretty: %s", pt.Pretty())

		dec := GetDecoder(WithDecEncoding(LineProtocol))
		defer PutDecoder(dec)
		pts, err := dec.Decode([]byte(lp))
		assert.NoError(t, err)
		eq, why := pts[0].EqualWithReason(pt)
		assert.Truef(t, eq, "not equal: %s", why)
	})
}

func TestPBEncode(t *T.T) {
	t.Run(`invalid-utf8-string-field`, func(t *T.T) {
		var kvs KVs
		invalidUTF8 := "a\xffb\xC0\xAFc\xff"

		t.Logf("invalidUTF8: %s", invalidUTF8) // the printed invalid-utf8 seems equal to `abc'

		kvs = kvs.Add("invalid-utf8", invalidUTF8)

		pt := NewPoint("p1", kvs)

		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)
		arr, err := enc.Encode([]*Point{pt})
		require.NoError(t, err)
		assert.Len(t, arr, 1)

		require.Lenf(t, pt.pt.Warns, 0, "point: %s", pt.Pretty())

		// but the real value get from point is "a\xffb\xC0\xAFc\xff"
		assert.Equal(t, invalidUTF8, pt.Get("invalid-utf8"), "abc")

		t.Logf("pt: %s", pt.Pretty())
	})

	t.Run(`invalid-utf8-string-field`, func(t *T.T) {
		var kvs KVs

		invalidUTF8Str := "a\xffb\xC0\xAFc\xff"

		validUTF8Str := strings.ToValidUTF8(invalidUTF8Str, "0X")
		kvs = kvs.Add("invalid-utf8", validUTF8Str)

		pt := NewPoint("p1", kvs)

		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)
		arr, err := enc.Encode([]*Point{pt})
		require.NoError(t, err)
		assert.Len(t, arr, 1)
		t.Logf("pt: %s", pt.Pretty())
	})

	t.Run(`invalid-utf8-[]byte-field`, func(t *T.T) {
		var kvs KVs

		invalidUTF8Bytes := []byte("a\xffb\xC0\xAFc\xff")

		kvs = kvs.Add("invalid-utf8", invalidUTF8Bytes)

		pt := NewPoint("p1", kvs)

		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)
		arr, err := enc.Encode([]*Point{pt})
		require.NoError(t, err)
		assert.Len(t, arr, 1)

		require.Equal(t, invalidUTF8Bytes, pt.Get("invalid-utf8"))
		t.Logf("pt: %s", pt.Pretty())
	})
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
			x, err := NewPointDeprecated("abc", map[string]string{
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

func TestEncBufUsage(t *T.T) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	pts := r.Rand(10000)

	cases := []struct {
		name string
		buf  []byte
	}{
		{
			`4k`, make([]byte, 4*1024),
		},

		{
			`8k`, make([]byte, 8*1024),
		},

		{
			`64k`, make([]byte, 64*1024),
		},

		{
			`128k`, make([]byte, 128*1024),
		},

		{
			`512k`, make([]byte, 512*1024),
		},

		{
			`1m`, make([]byte, 1<<20),
		},
	}

	totalSize := 0
	for _, pt := range pts {
		pt.MustAdd("s-arr", []string{"s1", "s2"}) // anypb
		totalSize += pt.Size()
	}

	t.Logf("avg size: %d", totalSize/len(pts))

	// the larger the buf, the higher usage of buf for encoding
	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			n := 0

			usages := 0.0
			for i := 0; i < 100; i++ {
				enc := GetEncoder(WithEncEncoding(Protobuf))
				enc.EncodeV2(pts)

				for {
					if x, ok := enc.Next(tc.buf); ok {
						u := (float64(len(x)) / float64(len(tc.buf)))
						if enc.lastPtsIdx != len(pts) {
							usages += u
							n++
						}
					} else {
						break
					}
				}

				PutEncoder(enc)
			}

			t.Logf("avg usage: %.4f", usages/float64(n))
		})
	}
}

func BenchmarkEncode(b *T.B) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	pts := r.Rand(1000)

	buf := make([]byte, 1<<20)

	b.ResetTimer()
	b.Run("encode-json", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(JSON))
			enc.Encode(pts)
			PutEncoder(enc)
		}
	})

	b.ResetTimer()
	b.Run("encode-lp", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder()
			enc.Encode(pts)
			PutEncoder(enc)
		}
	})

	b.ResetTimer()
	b.Run("v1-encode-pb", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(Protobuf), WithEncBatchBytes(1<<20))
			enc.Encode(pts)
			PutEncoder(enc)
		}
	})

	b.ResetTimer()
	b.Run("v2-encode-pb-pbsize", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			enc := GetEncoder(WithEncEncoding(Protobuf), WithApproxSize(false))
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

	b.ResetTimer()
	b.Run("v2-encode-pb-approx-size", func(b *T.B) {
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

	b.ResetTimer()
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
	randPts := r.Rand(1000)

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
	t.Run("skip-huge-tail-point", func(t *T.T) {
		pts := []*Point{
			NewPoint("small", NewKVs(map[string]any{
				"f1":   123,
				"str1": strings.Repeat("x", 100),
			}), WithTimestamp(123)),

			NewPoint("huge", NewKVs(map[string]any{
				"f1":   123,
				"str1": strings.Repeat("x", 100),
				"str2": strings.Repeat("y", 200),
				"str3": strings.Repeat("z", 400),
			}), WithTimestamp(123)),
		}

		var pbpts PBPoints
		sum := 0
		for _, pt := range pts {
			sum += pt.pt.Size()
			pbpts.Arr = append(pbpts.Arr, pt.pt)
		}

		t.Logf("sum: %d, size: %d", sum, pbpts.Size())

		assert.True(t, sum < pbpts.Size())    // pb need some more bytes
		buf := make([]byte, pts[1].pt.Size()) // size only fit to huge point

		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true))
		enc.EncodeV2(pts)
		defer PutEncoder(enc)

		var (
			decodePts []*Point
			round     int
			dec       = GetDecoder(WithDecEncoding(Protobuf))
		)
		defer PutDecoder(dec)

		for {
			if x, ok := enc.Next(buf); ok {
				decPts, err := dec.Decode(x)
				assert.NoErrorf(t, err, "decode %s failed", x)

				t.Logf("encoded %d(%d remain) bytes, %d points, encoder: %s",
					len(x), (len(buf) - len(x)), len(decPts), enc.String())
				decodePts = append(decodePts, decPts...)
				round++
				assert.Equal(t, round, enc.parts)
				t.Logf("trimmed: %d", enc.LastTrimmed())
			} else {
				break
			}
		}

		t.Logf("encoder: %s", enc)

		for i, pt := range decodePts {
			assert.Equal(t, pts[i].Pretty(), pt.Pretty())
		}

		assert.Equal(t, 1, enc.parts)
		assert.Equal(t, 1, enc.SkippedPoints())
		assert.NoError(t, enc.LastErr())
	})

	t.Run("encode-huge-tail-point", func(t *T.T) {
		pts := []*Point{
			NewPoint("p1", NewKVs(map[string]any{
				"f1":   123,
				"str1": strings.Repeat("x", 100),
				"str2": strings.Repeat("y", 200),
				"str3": strings.Repeat("z", 400),
			}), WithTimestamp(123)),

			NewPoint("p2", NewKVs(map[string]any{
				"f1":   123,
				"str1": strings.Repeat("x", 100),
				"str2": strings.Repeat("y", 200),
				"str3": strings.Repeat("z", 400),
			}), WithTimestamp(123)),
		}

		var pbpts PBPoints
		sum := 0
		for _, pt := range pts {
			sum += pt.pt.Size()
			pbpts.Arr = append(pbpts.Arr, pt.pt)
		}

		t.Logf("sum: %d, size: %d", sum, pbpts.Size())

		assert.True(t, sum < pbpts.Size()) // pb need some more bytes
		buf := make([]byte, sum)

		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true))
		enc.EncodeV2(pts)
		defer PutEncoder(enc)

		var (
			decodePts []*Point
			round     int
			dec       = GetDecoder(WithDecEncoding(Protobuf))
		)
		defer PutDecoder(dec)

		for {
			if x, ok := enc.Next(buf); ok {
				decPts, err := dec.Decode(x)
				assert.NoErrorf(t, err, "decode %s failed", x)

				t.Logf("encoded %d(%d remain) bytes, %d points, encoder: %s",
					len(x), (len(buf) - len(x)), len(decPts), enc.String())
				decodePts = append(decodePts, decPts...)
				round++
				assert.Equal(t, round, enc.parts)
				t.Logf("trimmed: %d", enc.LastTrimmed())
			} else {
				t.Logf("trimmed: %d", enc.LastTrimmed())
				break
			}
		}

		assert.NoError(t, enc.LastErr())

		for i, pt := range decodePts {
			assert.Equal(t, pts[i].Pretty(), pt.Pretty())
		}
	})

	r := NewRander(WithFixedTags(true), WithRandText(3))
	randPts := r.Rand(10000)

	t.Run("encode-pb-approx-size", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		dec := GetDecoder(WithDecEncoding(Protobuf))
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
				t.Logf("trimmed: %d", enc.LastTrimmed())
			} else {
				t.Logf("trimmed: %d", enc.LastTrimmed())
				break
			}
		}

		assert.NoError(t, enc.LastErr())

		for i, pt := range decodePts {
			assert.Equal(t, randPts[i].Pretty(), pt.Pretty())
		}
	})

	t.Run("encode-pb", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf), WithApproxSize(false))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		dec := GetDecoder(WithDecEncoding(Protobuf))
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
				t.Logf("trimmed: %d", enc.LastTrimmed())
			} else {
				t.Logf("trimmed: %d", enc.LastTrimmed())
				break
			}
		}

		assert.NoError(t, enc.LastErr())

		for i, pt := range decodePts {
			assert.Equal(t, randPts[i].Pretty(), pt.Pretty())
		}
	})

	t.Run("encode-lp", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(LineProtocol), WithApproxSize(false))
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
			ok, why := randPts[i].EqualWithReason(pt)
			require.Truef(t, ok, "reason: %s", why)
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
		enc := GetEncoder(WithEncEncoding(Protobuf), WithApproxSize(false))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		buf := make([]byte, 4) // too small

		for {
			buf, ok := enc.Next(buf)
			require.False(t, ok)
			require.Nil(t, buf)
			break
		}

		t.Logf("enc: %s", enc)

		assert.Error(t, enc.LastErr())
		t.Logf("go error: %s", enc.LastErr())
	})

	t.Run("too-small-buffer-pb-and-skipped", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true), WithApproxSize(false))
		enc.EncodeV2(randPts)
		defer PutEncoder(enc)

		buf := make([]byte, 4) // too small

		for {
			buf, ok := enc.Next(buf)
			require.Nil(t, buf)
			if !ok {
				break
			}
		}

		t.Logf("enc: %s", enc)

		assert.Equal(t, len(randPts), enc.SkippedPoints())
		assert.NoError(t, enc.LastErr())
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

func TestEncNilPoint(t *T.T) {
	t.Run(`nil-point-in-array`, func(t *T.T) {
		r := NewRander(WithFixedTags(true), WithRandText(3))
		pts := r.Rand(1)

		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true))
		enc.EncodeV2(append([]*Point{nil, nil, pts[0], nil}, pts...))
		defer PutEncoder(enc)

		buf := make([]byte, 1<<20)

		x, ok := enc.Next(buf)
		assert.True(t, ok)
		assert.NotNil(t, x)
		assert.Equal(t, 2, enc.TotalPoints())
		t.Logf("enc: %s", enc)

		x, ok = enc.Next(buf)
		assert.False(t, ok)
		assert.Nil(t, x)
		assert.Equal(t, 2, enc.TotalPoints())
		t.Logf("enc: %s", enc)
	})

	t.Run(`all-nil-point`, func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true))
		enc.EncodeV2([]*Point{nil, nil, nil})
		defer PutEncoder(enc)

		x, ok := enc.Next(nil)
		assert.False(t, ok)
		assert.Nil(t, x)
		assert.Equal(t, 0, enc.TotalPoints())
		t.Logf("enc: %s", enc)

		// always fail
		x, ok = enc.Next(nil)
		assert.False(t, ok)
		assert.Nil(t, x)
		assert.Equal(t, 0, enc.TotalPoints())
		t.Logf("enc: %s", enc)
	})
}

// nolint: ineffassign
func TestEncTrim(t *T.T) {
	strTiny := strings.Repeat("x", 4)
	strSmall := strings.Repeat("x", 32)
	str1M := strings.Repeat("x", 1<<20)
	str1K := strings.Repeat("x", 1<<10)
	str32K := strings.Repeat("x", 32*(1<<10))
	str128K := strings.Repeat("x", 128*(1<<10))

	var kvsBasic KVs
	kvsBasic = kvsBasic.Set("int8", int8(1))
	kvsBasic = kvsBasic.Set("int16", int16(1))
	kvsBasic = kvsBasic.Set("int32", int32(1))
	kvsBasic = kvsBasic.Set("int64", int64(1))
	kvsBasic = kvsBasic.Set("f32", float32(1.0))
	kvsBasic = kvsBasic.Set("f64", float64(1.0))

	var kvsStr KVs
	kvsStr = kvsStr.Set("str-tiny", strTiny)
	kvsStr = kvsStr.Set("str-small", strSmall)
	kvsStr = kvsStr.Set("str-1m", str1M)
	kvsStr = kvsStr.Set("str-1k", str1K)
	kvsStr = kvsStr.Set("str-32k", str32K)
	kvsStr = kvsStr.Set("str-128k", str128K)

	var kvsBytes KVs
	kvsBytes = kvsBytes.Set("bytes-tiny", []byte(strTiny))
	kvsBytes = kvsBytes.Set("bytes-small", []byte(strSmall))
	kvsBytes = kvsBytes.Set("bytes-1m", []byte(str1M))
	kvsBytes = kvsBytes.Set("bytes-1k", []byte(str1K))
	kvsBytes = kvsBytes.Set("bytes-32k", []byte(str32K))
	kvsBytes = kvsBytes.Set("bytes-128k", []byte(str128K))

	var kvsBool KVs
	kvsBool = kvsBasic.Set("bool-yes", true)
	kvsBool = kvsBasic.Set("bool-no", false)

	var kvsLargeNum KVs
	kvsLargeNum = kvsLargeNum.Set("large-i64", int64(math.MaxInt64))
	kvsLargeNum = kvsLargeNum.Set("large-u64", uint64(math.MaxUint64))
	kvsLargeNum = kvsLargeNum.Set("large-f64", math.MaxFloat64)

	type tcase struct {
		name string
		n    int
		pbSize,
		sumSize int

		withLargeNum,
		withStr,
		withBytes,
		withBool bool
		buf []byte
	}

	newPts := func(tc *tcase) (pts []*Point) {
		for i := 0; i < tc.n; i++ {
			ptkvs := kvsBasic

			if tc.withStr {
				ptkvs = append(ptkvs, kvsStr...)
			}

			if tc.withBytes {
				ptkvs = append(ptkvs, kvsBytes...)
			}

			if tc.withBool {
				ptkvs = append(ptkvs, kvsBool...)
			}

			if tc.withLargeNum {
				ptkvs = append(ptkvs, kvsLargeNum...)
			}

			pt := NewPoint(t.Name(), ptkvs, WithPrecheck(false), WithTime(time.Now()))
			pts = append(pts, pt)
		}

		return
	}

	cases := []tcase{
		{
			name: `4`,
			n:    4,
		},

		{
			name:         `8`,
			n:            8,
			withLargeNum: true,
			withStr:      true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			pts := newPts(&tc)
			var pbpts PBPoints
			sum := 0
			for _, pt := range pts {
				sum += pt.pt.Size()
				pbpts.Arr = append(pbpts.Arr, pt.pt)
			}

			t.Logf("sum: %d, pbsize: %d", sum, pbpts.Size())

			tc.buf = make([]byte, sum/tc.n*(tc.n-1)) // size is n-1 point size sum

			enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true), WithApproxSize(false))
			enc.EncodeV2(pts)
			defer PutEncoder(enc)

			encBuf, ok := enc.Next(tc.buf)
			assert.NoError(t, enc.LastErr())
			assert.True(t, ok)
			assert.NotNil(t, encBuf)
			assert.Equal(t, 1, enc.parts)
			assert.Equal(t, 1, enc.trimmedPts)
			t.Logf("encoder: %s", enc)

			encBuf, ok = enc.Next(tc.buf) // encode last trimmed point
			assert.NoError(t, enc.LastErr())
			assert.True(t, ok)
			assert.NotNil(t, encBuf)
			assert.Equal(t, 2, enc.parts)
			t.Logf("encoder: %s", enc)

			assert.Equal(t, tc.n, enc.TotalPoints())
		})
	}

	t.Run("too-small-buf-for-single-point", func(t *T.T) {
		// If single point's size is the same as encode-buf, nothing encoded, because we need some more bytes to
		// encode them into pbpts.
		tc := tcase{
			n: 2,
		}

		pts := newPts(&tc)
		var pbpts PBPoints
		sum := 0
		for _, pt := range pts {
			sum += pt.pt.Size()
			pbpts.Arr = append(pbpts.Arr, pt.pt)
		}

		t.Logf("sum: %d, pbsize: %d", sum, pbpts.Size())

		// buf size is n-1 point size sum
		tc.buf = make([]byte, sum/tc.n*(tc.n-1))

		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true), WithApproxSize(false))
		enc.EncodeV2(pts)
		defer PutEncoder(enc)

		encBuf, ok := enc.Next(tc.buf)
		assert.NoError(t, enc.LastErr())
		assert.False(t, ok) // noting encode
		assert.Nil(t, encBuf)
		assert.Equal(t, 1, enc.SkippedPoints())
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(tc.buf)
		assert.NoError(t, enc.LastErr())
		assert.False(t, ok) // noting encode
		assert.Nil(t, encBuf)
		assert.Equal(t, 2, enc.SkippedPoints())
		t.Logf("encoder: %s", enc)
	})

	t.Run(`single-point-too-large`, func(t *T.T) {
		tc := tcase{
			name: `1`,
			n:    1,
		}

		pts := newPts(&tc)
		var pbpts PBPoints
		sum := 0
		for _, pt := range pts {
			sum += pt.pt.Size()
			pbpts.Arr = append(pbpts.Arr, pt.pt)
		}

		t.Logf("sum: %d, pbsize: %d", sum, pbpts.Size())

		// 0 bytes
		tc.buf = make([]byte, sum/tc.n*(tc.n-1))

		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true), WithApproxSize(false))
		enc.EncodeV2(pts)
		defer PutEncoder(enc)

		encBuf, ok := enc.Next(tc.buf)
		assert.NoError(t, enc.LastErr())
		assert.False(t, ok) // noting encode
		assert.Nil(t, encBuf)
		assert.Equal(t, 1, enc.SkippedPoints())
		t.Logf("encoder: %s", enc)
	})
}

func TestSkipLargePoint(t *T.T) {
	t.Run("too-small-buffer-pb-trim", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true))

		var kvs1, kvs2, kvs3, kvs4 KVs

		kvs1 = kvs1.Add("msg1", strings.Repeat("x", 70))
		kvs1 = kvs1.Add("f1", 3.14)
		kvs2 = kvs2.Add("msg2", strings.Repeat("y", 70)) // small point
		kvs2 = kvs2.Add("f1", 3.14)
		kvs3 = kvs3.Add("msg3", strings.Repeat("z", 70))
		kvs3 = kvs3.Add("f1", 3.14)
		kvs4 = kvs3.Add("msg4", strings.Repeat("0", 700)) // large point that should skip

		pt1 := NewPoint("p1", kvs1, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)

		pt2 := NewPoint("p2", kvs2, append(DefaultLoggingOptions(),
			WithTimestamp(time.Now().Unix()),
			WithPrecheck(false))...)

		pt3 := NewPoint("p3", kvs3, append(DefaultLoggingOptions(),
			WithTimestamp(time.Now().Unix()),
			WithPrecheck(false))...)

		// p3 larger than encode buf
		pt4 := NewPoint("p4", kvs4, append(DefaultLoggingOptions(), // small
			WithTimestamp(time.Now().Unix()),
			WithPrecheck(false))...)

		enc.EncodeV2([]*Point{pt1, pt2, pt3, pt4})
		defer PutEncoder(enc)

		// 172 used to accept 2 point
		buf := make([]byte, 206)

		encBuf, ok := enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p1 encode this time
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		assert.Equal(t, 1, enc.parts)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p2 encoded, and p3 should trimmed
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		assert.Equal(t, 2, enc.parts)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf) // p3 encode again, and p4 skipped
		assert.NoError(t, enc.LastErr())
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.False(t, ok)
		assert.Nil(t, encBuf)
		assert.NoError(t, enc.LastErr())
		t.Logf("encoder: %s", enc)
	})

	t.Run("pb-2-large-point", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf))

		var (
			kvs1 KVs
			kvs2 KVs
			kvs3 KVs
		)

		kvs1 = kvs1.Add("msg1", strings.Repeat("x", 70))
		kvs2 = kvs2.Add("msg2", strings.Repeat("y", 70))
		kvs3 = kvs3.Add("msg3", strings.Repeat("z", 100))
		pt1 := NewPoint("p1", kvs1, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)
		pt2 := NewPoint("p2", kvs2, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)

		// p3 larger than encode buf
		pt3 := NewPoint("p3", kvs3, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)

		enc.EncodeV2([]*Point{pt1, pt2, pt3})
		defer PutEncoder(enc)

		buf := make([]byte, 90)

		encBuf, ok := enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p1 encode this time
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		assert.Equal(t, 1, enc.parts)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p2 encoded
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		assert.Equal(t, 2, enc.parts)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.Error(t, enc.LastErr()) // p3 not encoded, under ignoreLargePoint = false, got error
		t.Logf("[expected] %s", enc.LastErr())
		assert.False(t, ok)
		assert.Nil(t, encBuf)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.False(t, ok)
		assert.Nil(t, encBuf)
		assert.Error(t, enc.LastErr()) // still error once encode failed once
	})

	t.Run("too-small-buffer-pb-skip-huge-point", func(t *T.T) {
		enc := GetEncoder(WithEncEncoding(Protobuf), WithIgnoreLargePoint(true))

		var (
			kvs1 KVs
			kvs2 KVs
			kvs3 KVs
		)

		kvs1 = kvs1.Add("msg1", strings.Repeat("x", 70))
		kvs2 = kvs2.Add("msg2", strings.Repeat("y", 70))
		kvs3 = kvs3.Add("msg3", strings.Repeat("z", 100))
		pt1 := NewPoint("p1", kvs1, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)
		pt2 := NewPoint("p2", kvs2, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)

		// p3 larger than encode buf
		pt3 := NewPoint("p3", kvs3, append(DefaultLoggingOptions(),
			WithTimestamp(123),
			WithPrecheck(false))...)

		pt4 := NewPoint("p4", kvs1, append(DefaultLoggingOptions(), // small
			WithTimestamp(123),
			WithPrecheck(false))...)

		enc.EncodeV2([]*Point{pt1, pt2, pt3, pt4})
		defer PutEncoder(enc)

		buf := make([]byte, 90)

		encBuf, ok := enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p1 encode this time
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		assert.Equal(t, 1, enc.parts)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p2 encoded
		assert.True(t, ok)
		assert.NotNil(t, encBuf)
		assert.Equal(t, 2, enc.parts)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.NoError(t, enc.LastErr()) // p3 skipped

		assert.True(t, ok) // and p4 encoded
		assert.NotNil(t, encBuf)
		t.Logf("encoder: %s", enc)

		encBuf, ok = enc.Next(buf)
		assert.False(t, ok)
		assert.Nil(t, encBuf)
		assert.NoError(t, enc.LastErr())
		t.Logf("encoder: %s", enc)
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
	r := NewRander(WithFixedTags(true), WithRandText(1))

	for i := 0; i < 10; i++ {
		pt := r.Rand(1)[0]
		pt.MustAdd("s-arr", []string{"s1", "s2"})

		t.Logf("pt: %s", pt.Pretty())

		pbsize, rawSize := pt.pt.Size(), pt.Size()
		t.Logf("pt.pt.size: %d, pt.size: %d, diff: %.2f", pbsize, rawSize, float64(pbsize)/float64(rawSize))
	}
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
	kvs = kvs.Set("f1", math.Inf(1))
	kvs = kvs.Set("f2", math.Inf(-1))
	kvs = kvs.Set("f3", 123)

	pt := NewPoint("some", kvs)

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

				assert.Equal(t, uint64(math.MaxUint64), safeFloat64ToUint64(pts[0].Get("f1").(float64)))
				assert.Equal(t, int64(math.MinInt64), safeFloat64ToInt64(pts[0].Get("f2").(float64)))
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

// safeFloat64ToUint64 converts a float64 to uint64, handling out-of-range values
// by saturating to the min or max of uint64.
func safeFloat64ToUint64(f float64) uint64 {
	// Handle NaN and negative numbers
	if math.IsNaN(f) || f <= 0 {
		return 0
	}
	// Handle values larger than MaxUint64
	if f >= float64(math.MaxUint64) {
		return math.MaxUint64
	}
	return uint64(f)
}

// safeFloat64ToInt64 converts a float64 to int64, handling out-of-range values
// by saturating to the min or max of int64.
func safeFloat64ToInt64(f float64) int64 {
	// Handle NaN
	if math.IsNaN(f) {
		return 0
	}
	// Handle values larger than MaxInt64
	if f >= float64(math.MaxInt64) {
		return math.MaxInt64
	}
	// Handle values smaller than MinInt64
	if f <= float64(math.MinInt64) {
		return math.MinInt64
	}
	return int64(f)
}
