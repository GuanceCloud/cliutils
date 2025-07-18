// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"
	"math"
	"sort"
	"testing"
	T "testing"

	"github.com/GuanceCloud/cliutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckMeasurement(t *testing.T) {
	cases := []struct {
		name,
		measurement,
		expect string
		opts []Option
	}{
		{
			name:        "n-len",
			measurement: "abc-def",
			opts: []Option{
				WithMaxMeasurementLen(3),
			},
			expect: "abc",
		},

		{
			name:        "no-limit",
			measurement: "abc-def",
			expect:      "abc-def",
		},

		{
			name:        "empty-measurement",
			measurement: "",
			expect:      DefaultMeasurementName,
		},

		{
			name:        "empty-measurement-trim",
			measurement: "",
			opts: []Option{
				WithMaxMeasurementLen(3),
			},
			expect: DefaultMeasurementName[:3],
		},

		{
			name:        "test-utf8-measurement",
			measurement: "中文👍",
			expect:      "中文👍",
		},

		{
			name:        "test-utf8-measurement-trim",
			measurement: "中文👍",
			opts: []Option{
				WithMaxMeasurementLen(3),
			},
			expect: string([]byte("中文👍")[:3]),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			cfg := GetCfg()
			defer PutCfg(cfg)
			for _, opt := range tc.opts {
				opt(cfg)
			}

			c := checker{cfg: cfg}
			m := c.checkMeasurement(tc.measurement)
			assert.Equal(t, tc.expect, m)
		})
	}
}

func TestCheckPoints(t *T.T) {
	t.Run("string", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f1", 1.23)
		kvs = kvs.Add("str", "hello")
		kvs = kvs.Add("u64", uint64(math.MaxUint64))

		pt := NewPoint("m1", kvs, WithPrecheck(false))
		pts := CheckPoints([]*Point{pt}, WithStrField(false))
		assert.Len(t, pts, 1)
		assert.Nil(t, pts[0].Get("str"))
		assert.Equal(t, 1.23, pts[0].Get("f1"))
	})

	t.Run("u64", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f1", 1.23)
		kvs = kvs.Add("str", "hello")
		kvs = kvs.Add("u64", uint64(math.MaxUint64))

		pt := NewPoint("m1", kvs, WithPrecheck(false))
		pts := CheckPoints([]*Point{pt}, WithU64Field(false))
		assert.Len(t, pts, 1)
		assert.Nil(t, pts[0].Get("u64"))
		assert.Equal(t, "hello", pts[0].Get("str"))
	})

	t.Run("dot-in-key", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f.1", 1.23)
		kvs = kvs.Add("u64", uint64(math.MaxUint64))

		pt := NewPoint("m1", kvs, WithPrecheck(false))

		pts := CheckPoints([]*Point{pt}, WithDotInKey(false))
		assert.Len(t, pts, 1)
		assert.Equal(t, uint64(math.MaxUint64), pts[0].Get("u64"))
		assert.Equal(t, 1.23, pts[0].Get("f_1"))
		assert.Len(t, pts[0].Warns(), 1)

		t.Logf("point: %s", pts[0].Pretty())
	})
}

func TestCheckTags(t *T.T) {
	cases := []struct {
		name   string
		t      map[string]string
		expect KVs
		warns  int
		opts   []Option
	}{
		{
			name: "disable-tag",
			t: map[string]string{
				"t1": "123456",
				"t2": "23456",
			},
			opts: []Option{
				WithDisabledKeys(NewTagKey(`t1`, "")),
			},
			warns: 1,
			expect: NewTags(
				map[string]string{
					"t2": "23456",
				}),
		},

		// { TODO
		//	name: `exceed-tag-kv-compose`,
		//	t: map[string]string{
		//		"t1": "12345",
		//		"t2": "abcde",
		//	},
		//	opts: []Option{
		//		WithMaxKVComposeLen(10),
		//		WithTime(time.Unix(0, 123)),
		//	},

		//	warns: 1,
		//	expect: NewTags(map[string]string{
		//		"t1": "12345",
		//	}),
		// },

		{
			name: `tag-kv-compose-limit-0`,
			t: map[string]string{
				"t1": "12345",
				"t2": "abcde",
			},
			opts: []Option{
				WithMaxMeasurementLen(0), // do nothing
			},

			expect: NewTags(map[string]string{
				"t1": "12345",
				"t2": "abcde",
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			cfg := GetCfg()
			defer PutCfg(cfg)

			for _, opt := range tc.opts {
				opt(cfg)
			}

			c := checker{cfg: cfg}
			kvs := c.checkKVs(NewTags(tc.t))

			assert.Equal(t, tc.warns, len(c.warns), "got warns: %v, kvs: %s", c.warns, kvs.Pretty())

			eopt := eqopt{}
			if tc.expect != nil {
				eq, r := eopt.kvsEq(tc.expect, kvs)
				assert.True(t, eq, "reason: %s", r)
			}
		})
	}

	t.Run("key-updated-but-conflict", func(t *T.T) {
		///////////////////////
		// dot in tag key
		var kvs KVs
		kvs = kvs.Add("f.1", "some string", WithKVTagSet(true))
		kvs = kvs.Add("f_1", 1.23)

		pt := NewPoint("m", kvs, WithDotInKey(false))

		assert.Lenf(t, pt.pt.Fields, 1, "pt: %s", pt.Pretty())
		// drop tag
		assert.Len(t, pt.pt.Fields, 1)
		assert.Equal(t, 1.23, pt.Get(`f_1`).(float64))
		t.Logf("pt: %s", pt.Pretty())

		///////////////////////
		// too long tag key
		kvs = kvs[:0]
		kvs = kvs.Add("f111", "some string", WithKVTagSet(true))
		kvs = kvs.Add("f1", 1.23)
		pt = NewPoint("m", kvs, WithMaxTagKeyLen(2))

		assert.Len(t, pt.pt.Fields, 1)
		// drop tag
		assert.Equal(t, 1.23, pt.Get(`f1`).(float64))
		t.Logf("pt: %s", pt.Pretty())

		///////////////////////
		// too long field key
		kvs = kvs[:0]
		kvs = kvs.Add("f1", 1.23)
		kvs = kvs.Add("f111", "some string")
		pt = NewPoint("m", kvs, WithMaxFieldKeyLen(2))

		assert.Len(t, pt.pt.Fields, 1)
		// drop field
		assert.Equal(t, 1.23, pt.Get(`f1`).(float64))
		t.Logf("pt: %s", pt.Pretty())

		///////////////////////
		// conflict on updated-key
		kvs = kvs[:0]
		kvs = kvs.Add("f.1", 1.23)            // f.1 => f_1
		kvs = kvs.Add("f_111", "some string") // f_111 => f_1: conflict
		pt = NewPoint("m", kvs, WithMaxFieldKeyLen(3), WithDotInKey(false))

		assert.Len(t, pt.pt.Fields, 1)
		// drop field
		assert.Equal(t, 1.23, pt.Get(`f_1`).(float64))
		t.Logf("pt: %s", pt.Pretty())
	})
}

func TestCheckFields(t *T.T) {
	cases := []struct {
		name   string
		f      map[string]interface{}
		expect map[string]interface{}
		warns  int
		opts   []Option
	}{
		{
			name: "exceed-max-field-len",
			f: map[string]interface{}{
				"f1": "123456",
			},
			opts:  []Option{WithMaxFieldValLen(1)},
			warns: 1,
			expect: map[string]interface{}{
				"f1": "1",
			},
		},

		{
			name: "exceed-max-field-count",
			f: map[string]interface{}{
				"f1": "aaaaaa1",
				"f2": "aaaaaa2",
				"f3": "aaaaaa3",
				"f4": "aaaaaa4",
				"f5": "aaaaaa5",
				"f6": "aaaaaa6",
				"f7": "aaaaaa7",
				"f8": "aaaaaa8",
				"f9": "aaaaaa9",
				"f0": "aaaaaa0",
			},
			opts:  []Option{WithMaxFields(1), WithKeySorted(true)},
			warns: 1,
			expect: map[string]interface{}{
				"f0": "aaaaaa0",
			},
		},

		{
			name: "exceed-max-field-key-len",
			f: map[string]interface{}{
				"a1": "123456",
				"b":  "abc123",
			},
			opts:  []Option{WithMaxFieldKeyLen(1)},
			warns: 1,
			expect: map[string]interface{}{
				"a": "123456", // key truncated
				"b": "abc123",
			},
		},

		{
			name: "drop-metric-string-field",
			f: map[string]interface{}{
				"a": 123456,
				"b": "abc123", // dropped
			},
			opts:  []Option{WithStrField(false)},
			warns: 1,
			expect: map[string]interface{}{
				"a": int64(123456),
			},
		},

		{
			name: "invalid-field-type",
			f: map[string]interface{}{
				"b": struct{}{},
			},
			warns: 1,
		},

		{
			name: "nil-field",
			f: map[string]interface{}{
				"a": nil, // set value to nil
				"b": 123,
				"c": struct{}{}, // ignored
			},
			warns: 2,
			expect: map[string]interface{}{
				"b": int64(123),
				"a": nil,
				"c": nil,
			},
		},

		{
			name: "exceed-max-int64-under-influxdb1.x",
			f: map[string]interface{}{
				"b": uint64(math.MaxInt64) + 1, // exceed max-int64
			},
			opts:  DefaultMetricOptionsForInflux1X(),
			warns: 1,
		},

		{
			name: "exceed-max-int64",
			f: map[string]interface{}{
				"a": uint64(math.MaxInt64) + 1, // exceed max-int64, drop the key under non-strict mode
				"b": "abc",
			},

			expect: map[string]interface{}{
				"a": uint64(math.MaxInt64) + 1,
				"b": "abc",
			},
		},

		{
			name: "small-uint64",
			f: map[string]interface{}{
				"a": uint64(12345),
			},
			expect: map[string]interface{}{
				"a": uint64(12345),
			},
		},

		{
			name:   "no-field",
			expect: nil,
			warns:  0,
		},

		{
			name: "dot-in-key",
			f: map[string]interface{}{
				"a.b": 12345,
				"c":   "12345",
			},
			opts:  []Option{WithDotInKey(false)},
			warns: 1,
			expect: map[string]interface{}{
				"a_b": int64(12345),
				"c":   "12345",
			},
		},

		{
			name: "disabled-field",
			f: map[string]interface{}{
				"a": 12345,
				"b": "12345",
			},
			warns: 1,
			opts:  []Option{WithDisabledKeys(NewKey("a", I))},
			expect: map[string]interface{}{
				"b": "12345",
			},
		},

		{
			name: "valid-fields",
			f: map[string]interface{}{
				"small-uint64": uint64(12345),
				"int8":         int8(1),
				"int":          int(1),
				"int16":        int16(12345),
				"int32":        int32(1234567),
				"int64":        int64(123456789),
				"uint8":        uint8(1),
				"uint":         uint(1),
				"uint16":       uint16(12345),
				"uint32":       uint32(1234567),
				"uint64":       uint64(12345678),
				"float32":      float32(1.234),
				"float64":      float64(1.234),
				"str":          "abc",
			},

			expect: map[string]interface{}{
				"small-uint64": uint64(12345),
				"int8":         int64(1),
				"int":          int64(1),
				"int16":        int64(12345),
				"int32":        int64(1234567),
				"int64":        int64(123456789),
				"uint":         uint64(1),
				"uint8":        uint64(1),
				"uint16":       uint64(12345),
				"uint32":       uint64(1234567),
				"uint64":       uint64(12345678),
				"float32":      float32(1.234),
				"float64":      float64(1.234),
				"str":          "abc",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			cfg := GetCfg()
			defer PutCfg(cfg)

			for _, opt := range tc.opts {
				opt(cfg)
			}

			t.Logf("cfg: %+#v", cfg)

			c := checker{cfg: cfg}

			kvs := NewKVs(tc.f)
			expect := NewKVs(tc.expect)

			if cfg.keySorted {
				sort.Sort(kvs)
				sort.Sort(expect)
			}

			kvs = c.checkKVs(kvs)
			require.Equal(t, tc.warns, len(c.warns), "got pt %s", kvs.Pretty())

			eopt := eqopt{}
			if tc.expect != nil {
				eq, _ := eopt.kvsEq(expect, kvs)
				assert.True(t, eq, "expect:\n%s\ngot:\n%s", expect.Pretty(), kvs.Pretty())
			}
		})
	}
}

func TestAdjustKV(t *T.T) {
	cases := []struct {
		name, x, y string
	}{
		{
			name: "x-with-trailling-backslash",
			x:    "x\\",
			y:    "x",
		},

		{
			name: "x-with-line-break",
			x: `
x
def`,
			y: " x def",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			assert.Equal(t, tc.y, adjustKV(tc.x))
		})
	}
}

func TestRequiredKV(t *T.T) {
	t.Run(`add`, func(t *T.T) {
		pt := NewPoint(`abc`, NewKVs(map[string]any{"f1": 123}),
			WithRequiredKeys(NewKey(`rk`, I, 1024)))
		assert.Equal(t, int64(1024), pt.Get(`rk`))
	})
}

func BenchmarkCheck(b *T.B) {
	__shortKey := cliutils.CreateRandomString(10)
	__shortVal := cliutils.CreateRandomString(128)

	cases := []struct {
		name string
		m    string
		t    map[string]string
		f    map[string]interface{}
		opts []Option
	}{
		{
			name: "3-tags-4-field",
			m:    "not-set",
			t: map[string]string{
				__shortKey: __shortVal,
			},
			f: map[string]interface{}{
				"f1": 123,
				"f2": 123.0,
				"f3": __shortVal,
				"f4": false,
			},
		},

		{
			name: "3-tags-4-field-on-string-metric",
			m:    "not-set",
			t: map[string]string{
				__shortKey: __shortVal,
			},
			f: map[string]interface{}{
				"f1": 123,
				"f2": 123.0,
				"f3": __shortVal,
				"f4": false,
			},
			opts: DefaultMetricOptions(),
		},

		{
			name: "3-tags-4-field-on-disabled-tag-and-field",
			m:    "not-set",
			t: map[string]string{
				__shortKey: __shortVal,
				"source":   "should-be-dropped",
			},
			f: map[string]interface{}{
				"f1":     123,
				"f2":     123.0,
				"f3":     __shortVal,
				"f4":     false,
				"source": "should-be-dropped",
			},
			opts: DefaultLoggingOptions(),
		},

		{
			name: "100-tags-300-field-on-warnning-tags-fields",
			m:    "not-set",
			t: func() map[string]string {
				x := map[string]string{}
				for i := 0; i < 100; i++ {
					switch i % 3 {
					case 0: // normal
						x[fmt.Sprintf("%s-%d", __shortKey, i)] = cliutils.CreateRandomString(32)
					case 1: // key contains `\n'
						x[fmt.Sprintf("%s-\n%d", __shortKey, i)] = cliutils.CreateRandomString(32)
					case 2: // key suffix with `\'
						x[fmt.Sprintf("%s-%d\\", __shortKey, i)] = cliutils.CreateRandomString(32)
					}
				}
				return x
			}(),
			f: func() map[string]interface{} {
				x := map[string]interface{}{}
				for i := 0; i < 100; i++ {
					switch i % 3 {
					case 0: // exceed max int64
						x[fmt.Sprintf("%s-%d", __shortKey, i)] = uint64(math.MaxInt64) + 1
					case 1: // exceed max field value length
						x[fmt.Sprintf("%s-%d", __shortKey, i)] = cliutils.CreateRandomString(1024 + 1)
					case 2: // nil
						x[fmt.Sprintf("%s-%d", __shortKey, i)] = nil
					}
				}
				return x
			}(),
			opts: []Option{
				WithMaxFieldValLen(1024),
				WithMaxFields(299), // < 300
			},
		},
	}

	for _, tc := range cases {
		pt, err := NewPointDeprecated(tc.m, tc.t, tc.f, tc.opts...)
		assert.NoError(b, err)

		cfg := GetCfg()
		defer PutCfg(cfg)

		for _, opt := range tc.opts {
			opt(cfg)
		}
		c := checker{cfg: cfg}

		b.ResetTimer()
		b.Run(tc.name, func(b *T.B) {
			for i := 0; i < b.N; i++ {
				c.check(pt)
			}
		})
	}
}

func BenchmarkCheckPoints(b *T.B) {
	b.Run("check-rand-pts", func(b *T.B) {
		r := NewRander()
		pts := r.Rand(1000)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CheckPoints(pts)
		}
	})

	b.Run("check-pts-without-str-field", func(b *T.B) {
		r := NewRander()
		pts := r.Rand(1000)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CheckPoints(pts, WithStrField(false))
		}
	})

	b.Run("check-pts-without-u64-field", func(b *T.B) {
		r := NewRander()
		pts := r.Rand(1000)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			CheckPoints(pts, WithU64Field(false))
		}
	})
}
