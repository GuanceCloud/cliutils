// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEqual(t *T.T) {
	DefaultEncoding = Protobuf // set protobuf the default encoding

	cases := []struct {
		name        string
		l, r        *Point
		expectEqual bool
	}{
		{
			name:        "basic",
			expectEqual: true,
			l: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "time-not-equal",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(1, 0)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "with-warns",
			expectEqual: true,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{
						"f1": 123.1,
						"t1": "duplicated-tag-key", // duplicated key
					},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "tags-not-equal",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1", "t2": "v2"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "measurement-name-not-equal",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("def",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "field-value-type-not-match",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": "foo"},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "field-key-not-match",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f2": "foo"},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "field-count-not-match",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1, "f2": "haha"},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f2": "foo"},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "tag-count-not-match",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1", "t2": "v2"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "tag-key-not-match",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t2": "v2"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "tag-value-not-match",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "vx"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "field-value-match",
			expectEqual: true,
			l: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{
						"f1":           int64(123),
						"f2_f64":       123.01234567890123456789,
						"f2_large_f64": 1234567890123456789.01234567890123456789, // -> 1234567890123456800
						"f2_f32":       float32(123.4),
						"f2_f32_long":  float32(123.1234567890), // -> 123.12346
						"f3":           false,
						"f4":           "abc",
						"f5":           []byte("xyz"),
						"f6":           uint64(1234567890),
					},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{
						"f1":           int64(123),
						"f2_f64":       123.01234567890123456789,
						"f2_large_f64": 1234567890123456789.01234567890123456789,
						"f2_f32":       float32(123.4000),
						"f2_f32_long":  float32(123.1234567890),
						"f3":           false,
						"f4":           "abc",
						"f5":           []byte("xyz"),
						"f6":           uint64(1234567890),
					},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			eq, reason := tc.l.EqualWithReason(tc.r)
			assert.Equal(t, tc.expectEqual, eq, "l: %s\nr: %s", tc.l.Pretty(), tc.r.Pretty())

			if reason != "" {
				t.Logf("reason: %s", reason)
			}

			t.Logf("pt %s", tc.l.Pretty())
		})
	}

	t.Cleanup(func() {
		// reset them
		DefaultEncoding = LineProtocol
	})
}

func TestHash(t *T.T) {
	cases := []struct {
		name        string
		l, r        *Point
		expectEqual bool
	}{
		{
			name:        "diff-fields",
			expectEqual: true,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f2": 123},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "diff-time",
			expectEqual: true,
			l: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f2": 123},
					WithTime(time.Unix(0, 456)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "diff-measurement",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("def",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f2": 123},
					WithTime(time.Unix(0, 456)))
				assert.NoError(t, err)
				return x
			}(),
		},

		{
			name:        "diff-tags",
			expectEqual: false,
			l: func() *Point {
				x, err := NewPoint("def",
					map[string]string{"t2": "v1"},
					map[string]interface{}{"f1": 123.1},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return x
			}(),
			r: func() *Point {
				x, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f2": 123},
					WithTime(time.Unix(0, 456)))
				assert.NoError(t, err)
				return x
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			if tc.expectEqual {
				assert.Equal(t, tc.l.MD5(), tc.r.MD5())
				assert.Equal(t, tc.l.Sha256(), tc.r.Sha256())
			} else {
				assert.NotEqual(t, tc.l.MD5(), tc.r.MD5())
				assert.NotEqual(t, tc.l.Sha256(), tc.r.Sha256())
			}
		})
	}
}
