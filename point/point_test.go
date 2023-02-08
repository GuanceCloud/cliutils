// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

func TestGetTag(t *T.T) {
	t.Run(`get-tag`, func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(nil).MustAddTag([]byte(`t1`), []byte(`v1`)))

		assert.Equal(t, []byte(`v1`), pt.GetTag([]byte(`t1`)))
		assert.Nil(t, pt.GetTag([]byte(`not-exist`)))

		// get non-tag key
		pt.kvs = pt.kvs.MustAddKV(NewKV([]byte(`f1`), 1.23,
			WithKVUnit("bytes"),
			WithKVTagSet(true), // set failed
			WithKVType(MetricType_COUNT)))
		assert.Nil(t, pt.GetTag([]byte(`f1`)))

		pt.AddTag([]byte(`empty-tag`), []byte(``))
		assert.Equal(t, []byte(``), pt.GetTag([]byte(`empty-tag`)))

		t.Logf("kvs:\n%s", pt.kvs.Pretty())
	})
}

func TestFlags(t *T.T) {
	t.Run("test-flag-value", func(t *T.T) {
		t.Logf("Psent: %d", Psent)
		t.Logf("Ppb: %d", Ppb)
	})

	t.Run("test-flag-set-clear", func(t *T.T) {
		pt := &Point{}
		pt.SetFlag(Psent)

		assert.True(t, pt.HasFlag(Psent))

		pt.SetFlag(Ppb)
		assert.True(t, pt.HasFlag(Ppb))

		pt.ClearFlag(Ppb)
		assert.False(t, pt.HasFlag(Ppb))

		pt.ClearFlag(Psent)
		assert.False(t, pt.HasFlag(Psent))
	})
}

func TestPrettyPoint(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": 123}).AddTag([]byte("t1"), []byte("v1")))
		t.Logf("%s", pt.Pretty())
	})

	t.Run(`with-warns`, func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": 123}).
			AddTag([]byte("t1"), []byte("v1")).
			AddTag([]byte("t2"), []byte("v1")),
			WithDisabledKeys(NewTagKey([]byte(`t2`), nil)))

		t.Logf("%s", pt.Pretty())
	})

	t.Run(`with-all-types`, func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{
			"f1": 123,
			"f2": uint64(321),
			"f3": 3.14,
			"f4": false,
			"f5": []byte("hello"),
		}).
			AddTag([]byte("t1"), []byte("v1")).
			AddTag([]byte("t2"), []byte("v1")),
			WithDisabledKeys(NewTagKey([]byte(`t2`), nil)))

		t.Logf("%s", pt.Pretty())
	})
}

func TestPointString(t *T.T) {
	cases := []struct {
		name   string
		pt     *Point
		expect string
	}{
		{
			name: "normal-lppt",
			pt: func() *Point {
				pt, err := NewPoint("abc",
					map[string]string{"tag1": "v1"},
					map[string]interface{}{
						"f1": 123, "f2": true,
					},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)
				return pt
			}(),
			expect: `abc,tag1=v1 f1=123i,f2=true 123`,
		},

		{
			name: "normal-pbpt",
			pt: func() *Point {
				pt, err := NewPoint("abc",
					map[string]string{
						"tag1": "v1",
						"tag2": "v2",
						"xtag": "vx",
					}, map[string]interface{}{
						"f1": 123,
						"f2": true,
						"f3": uint64(123),
						"f4": 123.4,
						"f5": "foobar",
						"f6": []byte("hello, 屈原"),
						"f7": struct{ a int }{a: 123},
					},
					WithTime(time.Unix(0, 123)), WithEncoding(Protobuf))
				assert.NoError(t, err)
				return pt
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			if tc.pt.HasFlag(Ppb) {
				j, err := json.Marshal(tc.pt) // json protobuf point
				assert.NoError(t, err)

				var marshalPt Point
				assert.NoError(t, json.Unmarshal(j, &marshalPt))

				t.Logf("pb.JSON string: %s", j)

				assert.True(t, tc.pt.Equal(&marshalPt))

				t.Logf("pb.String: %s", marshalPt.Pretty())
			} else {
				assert.Equal(t, tc.expect, tc.pt.LineProto())
			}
		})
	}
}

func TestInfluxTags(t *T.T) {
	t.Run("get-tags", func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": 123}).AddTag([]byte(`t1`), []byte(`v1`)))
		tags := pt.InfluxTags()
		assert.Equal(t, map[string]string{"t1": "v1"}, tags)

		t.Log(pt.Pretty())
	})

	t.Run("no-tags", func(t *T.T) {
		pt := NewPointV2([]byte(`abc`),
			NewKVs(map[string]any{"v1": 123}).
				AddTag([]byte(`v1`), []byte(`foo`))) // tag key exist, skipped

		tags := pt.InfluxTags()
		assert.Equal(t, 0, len(tags), "pt: %s", pt.Pretty())

		t.Log(pt.Pretty())
	})
}

func TestPointLineProtocol(t *T.T) {
	cases := []struct {
		name   string
		pt     *Point
		pb     bool
		prec   Precision
		expect string
	}{
		{
			name: "lp-point-ns-prec",
			prec: NS,
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(0, 123)))...)

				assert.NoError(t, err)

				t.Logf("pt: %+#v", pt)
				return pt
			}(),
			expect: "abc f1=1i 123",
		},

		{
			name: "lp-point-ms-prec",
			prec: MS,
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(0, 12345678)))...)

				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 12",
		},

		{
			name: "lp-point-us-prec",
			prec: US, // only accept u
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(0, 12345678)))...)

				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 12345",
		},

		{
			name: "lp-point-ns-prec",
			prec: NS, // only accept u
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(0, 12345678)))...)
				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 12345678",
		},

		{
			name: "lp-point-invalid-prec",
			prec: -1,
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(0, 12345678)))...)
				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 12345678",
		},

		{
			name: "lp-point-second-prec",
			prec: S,
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(1, 123456789)))...)
				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 1",
		},

		{
			name: "lp-point-minute-prec",
			prec: M,
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(120, 123456789)))...)
				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 2",
		},

		{
			name: "lp-point-hour-prec",
			prec: H,
			pt: func() *Point {
				pt, err := NewPoint("abc", nil, map[string]interface{}{"f1": 1},
					append(DefaultLoggingOptions(), WithTime(time.Unix(7199, 123456789)))...)
				assert.NoError(t, err)
				return pt
			}(),
			expect: "abc f1=1i 1", // 7199 not reached 2hour
		},

		// pb point
		{
			name: "pb-point",
			pb:   true,
			prec: NS,
			pt: func() *Point {
				pt, err := NewPoint("abc",
					nil,
					map[string]interface{}{"f1": int64(1)},
					WithTime(time.Unix(0, 123)), WithEncoding(Protobuf))

				assert.NoError(t, err)

				t.Logf("pb point: %s", pt.Pretty())
				return pt
			}(),
			expect: `abc f1=1i 123`,
		},

		{
			name: "pb-point-with-binary-data",
			pb:   true,
			prec: NS,
			pt: func() *Point {
				pt, err := NewPoint("abc",
					map[string]string{"t1": "v1"},
					map[string]interface{}{"f1": []byte("abc123")},
					WithTime(time.Unix(0, 1)), WithEncoding(Protobuf))

				assert.NoError(t, err)

				t.Logf("pt: %s", pt.Pretty())

				return pt
			}(),
			expect: `abc,t1=v1 f1="abc123" 1`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			assert.Equal(t, tc.expect, tc.pt.LineProto(tc.prec))
		})
	}
}

func TestPBJson(t *T.T) {
	t.Run("pbjson", func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": 123, "f2": 3.14}))

		pt.kvs = pt.kvs.MustAddTag([]byte(`t1`), []byte(`v1`)).
			MustAddKV(NewKV([]byte(`f2`), 3.14, WithKVUnit("kb"), WithKVType(MetricType_COUNT)))

		j, _ := pt.PBJson()
		t.Logf("%s", string(j))

		j, _ = pt.PBJsonPretty()
		t.Logf("%s", string(j))
	})
}

func TestPointPB(t *T.T) {
	t.Run(`valid-types`, func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{
			"f1":  uint64(123),
			"f2":  uint64(math.MaxUint64),
			"f3":  int64(123),
			"f4":  false,
			"f5":  true,
			"f6":  "hello",
			"f7":  []byte("world"),
			"f8":  struct{}{}, // user-defined
			"f9":  nil,
			"f10": 3.14,
		}), WithTime(time.Unix(0, 123)))

		j := fmt.Sprintf(`{
	"name": "%s",
	"fields": [
		{ "key": "%s", "u": "123" },
		{ "key": "%s", "u": "%d" },
		{ "key": "%s", "i": "123" },
		{ "key": "%s", "b": false },
		{ "key": "%s", "b": true },
		{ "key": "%s", "d": "%s" },
		{ "key": "%s", "d": "%s" },
		{ "key": "%s" },
		{ "key": "%s" },
		{ "key": "%s", "f": "%f" }
	], "time":"123"}`,
			b64([]byte(`abc`)),
			b64([]byte(`f1`)),
			b64([]byte(`f2`)), uint64(math.MaxUint64),
			b64([]byte(`f3`)),
			b64([]byte(`f4`)),
			b64([]byte(`f5`)),
			b64([]byte(`f6`)), b64([]byte(`hello`)),
			b64([]byte(`f7`)), b64([]byte(`world`)),
			b64([]byte(`f8`)),
			b64([]byte(`f9`)),
			b64([]byte(`f10`)), float64(3.14))

		expect := MustFromPBJson([]byte(j))

		cfg := GetCfg()
		defer PutCfg(cfg)
		chk := checker{cfg: cfg}
		expect = chk.check(expect)
		expect.SetFlag(Pcheck)
		expect.warns = chk.warns

		assert.Equal(t, expect.Pretty(), pt.Pretty(), "got\n%s\nexpect\n%s", expect.Pretty(), pt.Pretty())

		t.Logf("pt: %s", pt.Pretty())
	})
}

func TestLPPoint(t *T.T) {
	t.Run(`uint`, func(t *T.T) {
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": uint64(123)}), WithTime(time.Unix(0, 123)))
		assert.Equal(t, `abc f1=123i 123`, pt.LPPoint().String())

		// max-int64 is ok
		pt = NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": uint64(math.MaxInt64)}), WithTime(time.Unix(0, 123)))
		assert.Equal(t, fmt.Sprintf(`abc f1=%di 123`, math.MaxInt64), pt.LPPoint().String())

		// max-int64 + 1 not ok
		pt = NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": uint64(math.MaxInt64 + 1), "f2": "foo"}), WithTime(time.Unix(0, 123)))
		assert.Equal(t, `abc f2="foo" 123`, pt.LPPoint().String())

		t.Logf("lp: %s", pt.LPPoint().String())
	})

	t.Run(`nil`, func(t *T.T) {
		// max-int64 + 1 not ok
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": 123, "f2": nil}), WithTime(time.Unix(0, 123)))
		assert.Equal(t, `abc f1=123i 123`, pt.LPPoint().String())

		t.Logf("lp: %s", pt.LPPoint().String())
	})

	t.Run(`struct`, func(t *T.T) {
		// max-int64 + 1 not ok
		pt := NewPointV2([]byte(`abc`), NewKVs(map[string]any{"f1": 123, "f2": struct{}{}}), WithTime(time.Unix(0, 123)))
		assert.Equal(t, `abc f1=123i 123`, pt.LPPoint().String())

		t.Logf("lp: %s", pt.LPPoint().String())
	})
}

func TestFields(t *T.T) {
	someAny, err := anypb.New(&AnyDemo{Demo: "demo example"})
	assert.NoError(t, err)

	cases := []struct {
		name   string
		pt     *Point
		expect map[string]interface{}
	}{
		{
			name: "basic-lp-point",

			pt: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{
						"i8":     int8(1),
						"u8":     uint8(1),
						"i16":    int16(1),
						"u16":    uint16(1),
						"i32":    int32(1),
						"u32":    uint32(1),
						"i64":    int64(1),
						"u64":    uint64(1),
						"f32":    float32(1.0),
						"f64":    float64(1.0),
						"bool_1": false,
						"bool_2": true,
						"str":    "hello",
						"data":   []byte("abc123"),
						"nil":    nil,
						"any":    someAny,
						"udf":    struct{}{},
					})
				assert.NoError(t, err)
				return x
			}(),

			expect: map[string]interface{}{
				"i8":     int64(1),
				"u8":     int64(1),
				"i16":    int64(1),
				"u16":    int64(1),
				"i32":    int64(1),
				"u32":    int64(1),
				"i64":    int64(1),
				"u64":    uint64(1),
				"f32":    float32(1.0),
				"f64":    float64(1.0),
				"bool_1": false,
				"nil":    nil,
				"udf":    nil,
				"any":    someAny,
				"data":   []byte("abc123"),
				"bool_2": true,
				"str":    "hello",
			},
		},

		{
			name: "basic-pb-point",

			pt: func() *Point {
				x, err := NewPoint("abc", nil,
					map[string]interface{}{
						"any":    someAny,
						"bool_1": false,
						"bool_2": true,
						"data":   []byte("abc123"),
						"f32":    float32(1.0),
						"f64":    float64(1.0),
						"i16":    int16(1),
						"i32":    int32(1),
						"i64":    int64(1),
						"i8":     int8(1),
						"nil":    nil,
						"str":    "hello",
						"u16":    uint16(1),
						"u32":    uint32(1),
						"u64":    uint64(1),
						"u8":     uint8(1),
						"udf":    struct{}{},
					}, WithEncoding(Protobuf))
				assert.NoError(t, err)
				return x
			}(),

			expect: map[string]interface{}{
				"bool_1": false,
				"bool_2": true,
				"data":   []byte("abc123"),
				"f32":    float32(1.0),
				"f64":    float64(1.0),
				"i16":    int64(1),
				"i32":    int64(1),
				"i64":    int64(1),
				"i8":     int64(1),
				"str":    "hello",
				"u16":    int64(1),
				"u32":    int64(1),
				"u64":    uint64(1),
				"u8":     int64(1),
				"any":    someAny,
				"nil":    nil,
				"udf":    nil,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			fs := tc.pt.Fields()
			assert.True(t, len(fs) > 0)

			eq, reason := kvsEq(fs, NewKVs(tc.expect))
			assert.True(t, eq, "not equal, reason: %s, pt: %s", reason, tc.pt.Pretty())

			assert.NotNil(t, tc.pt.PBPoint())
			assert.NotNil(t, tc.pt.LPPoint())

			eq, reason = kvsEq(fs, NewKVs(tc.expect))
			assert.True(t, eq, "not equal, reason: %s, pt: %s", reason, tc.pt.kvs.Pretty())
		})
	}
}

func FuzzPBPointString(f *T.F) {
	cases := []struct {
		measurement string
		tagk        string
		tagv        string
		fieldk      string

		i64  int64
		u64  uint64
		str  string
		b    bool
		f    float64
		d    []byte
		time int64
	}{
		{
			measurement: "fuzz",
			tagk:        "tag",
			tagv:        "tval",
			fieldk:      "field",

			i64:  int64(1),
			u64:  uint64(123),
			str:  "hello world",
			b:    false,
			f:    3.14,
			d:    []byte("hello, world"),
			time: 123,
		},
	}

	for _, tc := range cases {
		f.Add(
			tc.measurement, tc.tagk, tc.tagv, tc.fieldk,
			tc.i64, tc.u64, tc.str, tc.b, tc.f, tc.d, tc.time)
	}

	f.Fuzz(func(t *T.T,
		measurement string,
		tagk string,
		tagv string,
		fieldk string,

		i64 int64,
		u64 uint64,
		str string,
		b bool,
		f float64,
		d []byte,
		ts int64,
	) {
		pt, err := NewPoint(measurement,
			map[string]string{tagk: tagv},
			map[string]interface{}{
				"i64": i64,
				"u64": u64,
				"str": str,
				"b":   b,
			}, WithTime(time.Unix(0, 123)), WithDotInKey(true), WithEncoding(Protobuf))

		assert.NoError(t, err)
		if pt != nil {
			t.Logf(pt.Pretty())
		}
	})
}

func TestKey(t *T.T) {
	cases := []struct {
		name   string
		key    []byte
		pt     *Point
		expect any
	}{
		{
			"basic",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": 123})),
			int64(123),
		},

		{
			"query-tag-no-field",
			[]byte(`t1`),
			NewPointV2([]byte("abc"), nil, WithExtraTags(map[string]string{"t1": "v1"})),
			[]byte("v1"),
		},

		{
			"no-field-query-field-not-found",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), nil, nil),
			nil,
		},

		{
			"query-field-not-found",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f2": 123})),
			nil,
		},

		{
			"query-f32",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": float32(3.0)})),
			float64(3.0),
		},

		{
			"query-f64",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": float64(3.14)})),
			float64(3.14),
		},

		{
			"query-u64",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": uint64(3)})),
			uint64(3),
		},

		{
			"query-data",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": []byte("hello")}), WithEncoding(Protobuf)),
			[]byte("hello"),
		},

		{
			"query-bool",
			[]byte(`f1`),
			NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": false})),
			false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			assert.Equal(t, tc.expect, tc.pt.Get(tc.key))
		})
	}
}

func TestPointKeys(t *T.T) {
	t.Run("point-keys", func(t *T.T) {
		p := NewPointV2([]byte("abc"),
			NewKVs(map[string]any{"f1": "123", "f2": false, "f3": float32(3.14)}),
			WithExtraTags(map[string]string{"t1": "t2"}))
		keys := p.Keys()

		t.Logf("keys:\n%s", keys.Pretty())

		hash1 := keys.Hash()

		keys.Add(NewKey([]byte(`hello`), KeyType_D))

		hash2 := keys.Hash()
		assert.NotEqual(t, hash1, hash2, "keys:\n%s", keys.Pretty())

		keys.Del(NewKey([]byte(`hello`), KeyType_D))

		hash3 := keys.Hash()
		assert.Equal(t, hash1, hash3, "keys: \n%s", keys.Pretty())

		keys.Del(NewKey([]byte(`t1`), KeyType_D))

		hash4 := keys.Hash()
		assert.NotEqual(t, hash3, hash4, "keys: \n%s", keys.Pretty())

		t.Logf("keys:\n%s", keys.Pretty())
	})

	t.Run("exist", func(t *T.T) {
		p := NewPointV2([]byte("abc"), NewKVs(map[string]any{"x1": "123"}))
		keys := p.Keys()

		assert.True(t, keys.Has(NewKey([]byte(`x1`), KeyType_D)), "keys:\n%s", keys.Pretty())
	})

	t.Run("add", func(t *T.T) {
		p := NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": "123"}))
		keys := p.Keys()

		h1 := keys.Hash()

		// add exist key
		keys.Add(NewKey([]byte(`f1`), KeyType_D))

		h2 := keys.Hash()
		assert.Equal(t, h1, h2, "keys:\n%s", keys.Pretty())
	})

	t.Run("no-kvs", func(t *T.T) {
		p := NewPointV2([]byte("abc"), nil)
		keys := p.Keys()

		t.Logf("keys:\n%s", keys.Pretty())

		hash1 := keys.Hash()

		keys.Add(NewKey([]byte("hello"), KeyType_D))

		hash2 := keys.Hash()
		assert.NotEqual(t, hash1, hash2, "keys:\n%s", keys.Pretty())

		keys.Del(NewKey([]byte("hello"), KeyType_D))

		hash3 := keys.Hash()
		assert.Equal(t, hash1, hash3, "keys: \n%s", keys.Pretty())

		// delete not-exist-key
		hc := keys.hashCount
		keys.Del(NewKey([]byte("t1"), KeyType_D))
		hash4 := keys.Hash()
		assert.Equal(t, hash3, hash4, "keys: \n%s", keys.Pretty())
		assert.Equal(t, hc, keys.hashCount)

		t.Logf("keys:\n%s", keys.Pretty())
	})
}

func TestPointAddKey(t *T.T) {
	t.Run("add", func(t *T.T) {
		pt := NewPointV2([]byte("abc"), NewKVs(map[string]any{"f1": 123}))
		pt.Add([]byte("new-key"), "hello")
		assert.True(t, pt.kvs.Has([]byte(`new-key`)), "fields: %s", pt.kvs.Pretty())
	})
}

func TestSize(t *T.T) {
	t.Run("sizes", func(t *T.T) {
		// empty point
		pt := NewPointV2([]byte(`abc`), nil)
		t.Logf("pt size: %d, pb size: %d, lp size: %d", pt.Size(), pt.PBSize(), pt.LPSize())

		// basic point
		pt = NewPointV2([]byte(`abc`), NewKVs(map[string]any{
			"f1": 123,
			"f2": uint64(123),
			"f3": false,
			"f4": 3.14,
			"f5": []byte(`hello`),
		}))
		t.Logf("pt size: %d, pb size: %d, lp size: %d", pt.Size(), pt.PBSize(), pt.LPSize())

		// large numbers
		pt = NewPointV2([]byte(`abc`), NewKVs(map[string]any{
			"f1": math.MaxInt64,
			"f3": false,
			"f5": []byte(strings.Repeat(`hello`, 100)),
			"f4": float64(math.MaxFloat64),
			"f6": float32(math.MaxFloat32),
			"f7": 3.14,
			"f9": 3.14159265359,
		}))
		t.Logf("pt size: %d, pb size: %d, lp size: %d", pt.Size(), pt.PBSize(), pt.LPSize())
		t.Logf("lp: %s", pt.LineProto())

		// with kv unit/type
		pt = NewPointV2([]byte(`abc`), NewKVs(nil).
			MustAddKV(NewKV([]byte(`f1`), 123, WithKVUnit("MB"), WithKVType(MetricType_COUNT))).
			MustAddTag([]byte(`t1`), []byte(`v1`)))
		t.Logf("pt size: %d, pb size: %d, lp size: %d", pt.Size(), pt.PBSize(), pt.LPSize())

		// rand point
		r := NewRander(WithRandText(3))
		pts := r.Rand(10)
		for idx, pt := range pts {
			t.Logf("[%d] pt size: % 5d, pb size: % 5d, lp size: % 5d", idx, pt.Size(), pt.PBSize(), pt.LPSize())
		}
	})
}
