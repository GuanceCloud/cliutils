// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDecode(t *testing.T) {
	var fnCalled int

	cases := []struct {
		name     string
		data     []byte
		fn       DecodeFn
		fnErr    bool
		expectLP []string
		opts     []DecoderOption
		fail     bool
	}{
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

			pts, err := dec.Decode(tc.data)
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

			t.Logf("[ok] %q", tc.data)

			assert.Equal(t, len(pts), len(tc.expectLP))
			for idx := range pts {
				assert.Equal(t, tc.expectLP[idx], pts[idx].LineProto())
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

	b.Logf("pts[0]: %s", pts[0].Pretty())
	b.Logf("pts[-1]: %s", pts[999].Pretty())

	b.Run("bench-decode-lp", func(b *testing.B) {
		enc := GetEncoder()
		defer PutEncoder(enc)

		data, _ := enc.Encode(pts)

		d := GetDecoder()
		defer PutDecoder(d)

		b.Logf("decode %d lp", len(data[0]))

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

		b.Logf("decode %d pb", len(data[0]))
		for i := 0; i < b.N; i++ {
			d.Decode(data[0])
		}
	})
}
