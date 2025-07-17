// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	proto "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
)

var name = "abc"

func BenchmarkMarshal(b *testing.B) {
	cases := []struct {
		name   string
		repeat int
	}{
		{
			name:   "10-points",
			repeat: 10,
		},

		{
			name:   "1000-points",
			repeat: 1000,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name+"_pb-marshal", func(b *testing.B) {
			pbpts := RandPBPoints(tc.repeat)

			for i := 0; i < b.N; i++ {
				if _, err := proto.Marshal(pbpts); err != nil {
					b.Error(err)
				}
			}
		})

		b.Run(tc.name+"_lp-marshal", func(b *testing.B) {
			pts := RandPoints(tc.repeat)
			for i := 0; i < b.N; i++ {
				arr := []string{}
				for i := 0; i < len(pts); i++ {
					arr = append(arr, pts[i].LineProto())
				}
				x := strings.Join(arr, "\n")
				_ = x
			}
		})

		b.Logf("----------------------------------------------")

		b.Run(tc.name+"_pb-unmarshal", func(b *testing.B) {
			pbpts := RandPBPoints(tc.repeat)

			pb, err := proto.Marshal(pbpts)
			assert.NoError(b, err)

			for i := 0; i < b.N; i++ {
				var pt PBPoints
				if err := proto.Unmarshal(pb, &pt); err != nil {
					b.Error(err)
				}
			}
		})

		b.Run(tc.name+"_lp-unmarshal", func(b *testing.B) {
			pts := RandPoints(tc.repeat)
			arr := []string{}
			for i := 0; i < len(pts); i++ {
				arr = append(arr, pts[i].LineProto())
			}
			ptbytes := []byte(strings.Join(arr, "\n"))

			for i := 0; i < b.N; i++ {
				if _, err := parseLPPoints(ptbytes, nil); err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func TestPBPointJSON(t *testing.T) {
	cases := []struct {
		name   string
		tags   map[string]string
		fields map[string]interface{}
		time   time.Time
		warns  []*Warn
		debugs []*Debug
	}{
		{
			name: "simple",
			tags: nil,
			fields: map[string]interface{}{
				"f1": int64(123),
				"f2": 123.4,
				"f3": false,
				"f4": "abc",
				"f5": []byte("xyz"),
				"f6": uint64(1234567890),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "simple-2",
			tags: map[string]string{
				"t1": "123",
				"t2": "xyz",
			},
			fields: map[string]interface{}{
				"f1": int64(123),
				"f2": 123.4,
				"f3": false,
				"f4": "abc",
				"f5": []byte("xyz"),
				"f6": uint64(1234567890),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "with-warnings",
			tags: map[string]string{
				"t1": "123",
			},
			fields: map[string]interface{}{
				"t1": "dulicated key in tags", // triger warnning
				"f1": int64(123),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "with-debugs",
			tags: map[string]string{
				"t1": "123",
			},
			fields: map[string]interface{}{
				"t1": "dulicated key in tags",
				"f1": int64(123),
			},
			time:   time.Unix(0, 123),
			debugs: []*Debug{{Info: "some debug info"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pt, err := NewPoint(tc.name, tc.tags, tc.fields, WithEncoding(Protobuf), WithTime(tc.time), WithKeySorted(true))

			assert.NoError(t, err)

			for _, d := range tc.debugs {
				pt.AddDebug(d)
			}

			// json
			pbjson, err := json.Marshal(pt)
			assert.NoError(t, err)

			// test if debug/warns included in json
			if len(pt.pt.Debugs) > 0 {
				assert.Contains(t, string(pbjson), "debugs", "%s not include `debugs'", string(pbjson))
			}

			if len(pt.pt.Warns) > 0 {
				assert.Contains(t, string(pbjson), "warns", "%s not include `warns'", string(pbjson))
			}

			// unmarshal
			var unmarshalPt Point
			assert.NoError(t, json.Unmarshal(pbjson, &unmarshalPt))

			// json unmarshaled point should equal to origin point
			assert.True(t, pt.Equal(&unmarshalPt))

			// encode to pb
			enc := GetEncoder(WithEncEncoding(Protobuf))
			defer PutEncoder(enc)

			batches, err := enc.Encode([]*Point{&unmarshalPt})
			assert.NoError(t, err)
			assert.Equal(t, 1, len(batches))

			// decode the pb
			// test equality: decoded pb point equal(json format) the point before encode
			dec := GetDecoder(WithDecEncoding(Protobuf))

			pts, err := dec.Decode(batches[0])
			assert.NoError(t, err)
			assert.Equal(t, 1, len(pts))

			// test equality on origin json
			assert.Equal(t, string(pbjson),
				func() string {
					j, err := json.Marshal(pts[0])
					assert.NoError(t, err)
					return string(j)
				}())

			t.Logf("pb json after:\n%s", pts[0].Pretty())

			if len(pt.pt.Warns) > 0 {
				assert.Contains(t, string(pbjson), "warns")
			}

			if len(tc.debugs) > 0 {
				assert.Contains(t, string(pbjson), "debugs")
			}
		})
	}
}

func TestPBPointPayload(t *testing.T) {
	cases := []struct {
		name   string
		repeat int
	}{
		{
			name:   "1-point",
			repeat: 1,
		},

		{
			name:   "2-point",
			repeat: 2,
		},

		{
			name:   "4-point",
			repeat: 4,
		},

		{
			name:   "8-point",
			repeat: 8,
		},

		{
			name:   "100-point",
			repeat: 100,
		},

		{
			name:   "1000-point",
			repeat: 1000,
		},

		{
			name:   "10000-point",
			repeat: 10000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lppts := RandPoints(tc.repeat)
			pbpts := RandPBPoints(tc.repeat)

			totalPtsSize := 0
			for _, pt := range pbpts.Arr {
				totalPtsSize += pt.Size()
			}

			pb, err := proto.Marshal(pbpts)
			assert.NoError(t, err)
			t.Logf("pb len: %d, pts size: %d, diff: %d", len(pb), totalPtsSize, len(pb)-totalPtsSize)

			ratio, size := func() (float64, int) {
				x, err := cliutils.GZip(pb)
				assert.NoError(t, err)
				return float64(len(x)) / float64(len(pb)), len(x)
			}()
			t.Logf("pb gz ratio: %f/%d", ratio, size)

			// line-protocol
			ptStrArr := []string{}
			for i := 0; i < tc.repeat; i++ {
				str := lppts[i].LineProto()

				ptStrArr = append(ptStrArr, str)
			}

			ptstr := strings.Join(ptStrArr, "\n")
			ratio, size = func() (float64, int) {
				x, err := cliutils.GZipStr(ptstr)
				assert.NoError(t, err)
				return float64(len(x)) / float64(len(ptstr)), len(x)
			}()
			t.Logf("lp len: %d", len(ptstr))
			t.Logf("lp gz ratio: %f/%d", ratio, size)
		})
	}
}

// nolint:ineffassign
func TestPBPointPayloadSize(t *testing.T) {
	type tcase struct {
		name string
		n    int

		withLargeNum,
		withStr,
		withBytes,
		withBool bool
	}

	cases := []tcase{
		{name: "1-point", n: 1},
		{name: "2-point", n: 2},
		{name: "4-point", n: 4},
		{name: "8-point", n: 8},
		{name: "100-point", n: 100},
		{name: "1000-point", n: 1000},
		{name: "10000-point", n: 10000},

		{withStr: true, name: "with-str-1-point", n: 1},
		{withStr: true, name: "with-str-2-point", n: 2},
		{withStr: true, name: "with-str-4-point", n: 4},
		{withStr: true, name: "with-str-8-point", n: 8},
		{withStr: true, name: "with-str-100-point", n: 100},
		{withStr: true, name: "with-str-1000-point", n: 1000},
		{withStr: true, name: "with-str-10000-point", n: 10000},

		{withBytes: true, name: "with-bytes-1-point", n: 1},
		{withBytes: true, name: "with-bytes-2-point", n: 2},
		{withBytes: true, name: "with-bytes-4-point", n: 4},
		{withBytes: true, name: "with-bytes-8-point", n: 8},
		{withBytes: true, name: "with-bytes-100-point", n: 100},
		{withBytes: true, name: "with-bytes-1000-point", n: 1000},
		{withBytes: true, name: "with-bytes-10000-point", n: 10000},

		{withBool: true, name: "with-bool-1-point", n: 1},
		{withBool: true, name: "with-bool-2-point", n: 2},
		{withBool: true, name: "with-bool-4-point", n: 4},
		{withBool: true, name: "with-bool-8-point", n: 8},
		{withBool: true, name: "with-bool-100-point", n: 100},
		{withBool: true, name: "with-bool-1000-point", n: 1000},
		{withBool: true, name: "with-bool-10000-point", n: 10000},

		{withLargeNum: true, name: "with-large-num-1-point", n: 1},
		{withLargeNum: true, name: "with-large-num-2-point", n: 2},
		{withLargeNum: true, name: "with-large-num-4-point", n: 4},
		{withLargeNum: true, name: "with-large-num-8-point", n: 8},
		{withLargeNum: true, name: "with-large-num-100-point", n: 100},
		{withLargeNum: true, name: "with-large-num-1000-point", n: 1000},
		{withLargeNum: true, name: "with-large-num-10000-point", n: 10000},
	}

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

	newPts := func(tc *tcase) (pbpts PBPoints) {
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

			pbpts.Arr = append(pbpts.Arr, NewPointV2(t.Name(), ptkvs, WithPrecheck(false), WithTime(time.Now())).pt)
		}
		return
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			pts := newPts(&tc)
			ptSizeSum := 0
			for _, pt := range pts.Arr {
				ptSizeSum += pt.Size()
			}
			t.Logf("pts size: %d, sum size: %d, diff_per_pt: %d", pts.Size(), ptSizeSum, (pts.Size()-ptSizeSum)/tc.n)
		})
	}

	t.Run("timestamp", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f1", 123)
		pt := NewPointV2(t.Name(), kvs, WithPrecheck(false), WithTimestamp(0))
		t.Logf("ts = 0/size: %d", pt.pt.Size())

		pt = NewPointV2(t.Name(), kvs, WithPrecheck(false), WithTimestamp(123))
		t.Logf("ts = 123/size: %d", pt.pt.Size())

		pt = NewPointV2(t.Name(), kvs, WithPrecheck(false), WithTimestamp(time.Now().UnixNano()))
		t.Logf("ts = now/size: %d", pt.pt.Size())
	})
}
