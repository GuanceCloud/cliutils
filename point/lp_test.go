// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.
package point

import (
	"fmt"
	"math"
	"testing"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/influxdata/influxdb1-client/models"
	influxm "github.com/influxdata/influxdb1-client/models"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parseLineProto(t *testing.T, data []byte, precision string) (models.Points, error) {
	t.Helper()

	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	return models.ParsePointsWithPrecision(data, time.Now().UTC(), precision)
}

func TestNewLPPoint(t *testing.T) {
	__largeVal := cliutils.CreateRandomString(32)

	cases := []struct {
		tname  string // test name
		name   string
		tags   map[string]string
		fields map[string]interface{}
		opts   []Option
		expect string
		warns  int
		fail   bool
	}{
		{
			tname:  `field-key-with-dot`,
			name:   "abc",
			fields: map[string]interface{}{"f1.a": 1},
			tags:   map[string]string{"t1": "def"},
			opts:   []Option{WithTime(time.Unix(0, 123)), WithDotInKey(false)},
			warns:  1,
			expect: "abc,t1=def f1_a=1i 123",
		},

		{
			tname:  `tag-key-with-dot`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1.a": "def"},
			opts:   []Option{WithTime(time.Unix(0, 123)), WithDotInKey(false)},
			warns:  2,
			expect: `abc,t1_a=def f1=1i 123`,
		},

		{
			tname:  `influx1.x-small-uint`,
			name:   "abc",
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			opts:   append(DefaultMetricOptionsForInflux1X(), WithTime(time.Unix(0, 123))),
			expect: "abc,t1=abc,t2=32 f.1=1i,f2=32i 123",
		},

		{
			tname: `large-field-value-length`,
			name:  "some",
			fields: map[string]interface{}{
				"key1": __largeVal,
			},
			opts: []Option{WithMaxFieldValLen(len(__largeVal) - 2), WithTime(time.Unix(0, 123))},

			expect: fmt.Sprintf(`some key1="%s" 123`, __largeVal[:len(__largeVal)-2]),
			warns:  1,
		},

		{
			tname:  `max-field-value-length`,
			name:   "some",
			fields: map[string]interface{}{"key": "too-long-field-value-123"},
			opts: []Option{
				WithMaxFieldValLen(2),
				WithTime(time.Unix(0, 123)),
			},
			expect: "some key=\"to\" 123",
			warns:  1,
		},

		{
			tname: `max-field-key-length`,
			name:  "some",
			fields: map[string]interface{}{
				__largeVal: "123",
			},
			opts: []Option{
				WithMaxFieldKeyLen(len(__largeVal) - 1),
				WithTime(time.Unix(0, 123)),
			},
			expect: fmt.Sprintf(`some %s="123" 123`, __largeVal[:len(__largeVal)-1]),
			warns:  1,
		},

		{
			tname:  `max-tag-value-length`,
			name:   "some",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"key": "too-long-tag-value-123"},
			opts: []Option{
				WithMaxTagValLen(2),
				WithTime(time.Unix(0, 123)),
			},
			warns:  1,
			expect: "some,key=to f1=1i 123",
		},
		{
			tname:  `disable-string-field`,
			name:   "some",
			fields: map[string]interface{}{"f1": 1, "f2": "this is a string"},
			tags:   map[string]string{"key": "string"},
			opts: []Option{
				WithStrField(false),
				WithTime(time.Unix(0, 123)),
			},
			warns:  1,
			expect: "some,key=string f1=1i 123",
		},

		{
			tname:  `max-tag-key-length`,
			name:   "some",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"too-long-tag-key": "123"},
			opts: []Option{
				WithMaxTagKeyLen(2),
				WithTime(time.Unix(0, 123)),
			},
			warns:  1,
			expect: "some,to=123 f1=1i 123",
		},

		{
			tname:  `empty-measurement-name`,
			name:   "", // empty
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			opts: []Option{
				WithTime(time.Unix(0, 123)),
			},

			warns:  1,
			expect: "__default,t1=abc,t2=32 f1=1i,f2=32i 123",
		},

		{
			tname:  `enable-dot-in-metric-point`,
			name:   "abc",
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t.1": "abc", "t2": "32"},
			opts: []Option{
				WithDotInKey(true),
				WithTime(time.Unix(0, 123)),
			},

			expect: "abc,t.1=abc,t2=32 f.1=1i,f2=32i 123",
		},

		{
			tname:  `with-disabled-field-keys`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32), "f3": 32},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			warns:  1,
			opts: []Option{
				WithDisabledKeys(NewKey(`f1`, I)),
				WithTime(time.Unix(0, 123)),
			},
			expect: "abc,t1=abc,t2=32 f2=32i,f3=32i 123",
		},

		{
			tname:  `with-disabled-tag-keys`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			opts: []Option{
				WithDisabledKeys(NewTagKey(`t2`, "")),
				WithTime(time.Unix(0, 123)),
			},

			warns:  1,
			expect: "abc,t1=abc f1=1i,f2=32i 123",
		},

		{
			tname:  `int-exceed-influx1.x-int64-max`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(math.MaxInt64) + 1},
			opts:   append(DefaultMetricOptionsForInflux1X(), WithTime(time.Unix(0, 123))),

			warns:  1,
			expect: "abc f1=1i 123", // f2 dropped
		},

		{
			tname:  `extra-tags-and-field-exceed-max-tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": "3"},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithMaxTags(2),
				WithMaxFields(1),
				WithKeySorted(true),
				WithExtraTags(map[string]string{
					"etag1": "1",
					"etag2": "2",
				}),
			},

			warns:  2,
			expect: "abc,etag1=1,etag2=2 f1=1i 123", // f2 dropped,
		},

		{
			tname:  `extra-tags-exceed-max-tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			warns:  1,
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithMaxTags(2),
				WithKeySorted(true),
				WithExtraTags(map[string]string{
					"etag1": "1",
					"etag2": "2",
				}),
			},

			expect: "abc,etag1=1,etag2=2 f1=1i 123",
		},

		{
			tname:  `extra-tags-not-exceed-max-tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			expect: "abc,etag1=1,etag2=2,t1=def,t2=abc f1=1i 123",
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithMaxTags(4),
				WithExtraTags(map[string]string{
					"etag1": "1",
					"etag2": "2",
				}),
			},
		},

		{
			tname:  `only-extra-tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			expect: "abc,etag1=1,etag2=2 f1=1i 123",

			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithMaxTags(4),
				WithExtraTags(map[string]string{
					"etag1": "1",
					"etag2": "2",
				}),
			},
		},

		{
			tname:  `exceed-max-tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			opts:   []Option{WithTime(time.Unix(0, 123)), WithMaxTags(1), WithKeySorted(true)},
			expect: `abc,t1=def f1=1i 123`,
			warns:  2,
		},

		{
			tname:  `exceed-max-field`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": 2},
			tags:   map[string]string{"t1": "def"},
			opts:   []Option{WithTime(time.Unix(0, 123)), WithMaxFields(1)},
			warns:  1,
			expect: "abc,t1=def f1=1i 123",
		},

		{
			tname:  `nil-field-not-allowed`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def"},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			warns:  1,
			expect: `abc,t1=def f1=1i 123`,
		},

		{
			tname:  `same-key-in-field-and-tag`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": 2, "x": 123},
			tags:   map[string]string{"t1": "def", "x": "42"},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			warns:  0, // tag `x` override field `x` before checking point(kvs.Add()), so no warning here.
			expect: "abc,t1=def,x=42 f1=1i,f2=2i 123",
		},

		{
			tname:  `no-tag`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   nil,
			opts:   []Option{WithTime(time.Unix(0, 123))},
			expect: "abc f1=1i 123",
		},

		{
			tname:  `no-filed`,
			name:   "abc",
			fields: nil,
			tags:   map[string]string{"f1": "def"},
			fail:   true,
		},

		{
			tname: `field-val-with-new-line`,
			name:  "abc",
			fields: map[string]interface{}{"f1": `abc
123`},
			opts: []Option{WithTime(time.Unix(0, 123))},
			expect: `abc f1="abc
123" 123`,
			fail: false,
		},

		{
			tname: `tag-k-v-with-new-line-under-non-strict`,
			name:  "abc",
			tags: map[string]string{
				"tag1": `abc
123`,
				`tag
2`: `def
456\`,
			},
			fields: map[string]interface{}{"f1": 123},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			warns:  3,
			expect: "abc,tag\\ 2=def\\ 456,tag1=abc\\ 123 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `ok-case`,
			name:   "abc",
			tags:   nil,
			fields: map[string]interface{}{"f1": 123},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			expect: "abc f1=123i 123",
			fail:   false,
		},

		{
			tname:  `tag-key-with-backslash`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			warns:  2,
			expect: `abc,tag1=val1,tag2=val2 f1=123i 123`,
		},

		{
			tname:  `field-is-nil`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},

			opts:  []Option{WithTime(time.Unix(0, 123))},
			warns: 1,

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
		},

		{
			tname:  `field-is-map`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": map[string]interface{}{"a": "b"}},

			opts:  []Option{WithTime(time.Unix(0, 123))},
			warns: 1,

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
		},

		{
			tname:  `field-is-object`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": struct{ a string }{a: "abc"}},

			opts: []Option{WithTime(time.Unix(0, 123))},

			warns:  1,
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
		},

		{
			tname:  `ignore-nil-field`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},
			opts:   []Option{WithTime(time.Unix(0, 123))},

			warns:  3,
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
		},

		{
			tname:  `utf8-characters-in-metric-name`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123},

			opts:  []Option{WithTime(time.Unix(0, 123))},
			warns: 0,

			expect: fmt.Sprintf("%s,tag1=val1,tag2=val2 f1=123i 123", "abc≈≈≈≈øøππ†®"),
		},

		{
			tname:  `utf8-characters-in-metric-name-fields-tags`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`, `tag-中文`: "foobar"},
			fields: map[string]interface{}{"f1": 123, "f2": "¡™£¢∞§¶•ªº", `field-中文`: "barfoo"},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			warns:  0,

			expect: fmt.Sprintf(`%s,tag-中文=foobar,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº",field-中文="barfoo" 123`, "abc≈≈≈≈øøππ†®"),
			fail:   false,
		},

		{
			tname: `missing-field`,
			name:  "abc≈≈≈≈øøππ†®",
			tags:  map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},

			opts: []Option{WithTime(time.Unix(0, 123))},

			expect: `abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº" 123`,
			fail:   true,
		},

		{
			tname: `new-line-in-field`,
			name:  "abc",
			tags:  map[string]string{"tag1": "val1"},
			fields: map[string]interface{}{
				"f1": `aaa
	bbb
			ccc`,
			},
			opts: []Option{WithTime(time.Unix(0, 123))},

			expect: `abc,tag1=val1 f1="aaa
	bbb
			ccc" 123`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.tname, func(t *testing.T) {
			pt, err := NewPoint(tc.name, tc.tags, tc.fields, tc.opts...)

			if tc.fail {
				if pt != nil {
					assert.Error(t, err, "got point %s", pt.Pretty())
				} else {
					assert.Error(t, err)
				}
				t.Logf("expect error: %s", err)
				return
			} else {
				if pt != nil {
					assert.NoError(t, err, "got point %s", pt.Pretty())
				} else {
					assert.NoError(t, err)
				}
			}

			assert.Equal(t, tc.warns, len(pt.pt.Warns), "got pt with warns: %s", pt.Pretty())

			x := pt.LineProto()
			assert.Equal(t, tc.expect, x, "got pt %s", pt.Pretty())
			_, err = parseLineProto(t, []byte(x), "n")
			if err != nil {
				t.Logf("parseLineProto: %s", err)
			}
		})
	}
}

func TestParsePoint(t *testing.T) {
	newPoint := func(m string,
		tags map[string]string,
		fields map[string]interface{},
		ts ...time.Time,
	) *influxdb.Point {
		pt, err := influxdb.NewPoint(m, tags, fields, ts...)
		if err != nil {
			t.Fatal(err) // should never been here
		}

		return pt
	}

	__32mbString := cliutils.CreateRandomString(32 * 1024 * 1024)
	__65kbString := cliutils.CreateRandomString(65 * 1024)

	cases := []struct {
		name   string
		data   []byte
		opts   []Option
		expect []*influxdb.Point
		fail   bool
		skip   bool
	}{
		{
			name: `32mb-field`,
			data: []byte(fmt.Sprintf(`abc f1="%s" 123`, __32mbString)),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithMaxFieldValLen(32 * 1024 * 1024),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": __32mbString},
					time.Unix(0, 123)),
			},
		},
		{
			name: `65k-field`,
			data: []byte(fmt.Sprintf(`abc f1="%s" 123`, __65kbString)),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithMaxFieldValLen(0),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": __65kbString},
					time.Unix(0, 123)),
			},
		},

		{
			name: `with-disabled-field`,
			data: []byte(`abc,t1=1,t2=2 f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithDisabledKeys(NewKey(`f1`, I)),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"t1": "1", "t2": "2"},
					map[string]interface{}{"f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `with-disabled-tags`,
			data: []byte(`abc,t1=1,t2=2 f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithDisabledKeys(NewTagKey(`t1`, "")),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"t2": "2"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `exceed-max-tags`,
			data: []byte(`abc,t1=1,t2=2 f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{WithTime(time.Unix(0, 123)), WithMaxTags(1), WithKeySorted(true)},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"t1": "1"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `exceed-max-fields`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{WithTime(time.Unix(0, 123)), WithMaxFields(2), WithKeySorted(true)},

			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0},
					time.Unix(0, 123)),
			},
		},

		{
			name: `tag-key-with-dot`,
			data: []byte(`abc,tag.1=xxx f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{WithTime(time.Unix(0, 123)), WithDotInKey(false)},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag_1": "xxx"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `field-key-with-dot`,
			data: []byte(`abc f.1=1i,f2=2,f3="abc" 123`),
			opts: []Option{WithDotInKey(false), WithTime(time.Unix(0, 123))},
			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f_1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `with-comments`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123
# some comments
abc f1=1i,f2=2,f3="abc" 456
							# other comments with leading spaces
abc f1=1i,f2=2,f3="abc" 789

			`),
			opts: []Option{WithTime(time.Unix(0, 123))},
			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),

				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 456)),

				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 789)),
			},
		},

		{
			name: `same-key-in-field-and-tag-dup-tag-comes-from-extra-tags`,
			data: []byte(`abc f1="abc",x=1i,f3=2 123`),
			opts: []Option{WithTime(time.Unix(0, 123)), WithExtraTags(map[string]string{"x": "456"})},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"x": "456"},
					map[string]interface{}{"f1": "abc", "f3": 2.0}, // field x dropped
					time.Unix(0, 123)),
			},
		},

		{
			name: `same-key-in-tags-and-fields`,
			data: []byte(`abc,b=abc a=1i,b=2 123`),
			opts: []Option{WithTime(time.Unix(0, 123))},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"b": "abc"},
					map[string]interface{}{"a": 1}, // field b dropped
					time.Unix(0, 123)),
			},
		},

		{
			name: `same-key-in-fields`,
			data: []byte(`abc,b=abc a=1i,c="xyz",c=12 123`),
			skip: true, // this maybe a bug in influxdb: same key in fields parse ok!
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"b": "abc"},
					map[string]interface{}{"a": 1, "c": "xyz"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `same-key-in-tag`,
			data: []byte(`abc,b=abc,b=xyz a=1i 123`),
			fail: true,
		},

		{
			name: `empty-data`,
			data: nil,
			fail: true,
		},

		{
			name: "normal-case",
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{WithTime(time.Unix(0, 123))},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag1": "1", "tag2": "2"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `no-tags`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{WithTime(time.Unix(0, 123))},
			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `multiple-empty-lines-in-body`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123

abc f1=1i,f2=2,f3="abc" 456

abc f1=1i,f2=2,f3="abc" 789

			`),
			opts: []Option{WithTime(time.Unix(0, 123))},
			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),

				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 456)),

				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 789)),
			},
		},

		{
			name: `no-fields`,
			data: []byte(`abc,tag1=1,tag2=2 123123`),
			fail: true,
		},

		{
			name: `parse-with-extra-tags`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithExtraTags(map[string]string{"tag1": "1", "tag2": "2"}),
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag1": "1", "tag2": "2"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `extra-tag-key-with-slash-suffix`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithExtraTags(map[string]string{`tag1\`: `1`, "tag2": `2`}),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag1`: "1", "tag2": `2`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `extra-tag-val-with-slash-suffix`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithExtraTags(map[string]string{`tag1`: `1,`, "tag2": `2\`, "tag3": `3`}),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag1`: "1,", "tag2": `2`, "tag3": `3`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `extra-tag-kv-with-slash`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithExtraTags(map[string]string{`tag\1`: `1`, "tag2": `2\34`}),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag\1`: "1", "tag2": `2\34`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `tag-kv-with-slash-missing-tag-value`,
			data: []byte(`abc,tag1\=1,tag2=2\ f1=1i 123123`),
			fail: true,
		},

		{
			name: `parse-with-callback-no-point`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),

			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithCallback(func(p *Point) (*Point, error) {
					return nil, nil
				}),
			},

			fail: true,
		},

		{
			name: `parse-with-callback-failed`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),

			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithCallback(func(p *Point) (*Point, error) {
					return nil, fmt.Errorf("callback failed")
				}),
			},

			fail: true,
		},

		{
			name: `parse-with-callback`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),

			opts: []Option{
				WithTime(time.Unix(0, 123)),
				WithCallback(func(p *Point) (*Point, error) {
					if p.Name() == "abc" {
						t.Logf("haha, we get measurement `abc'")
					}
					return p, nil
				}),
			},

			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `parse-field-with-new-line`,
			data: []byte(`abc f1="data
with
new
line" 123`),
			expect: []*influxdb.Point{
				newPoint(`abc`, nil, map[string]any{`f1`: `data
with
new
line`}, time.Unix(0, 123)),
			},
		},
	}

	for _, tc := range cases {
		if tc.skip {
			t.Logf("skip case %s", tc.name)
			continue
		}

		t.Run(tc.name, func(t *testing.T) {
			c := GetCfg()
			defer PutCfg(c)

			for _, opt := range tc.opts {
				opt(c)
			}

			pts, err := parseLPPoints(tc.data, c)
			if tc.fail {
				if len(pts) > 0 {
					assert.Error(t, err, "got point[0]: %s", pts[0].Pretty())
				} else {
					assert.Error(t, err)
				}
				t.Logf("expect error: %s", err)
				return
			}

			assert.NoError(t, err, "data: %q", tc.data)

			for idx, pt := range pts {
				if len(tc.expect) > 0 {
					got := pt.LineProto()
					assert.Equal(t, tc.expect[idx].String(), got, "got point: %s", pt.Pretty())

					if _, err := parseLineProto(t, []byte(got), "n"); err != nil {
						t.Logf("parseLineProto failed: %s", err)
						continue
					}
				}
			}
		})
	}
}

func TestParseLineProto(t *testing.T) {
	__32mbString := cliutils.CreateRandomString(32 * 1024 * 1024)
	__65kbString := cliutils.CreateRandomString(65 * 1024)

	cases := []struct {
		data  []byte
		prec  string
		fail  bool
		name  string
		check func(pts models.Points) error
	}{
		{
			name: `nil-data`,
			data: nil,
			prec: "n",
			fail: true,
		},

		{
			name: `no-data`,
			data: []byte(""),
			prec: "n",
			fail: true,
		},

		{
			name: `with-multiple-empty-lines`,
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"

		`),
			prec: "n",
		},

		{
			name: `missing-field`,
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc
		`),
			prec: "n",
			fail: true,
		},

		{
			name: `missing-tag`,
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
			abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
			abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc" 123456789
			abc f1=1i,f2=2,f3="abc"
			`),
			prec: "n",
		},

		{
			name: `65kb-field-key`,
			data: []byte(fmt.Sprintf(`abc,tag1=1,tag2=2 "%s"="hello" 123`, func() string {
				return __65kbString
			}())),

			fail: true,
			prec: "n",
		},

		{
			name: `65kb-tag-key`,
			data: []byte(fmt.Sprintf(`abc,tag1=1,%s=2 f1="hello" 123`, func() string {
				return __65kbString
			}())),

			fail: true,
			prec: "n",
		},

		{
			name: `65kb-measurement-name`,
			data: []byte(fmt.Sprintf(`%s,tag1=1,t2=2 f1="hello" 123`, func() string {
				return __65kbString
			}())),

			fail: true,
			prec: "n",
		},

		{
			name: `parse-uint`,
			data: []byte(`abc,t1=v1 f1=32u 123`), // Error:  unable to parse 'abc,t1=v1 f1=32u 123': invalid number
			fail: true,
			prec: "n",
		},

		{
			name: `32mb-field`,
			data: []byte(fmt.Sprintf(`abc,tag1=1,tag2=2 f3="%s" 123`, func() string {
				return __32mbString
			}())),

			check: func(pts models.Points) error {
				if len(pts) != 1 {
					return fmt.Errorf("expect only 1 point, got %d", len(pts))
				}

				fields, err := pts[0].Fields()
				if err != nil {
					return err
				}

				if v, ok := fields["f3"]; !ok {
					return fmt.Errorf("field f3 missing")
				} else if v != __32mbString {
					return fmt.Errorf("field f3 not expected")
				}
				return nil
			},

			prec: "n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pts, err := parseLineProto(t, tc.data, tc.prec)

			if tc.fail {
				assert.Error(t, err)
				t.Logf("expect error: %s", cliutils.LeftStringTrim(err.Error(), 64))
			} else {
				assert.NoError(t, err)
			}

			if tc.check != nil {
				assert.NoError(t, tc.check(pts))
			}
		})
	}
}

func TestAppendString(t *T.T) {
	r := NewRander()
	pts := r.Rand(3)

	var lppts []influxm.Point
	for _, pt := range pts {
		lppt, err := pt.LPPoint()
		assert.NoError(t, err)
		lppts = append(lppts, lppt)
	}

	var ptbuf []byte
	totalBuf := make([]byte, 4<<20)

	for _, pt := range lppts {
		ptbuf = pt.AppendString(ptbuf)
		totalBuf = append(totalBuf, ptbuf[:pt.StringSize()]...)
		totalBuf = append(totalBuf, '\n')

		ptbuf = ptbuf[:0]
	}

	t.Logf("total:\n%s", string(totalBuf))

	dec := GetDecoder(WithDecEncoding(LineProtocol))
	decPts, err := dec.Decode(totalBuf)
	assert.NoError(t, err)

	for i, pt := range decPts {
		exp := pts[i].Pretty()
		got := pt.Pretty()
		require.Equalf(t, exp, got, "exp %s\ngot %s", exp, got)

		t.Logf("got %s", got)
	}
}

func BenchmarkLPString(b *T.B) {
	r := NewRander()
	pts := r.Rand(100)

	var lppts []influxm.Point
	for _, pt := range pts {
		lppt, err := pt.LPPoint()
		assert.NoError(b, err)
		lppts = append(lppts, lppt)
	}

	var ptbuf []byte
	totalBuf := make([]byte, 0, 8<<20)

	b.ResetTimer()
	b.Run("AppendString", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			for _, pt := range lppts {
				ptbuf = pt.AppendString(ptbuf)
				totalBuf = append(totalBuf, ptbuf[:pt.StringSize()]...)
				totalBuf = append(totalBuf, '\n')

				ptbuf = ptbuf[:0]
			}
		}
	})

	b.Run("String", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			for _, pt := range lppts {
				ptstr := pt.String()
				totalBuf = append(totalBuf, []byte(ptstr)...)
				totalBuf = append(totalBuf, '\n')
			}
		}
	})
}
