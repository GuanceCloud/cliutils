// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTimeRound(t *testing.T) {
	t.Run(`decode-time`, func(t *testing.T) {
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

func TestDecode(t *testing.T) {
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
			ptsOpts: []Option{WithPrecision(S)},
		},

		{
			name: "decode-json-with-precision-ms",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(MS)},
		},

		{
			name: "decode-json-with-precision-us",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(US)},
		},

		{
			name: "decode-json-with-precision-m",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 7380000000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(M)},
		},

		{
			name: "decode-json-with-precision-h",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 442800000000000`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(H)},
		},

		{
			name: "decode-json-with-precision-x",
			data: []byte(`[ { "measurement": "abc",  "tags": {"t1": "val1"}, "fields": {"f1": 123, "f2": 3.14}, "time":123} ]`),
			expectLP: []string{
				`abc,t1=val1 f1=123,f2=3.14 123`,
			},

			opts:    []DecoderOption{WithDecEncoding(JSON)},
			ptsOpts: []Option{WithPrecision(W)},
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
		t.Run(tc.name, func(t *testing.T) {
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

			assert.Equal(t, len(pts), len(tc.expectLP))
			for idx := range pts {
				assert.Equal(t, tc.expectLP[idx], pts[idx].LineProto())

				t.Logf("point: %s", pts[idx].Pretty())
			}

			if tc.fn != nil {
				assert.Equal(t, 1, fnCalled)
			}
		})
	}
}

func BenchmarkDecode(b *testing.B) {
	r := NewRander(WithFixedTags(true), WithRandText(3))
	pts := r.Rand(1000)

	b.Run("bench-decode-lp", func(b *testing.B) {
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

	b.Run("bench-decode-pb", func(b *testing.B) {
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

	b.Run("bench-decode-json", func(b *testing.B) {
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

func BenchmarkBytes2String(b *testing.B) {
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

	b.Run("bytes2str", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			y := string(raw)
			_ = y
		}
	})

	b.Run("str2bytes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			y := []byte(strData)
			_ = y
		}
	})
}
