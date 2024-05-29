// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"bytes"
	"fmt"
	"strings"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLPFieldArray(t *T.T) {
	t.Run(`decode-lp-array-field`, func(t *T.T) {
		lp := []byte(`a,t1=v1 arr=["s1","s2"] 123`)

		dec := GetDecoder(WithDecEncoding(LineProtocol))
		_, err := dec.Decode(lp)
		assert.NoErrorf(t, err, "should got no error: %s", err)
	})

	t.Run(`encode-lp-array-field`, func(t *T.T) {
		pt := emptyPoint()

		pt.SetName("abc")
		pt.SetTime(time.Unix(0, 123))

		pt.AddKVs(NewKV("i-arr", []int{1, 2, 3}))
		pt.AddKVs(NewKV("s-arr", []string{"hello", "world"}))
		pt.AddKVs(NewKV("f-arr", []float64{3.14, 6.18}))
		pt.AddKVs(NewKV("b-arr", []bool{false, true}))

		t.Logf("lp: %s", pt.LineProto())
	})

	t.Run(`encode-json-array-field`, func(t *T.T) {
		pt := emptyPoint()

		pt.SetName("abc")
		pt.SetTime(time.Unix(0, 123))

		pt.AddKVs(NewKV("i-arr", []int{1, 2, 3}))
		pt.AddKVs(NewKV("s-arr", []string{"hello", "world"}))
		pt.AddKVs(NewKV("f-arr", []float64{3.14, 6.18}))
		pt.AddKVs(NewKV("b-arr", []bool{false, true}))

		enc := GetEncoder(WithEncEncoding(JSON))
		defer PutEncoder(enc)

		arr, err := enc.Encode([]*Point{pt})
		assert.NoError(t, err)

		t.Logf("lp json: %s", string(arr[0]))
	})

	t.Run(`decode-json-array-field`, func(t *T.T) {
		j := `[{"measurement":"abc","fields":{"b-arr":[false,true],"f-arr":[3.14,6.18],"i-arr":[1,2,3],"s-arr":["hello","world"]},"time":123}]`
		dec := GetDecoder(WithDecEncoding(JSON))
		defer PutDecoder(dec)

		pts, err := dec.Decode([]byte(j))
		assert.NoError(t, err)

		for _, pt := range pts {
			t.Logf("json pt: %s", pt.LineProto())
		}
	})
}

func TestTimeRound(t *T.T) {
	t.Run(`decode-time`, func(t *T.T) {
		pt := NewPointV2("some", nil, WithTime(time.Now()))
		enc := GetEncoder(WithEncEncoding(Protobuf))
		data, err := enc.Encode([]*Point{pt})
		assert.NoError(t, err)

		dec := GetDecoder(WithDecEncoding(Protobuf))
		pts, err := dec.Decode(data[0])
		assert.NoError(t, err)

		assert.Equal(t, pt.Pretty(), pts[0].Pretty())
	})
}

func TestDynamicPrecision(t *T.T) {
	pts := []*Point{
		func() *Point {
			var kvs KVs
			kvs = kvs.AddV2("f1", 123, true)
			return NewPointV2("p1", kvs, WithTimestamp(1716536956))
		}(),

		func() *Point {
			var kvs KVs
			kvs = kvs.AddV2("f1", 123, true)
			return NewPointV2("p1", kvs, WithTimestamp(1716536956000))
		}(),

		func() *Point {
			var kvs KVs
			kvs = kvs.AddV2("f1", 123, true)
			return NewPointV2("p1", kvs, WithTimestamp(1716536956000000))
		}(),

		func() *Point {
			var kvs KVs
			kvs = kvs.AddV2("f1", 123, true)
			return NewPointV2("p1", kvs, WithTimestamp(1716536956000000000))
		}(),
	}

	cases := []struct {
		name string
		e    Encoding
	}{
		{
			"line-protocol",
			LineProtocol,
		},

		//{
		//	"json",
		//	JSON,
		//},

		//{
		//	"pbjson",
		//	PBJSON,
		//},

		{
			"pb",
			Protobuf,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			enc := GetEncoder(WithEncEncoding(tc.e))
			defer PutEncoder(enc)

			enc.EncodeV2(pts)
			buf := make([]byte, 1<<20) // large buffer
			var (
				encBuf []byte
				ok     bool
			)

			encBuf, ok = enc.Next(buf) // encode once we should get all the buffer
			assert.True(t, ok, "enc last error: %s", enc.LastErr())

			dec := GetDecoder(WithDecEncoding(tc.e))
			defer PutDecoder(dec)

			newPts, err := dec.Decode(encBuf, WithPrecision(PrecDyn))
			assert.NoError(t, err)
			for _, pt := range newPts {
				assert.Equal(t, int64(1716536956000000000), pt.pt.Time)
			}
		})
	}
}

func TestDecode(t *T.T) {
	var fnCalled int

	cases := []struct {
		name     string
		data     []byte
		fn       DecodeFn
		fnErr    bool
		expectLP []string

		opts    []DecoderOption
		ptsOpts []Option

		fail bool
	}{
		{
			name: "decode-json",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123`,
			},

			opts: []DecoderOption{WithDecEncoding(JSON)},
		},

		{
			name: "decode-json-with-precision-s",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123000000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(PrecS)},
		},

		{
			name: "decode-json-with-precision-ms",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(PrecMS)},
		},

		{
			name: "decode-json-with-precision-us",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(PrecUS)},
		},

		{
			name: "decode-json-with-precision-m",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 7380000000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(PrecM)},
		},

		{
			name: "decode-json-with-precision-h",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 442800000000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(PrecH)},
		},

		{
			name: "decode-json-with-precision-x",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(PrecW)},
		},

		{
			name: "decode-metric-json",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14, "f-str": "hello"}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123`,
			},
			ptsOpts: DefaultMetricOptions(),

			opts: []DecoderOption{WithDecEncoding(JSON)},
		},

		{
			name:     "lp",
			data:     []byte(`abc,tag1=v1,tag2=v2 f1=1i,f2=2 123`),
			expectLP: []string{`abc,tag1=v1,tag2=v2 f1=1i,f2=2 123`},
		},

		{
			fail: true,
			name: "invalid-lp",
			data: []byte(`abc,tag1=v1,tag2=v2 f1=1i,f2=2 123,`),
		},

		{
			name: "pb",
			data: func() []byte {
				pt, err := NewPoint("abc",
					map[string]string{"tag1": "v1", "tag2": "v2"},
					map[string]interface{}{"f1": 1, "f2": 2.0},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)

				enc := GetEncoder(WithEncEncoding(Protobuf))
				defer PutEncoder(enc)

				data, err := enc.Encode([]*Point{pt})
				assert.NoError(t, err)
				assert.Equal(t, 1, len(data))
				return data[0]
			}(),

			opts:     []DecoderOption{WithDecEncoding(Protobuf)},
			expectLP: []string{`abc,tag1=v1,tag2=v2 f1=1i,f2=2 123`},
		},

		{
			fail: true,
			name: "invalid-pb",
			data: func() []byte {
				pt, err := NewPoint("abc",
					map[string]string{"tag1": "v1", "tag2": "v2"},
					map[string]interface{}{"f1": 1, "f2": 2.0},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)

				enc := GetEncoder(WithEncEncoding(Protobuf))
				defer PutEncoder(enc)

				data, err := enc.Encode([]*Point{pt})
				assert.NoError(t, err)
				assert.Equal(t, 1, len(data))
				return data[0][:len(data[0])/2] // half of pb
			}(),
			opts: []DecoderOption{WithDecEncoding(Protobuf)},
		},

		{
			name: "pb-with-fn",
			data: func() []byte {
				pt, err := NewPoint("abc",
					map[string]string{"tag1": "v1", "tag2": "v2"},
					map[string]interface{}{"f1": 1, "f2": 2.0},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)

				enc := GetEncoder(WithEncEncoding(Protobuf))
				defer PutEncoder(enc)

				data, err := enc.Encode([]*Point{pt})
				assert.NoError(t, err)
				assert.Equal(t, 1, len(data))
				return data[0]
			}(),

			fn: func(pts []*Point) error {
				t.Logf("get %d point", len(pts))
				fnCalled++
				return nil
			},

			opts: []DecoderOption{WithDecEncoding(Protobuf)},

			expectLP: []string{`abc,tag1=v1,tag2=v2 f1=1i,f2=2 123`},
		},

		{
			name: "pb-with-fn-on-error",
			data: func() []byte {
				pt, err := NewPoint("abc",
					map[string]string{"tag1": "v1", "tag2": "v2"},
					map[string]interface{}{"f1": 1, "f2": 2.0},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)

				enc := GetEncoder(WithEncEncoding(Protobuf))
				defer PutEncoder(enc)

				data, err := enc.Encode([]*Point{pt})
				assert.NoError(t, err)
				assert.Equal(t, 1, len(data))
				return data[0]
			}(),

			fn: func(pts []*Point) error {
				fnCalled++
				return fmt.Errorf("mocked error")
			},
			fnErr: true,

			opts:     []DecoderOption{WithDecEncoding(Protobuf)},
			expectLP: []string{`abc,tag1=v1,tag2=v2 f1=1i,f2=2 123`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			fnCalled = 0 // reset

			opts := []DecoderOption{WithDecFn(tc.fn)}
			opts = append(opts, tc.opts...)

			dec := GetDecoder(opts...)
			defer PutDecoder(dec)

			pts, err := dec.Decode(tc.data, tc.ptsOpts...)
			if tc.fail {
				assert.Error(t, err, "decode %s got pts: %+#v", tc.data, pts)
				t.Logf("expect error: %s", err)
				return
			}

			if tc.fnErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, len(tc.expectLP), len(pts))
			for idx := range pts {
				assert.Equal(t, tc.expectLP[idx], pts[idx].LineProto())

				t.Logf("point: %s", pts[idx].Pretty())
			}

			if tc.fn != nil {
				assert.Equal(t, 1, fnCalled)
			}
		})
	}

	t.Run("decode-pb-json", func(t *T.T) {
		j := `[
{"name":"abc","fields":[{"key":"f1","i":"123"},{"key":"f2","b":false},{"key":"t1","s":"tv1","is_tag":true},{"key":"t2","s":"tv2","is_tag":true}],"time":"123"}
]`
		dec := GetDecoder(WithDecEncoding(JSON))
		defer PutDecoder(dec)
		pts, err := dec.Decode([]byte(j), DefaultLoggingOptions()...)
		require.NoError(t, err)

		for _, pt := range pts {
			assert.Equal(t, "unknown", pt.Get("status").(string))

			t.Logf("pt: %s", pt.Pretty())
		}
	})

	t.Run("decode-bytes-array", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f_d_arr", MustNewAnyArray([]byte("hello"), []byte("world")), false, false)
		pt := NewPointV2("m1", kvs)
		enc := GetEncoder(WithEncEncoding(LineProtocol))
		defer PutEncoder(enc)
		arr, err := enc.Encode([]*Point{pt})
		assert.NoError(t, err)

		t.Logf("lp: %s", arr[0])

		dec := GetDecoder(WithDecEncoding(LineProtocol))
		defer PutDecoder(dec)
		pts, err := dec.Decode(arr[0])
		assert.NoError(t, err)
		for _, pt := range pts {
			t.Logf("pt: %s", pt.Pretty())
		}
	})

	t.Run("decode-with-check", func(t *T.T) {
		var kvs KVs
		kvs = kvs.AddV2("f.1", 1.23, false) // f.1 rename to f_1 and key conflict
		kvs = kvs.AddV2("f_1", 321, false)
		kvs = kvs.AddV2("tag.1", "some-val", false, WithKVTagSet(true))

		pt := NewPointV2("m1", kvs, WithTime(time.Unix(0, 123)))

		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)
		enc.EncodeV2([]*Point{pt})

		src := make([]byte, 1<<20)
		src, ok := enc.Next(src)
		assert.True(t, ok)

		dec := GetDecoder(WithDecEncoding(Protobuf))

		pts, err := dec.Decode(src, WithDotInKey(false)) // disable dot(.) in key
		assert.NoError(t, err)
		assert.Len(t, pts, 1)

		assert.Equal(t, int64(321), pts[0].Get("f_1").(int64))
		assert.Equal(t, "some-val", pts[0].Get("tag_1").(string))

		assert.Len(t, pts[0].pt.Warns, 3)

		t.Logf("pt: %s", pts[0].Pretty())
		t.Logf("pt: %s", pts[0].LineProto())
		defer PutDecoder(dec)

		// test on easyproto
		dec = GetDecoder(WithDecEncoding(Protobuf), WithDecEasyproto(true))
		pts, err = dec.Decode(src, WithDotInKey(false)) // disable dot(.) in key
		assert.NoError(t, err)
		assert.Len(t, pts, 1)

		assert.Equal(t, int64(321), pts[0].Get("f_1").(int64))
		assert.Equal(t, "some-val", pts[0].Get("tag_1").(string))

		assert.Len(t, pts[0].pt.Warns, 3)

		t.Logf("pt: %s", pts[0].Pretty())
		t.Logf("pt: %s", pts[0].LineProto())
		defer PutDecoder(dec)
	})
}

func BenchmarkDecode(b *T.B) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	pts := r.Rand(1000)

	b.Run("bench-decode-lp", func(b *T.B) {
		enc := GetEncoder()
		defer PutEncoder(enc)

		data, _ := enc.Encode(pts)

		d := GetDecoder(WithDecEncoding(LineProtocol))
		defer PutDecoder(d)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d.Decode(data[0])
		}
	})

	b.Run("bench-decode-pb", func(b *T.B) {
		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)

		data, _ := enc.Encode(pts)

		d := GetDecoder(WithDecEncoding(Protobuf))
		defer PutDecoder(d)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d.Decode(data[0])
		}
	})

	b.Run("bench-decode-json", func(b *T.B) {
		enc := GetEncoder(WithEncEncoding(JSON))
		defer PutEncoder(enc)

		data, _ := enc.Encode(pts)

		d := GetDecoder(WithDecEncoding(JSON))
		defer PutDecoder(d)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			d.Decode(data[0])
		}
	})
}

func BenchmarkBytes2String(b *T.B) {
	repeat := 1
	raw := []byte("xxxxxxxxxxxxxxxx")

	bytesData := bytes.Repeat(raw, repeat)
	strData := strings.Repeat(string(raw), repeat)

	str := string(raw)
	b.Logf("str:   %p", &str)
	b.Logf("bytes: %p", raw)
	b.Logf("repeat: %p", &repeat)

	b.Logf("str:   %p", &strData)
	b.Logf("bytes: %p", []byte(strData))

	{
		y := string(bytesData)
		_ = y
		b.Errorf("y: %p, d: %p", &y, bytesData)
	}

	{
		b.Errorf("y: %p, d: %p", []byte(strData), &strData)
	}

	b.Run("bytes2str", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			y := string(raw)
			_ = y
		}
	})

	b.Run("str2bytes", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			y := []byte(strData)
			_ = y
		}
	})
}
