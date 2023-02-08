// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package lineproto

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/influxdata/influxdb1-client/models"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"
)

func parseLineProto(t *testing.T, data []byte, precision string) (models.Points, error) {
	t.Helper()

	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	return models.ParsePointsWithPrecision(data, time.Now().UTC(), precision)
}

func TestAdjustTags(t *testing.T) {
	cases := []struct {
		tags map[string]string
	}{}

	_ = cases
}

func TestAdjustKV(t *testing.T) {
	cases := []struct {
		name, x, y string
	}{
		{
			name: "x with trailling backslash",
			x:    "x\\",
			y:    "x",
		},

		{
			name: "x with line break",
			x: `
x
def`,
			y: " x def",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testutil.Equals(t, tc.y, adjustKV(tc.x))
		})
	}
}

func TestMakeLineProtoPointWithWarnings(t *testing.T) {
	cases := []struct {
		tname     string // test name
		name      string
		tags      map[string]string
		fields    map[string]interface{}
		ts        time.Time
		opt       *Option
		expect    string
		warnTypes []string
		fail      bool
	}{
		{
			tname: `64k-field-value-length`,
			name:  "some",
			fields: map[string]interface{}{
				"key": func() string {
					const str = "1234567890"
					var out string
					for {
						out += str
						if len(out) > 64*1024 {
							break
						}
					}
					return out
				}(),
			},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxFieldValueLen = 0
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: fmt.Sprintf(`some key="%s" 123`, func() string {
				const str = "1234567890"
				var out string
				for {
					out += str
					if len(out) > 64*1024 {
						break
					}
				}
				return out
			}()),
		},

		{
			tname:  `max-field-value-length`,
			name:   "some",
			fields: map[string]interface{}{"key": "too-long-field-value-123"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxFieldValueLen = 2
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			expect:    "some key=\"to\" 123",
			warnTypes: []string{WarnMaxFieldValueLen},
		},

		{
			tname:  `max-field-key-length`,
			name:   "some",
			fields: map[string]interface{}{"too-long-field-key": "123"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxFieldKeyLen = 2
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			expect:    "some to=\"123\" 123",
			warnTypes: []string{WarnMaxFieldKeyLen},
		},

		{
			tname:  `max-tag-value-length`,
			name:   "some",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"key": "too-long-tag-value-123"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxTagValueLen = 2
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			warnTypes: []string{WarnMaxTagValueLen},
			expect:    "some,key=to f1=1i 123",
		},
		{
			tname:  `disable-string-field`,
			name:   "some",
			fields: map[string]interface{}{"f1": 1, "f2": "this is a string"},
			tags:   map[string]string{"key": "string"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.DisableStringField = true
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			warnTypes: []string{WarnInvalidFieldValueType},
			expect:    "some,key=string f1=1i 123",
		},

		{
			tname:  `max tag key length`,
			name:   "some",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"too-long-tag-key": "123"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxTagKeyLen = 2
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			warnTypes: []string{WarnMaxTagKeyLen},
			expect:    "some,to=123 f1=1i 123",
		},

		{
			tname:  `empty measurement name`,
			name:   "", // empty
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t.1": "abc", "t2": "32"},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.EnablePointInKey = true
				return opt
			}(),

			fail: true,
		},

		{
			tname:  `enable point in metric point`,
			name:   "abc",
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t.1": "abc", "t2": "32"},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.EnablePointInKey = true
				return opt
			}(),

			expect: "abc,t.1=abc,t2=32 f.1=1i,f2=32i 123",
		},

		{
			tname:  `enable point in metric point`,
			name:   "abc",
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.EnablePointInKey = true
				return opt
			}(),
			expect: "abc,t1=abc,t2=32 f.1=1i,f2=32i 123",
		},

		{
			tname:  `with disabled field keys`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.DisabledFieldKeys = []string{"f1"}
				return opt
			}(),

			fail: true,
		},

		{
			tname:  `with disabled tag keys`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.DisabledTagKeys = []string{"t2"}
				return opt
			}(),

			fail: true,
		},

		{
			tname:  `int exceed int64-max under non-strict mode`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			expect: "abc f1=1i,f2=32i 123",
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			fail: false,
		},

		{
			tname:     `int exceed int64-max under non-strict mode`,
			name:      "abc",
			fields:    map[string]interface{}{"f1": 1, "f2": uint64(math.MaxInt64) + 1},
			expect:    "abc f1=1i 123", // f2 dropped
			warnTypes: []string{WarnMaxFieldValueInt},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.Strict = false
				return opt
			}(),

			fail: false,
		},

		{
			tname:  `int exceed int64-max under strict mode`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(math.MaxInt64) + 1},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			fail: true,
		},

		{
			tname:  `extra tags and field exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": "3"},
			tags:   map[string]string{"t1": "def", "t2": "abc"},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.MaxTags = 2
				opt.MaxFields = 1
				opt.ExtraTags = map[string]string{
					"etag1": "1",
					"etag2": "2",
				}
				return opt
			}(),
			warnTypes: []string{WarnMaxTags, WarnMaxFields},
			expect:    "abc,etag1=1,etag2=2 f1=1i 123", // f2 dropped,
		},

		{
			tname:     `extra tags exceed max tags`,
			name:      "abc",
			fields:    map[string]interface{}{"f1": 1},
			tags:      map[string]string{"t1": "def", "t2": "abc"},
			warnTypes: []string{WarnMaxTags},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.MaxTags = 2
				opt.ExtraTags = map[string]string{
					"etag1": "1",
					"etag2": "2",
				}
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			expect: "abc,etag1=1,etag2=2 f1=1i 123",
		},

		{
			tname:  `extra tags not exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			expect: "abc,etag1=1,etag2=2,t1=def,t2=abc f1=1i 123",

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.MaxTags = 4
				opt.ExtraTags = map[string]string{
					"etag1": "1",
					"etag2": "2",
				}
				return opt
			}(),

			fail: false,
		},

		{
			tname:  `only extra tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			expect: "abc,etag1=1,etag2=2 f1=1i 123",

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.MaxTags = 4
				opt.ExtraTags = map[string]string{
					"etag1": "1",
					"etag2": "2",
				}
				return opt
			}(),

			fail: false,
		},

		{
			tname:  `exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.MaxTags = 1
				return opt
			}(),
			warnTypes: []string{WarnMaxTags},
			fail:      true,
		},

		{
			tname:  `exceed max field`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": 2},
			tags:   map[string]string{"t1": "def"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.MaxFields = 1
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			warnTypes: []string{WarnMaxFields},
			expect:    "abc,t1=def f1=1i 123",
		},

		{
			tname:  `field key with "."`,
			name:   "abc",
			fields: map[string]interface{}{"f1.a": 1},
			tags:   map[string]string{"t1.a": "def"},
			opt:    NewDefaultOption(),
			fail:   true,
		},

		{
			tname:  `field key with "."`,
			name:   "abc",
			fields: map[string]interface{}{"f1.a": 1},
			tags:   map[string]string{"t1": "def"},
			opt:    NewDefaultOption(),
			fail:   true,
		},

		{
			tname:  `tag key with "."`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1.a": "def"},
			opt:    NewDefaultOption(),
			fail:   true,
		},

		{
			tname:  `nil field, not allowed`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			fail: true,
		},

		{
			tname:  `same key in field and tag`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": 2},
			tags:   map[string]string{"f1": "def"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			warnTypes: []string{WarnSameTagFieldKey},
			expect:    "abc,f1=def f2=2i 123",
		},

		{
			tname:  `no tag`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   nil,
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			expect: "abc f1=1i 123",
		},

		{
			tname:  `no filed`,
			name:   "abc",
			fields: nil,
			tags:   map[string]string{"f1": "def"},
			opt:    NewDefaultOption(),
			fail:   true,
		},

		{
			tname: `field-val with '\n'`,
			name:  "abc",
			fields: map[string]interface{}{"f1": `abc
123`},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.Strict = false
				return opt
			}(),
			expect: `abc f1="abc
123" 123`,
			fail: false,
		},

		{
			tname: `tag-k/v with '\n' under non-strict`,
			name:  "abc",
			tags: map[string]string{
				"tag1": `abc
123`,
				`tag
2`: `def
456\`,
			},
			fields: map[string]interface{}{"f1": 123},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.Strict = false
				return opt
			}(),
			expect: "abc,tag\\ 2=def\\ 456,tag1=abc\\ 123 f1=123i 123",
			fail:   false,
		},

		{
			tname: `tag-k/v with '\n' under strict`,
			name:  "abc",
			tags: map[string]string{
				"tag1": `abc
123`,
				`tag
2`: `def
456\`,
			},
			fields: map[string]interface{}{"f1": 123},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			fail: true,
		},

		{
			tname:  `ok case`,
			name:   "abc",
			tags:   nil,
			fields: map[string]interface{}{"f1": 123},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			expect: "abc f1=123i 123",
			fail:   false,
		},

		{
			tname:  `tag key with backslash`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),
			fail: true,
		},

		{
			tname:  `auto fix tag-key, tag-value under non-strict mode`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.Strict = false
				return opt
			}(),
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `under strict: error`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under strict: field is nil`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under strict: field is map`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": map[string]interface{}{"a": "b"}},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under strict: field is object`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": struct{ a string }{a: "abc"}},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under non-strict, ignore nil field`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				opt.Strict = false
				return opt
			}(),

			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `under strict, utf8 characters in metric-name`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: "abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `under strict, utf8 characters in metric-name, fields, tags`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},
			fields: map[string]interface{}{"f1": 123, "f2": "¡™£¢∞§¶•ªº"},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: `abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº" 123`,
			fail:   false,
		},

		{
			tname: `missing field`,
			name:  "abc≈≈≈≈øøππ†®",
			tags:  map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},

			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: `abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº" 123`,
			fail:   true,
		},

		{
			tname: `new line in field`,
			name:  "abc",
			tags:  map[string]string{"tag1": "val1"},
			fields: map[string]interface{}{
				"f1": `aaa
	bbb
			ccc`,
			},
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: `abc,tag1=val1 f1="aaa
	bbb
			ccc" 123`,
		},
	}

	for i, tc := range cases {
		t.Run(tc.tname, func(t *testing.T) {
			pt, warnings, err := MakeLineProtoPointWithWarnings(tc.name, tc.tags, tc.fields, tc.opt)

			if len(tc.warnTypes) == 0 {
				testutil.Equals(t, 0, len(warnings))
			}
			for _, warnType := range tc.warnTypes {
				isFound := false
				for _, w := range warnings {
					if w.WarningType == warnType {
						isFound = true
						break
					}
				}
				if isFound {
					continue
				} else {
					t.Fail()
					t.Logf("[%d]: expected warning type %s, but not found", i, warnType)
				}
			}
			if tc.fail {
				testutil.NotOk(t, err, "")
				t.Logf("[%d] expect error: %s", i, err)
			} else {
				testutil.Ok(t, err)
				x := pt.String()
				testutil.Equals(t, tc.expect, x)
				_, err := parseLineProto(t, []byte(x), "n")
				testutil.Equals(t, err, nil)
				fmt.Printf("\n[%d]%s\n", i, x)
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
		opt    *Option
		expect []*influxdb.Point
		fail   bool
	}{
		{
			name: `32mb-field`,
			data: []byte(fmt.Sprintf(`abc f1="%s" 123`, __32mbString)),
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxFieldValueLen = 32 * 1024 * 1024
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

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
			opt: func() *Option {
				opt := NewDefaultOption()
				opt.MaxFieldValueLen = 0
				opt.Time = time.Unix(0, 123)
				return opt
			}(),

			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": __65kbString},
					time.Unix(0, 123)),
			},
		},

		{
			name: `with disabled field`,
			data: []byte(`abc,t1=1,t2=2 f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{DisabledFieldKeys: []string{"f1"}},
			fail: true,
		},

		{
			name: `with disabled tags`,
			data: []byte(`abc,t1=1,t2=2 f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{DisabledTagKeys: []string{"t1"}},
			fail: true,
		},

		{
			name: `exceed max tags`,
			data: []byte(`abc,t1=1,t2=2 f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123), MaxTags: 1},
			fail: true,
		},

		{
			name: `exceed max fields`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123), MaxFields: 2},
			fail: true,
		},

		{
			name: `tag key with .`,
			data: []byte(`abc,tag.1=xxx f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			fail: true,
		},

		{
			name: `field key with .`,
			data: []byte(`abc f.1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			fail: true,
		},

		{
			name: `with comments`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123
# some comments
abc f1=1i,f2=2,f3="abc" 456
							# other comments with leading spaces
abc f1=1i,f2=2,f3="abc" 789

			`),
			opt: &Option{Time: time.Unix(0, 123)},
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
			name: `same key in field and tag, dup tag comes from ExtraTags`,
			data: []byte(`abc b="abc",a=1i 123`),
			opt:  &Option{Time: time.Unix(0, 123), ExtraTags: map[string]string{"a": "456"}}, // dup tag from Option
			fail: true,
		},

		{
			name: `same key in tags and fields`,
			data: []byte(`abc,b=abc a=1i,b="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			fail: true,
		},

		{
			name: `same key in fields`,
			data: []byte(`abc,b=abc a=1i,c="abc",c=f 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			fail: true,
		},

		{
			name: `same key in tag`,
			data: []byte(`abc,b=abc,b=xyz a=1i 123`),
			fail: true,
		},

		{
			name: `empty data`,
			data: nil,
			fail: true,
		},

		{
			name: "normal case",
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag1": "1", "tag2": "2"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `no tags`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `multiple empty lines in body`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123

abc f1=1i,f2=2,f3="abc" 456

abc f1=1i,f2=2,f3="abc" 789

			`),
			opt: &Option{Time: time.Unix(0, 123)},
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
			name: `no fields`,
			fail: true,
			data: []byte(`abc,tag1=1,tag2=2 123123`),
		},

		{
			name: `parse with extra tags`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{"tag1": "1", "tag2": "2"},
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag1": "1", "tag2": "2"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `extra tag key with '\' suffix`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{`tag1\`: `1`, "tag2": `2`},
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag1\`: "1", "tag2": `2`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `extra tag val with '\' suffix`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{`tag1`: `1,`, "tag2": `2\`, "tag3": `3`},
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag1`: "1,", "tag2": `2\`, "tag3": `3`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `extra tag kv with '\'`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{`tag\1`: `1`, "tag2": `2\34`},
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag\1`: "1", "tag2": `2\34`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			name: `tag kv with '\': missing tag value`,
			fail: true,
			data: []byte(`abc,tag1\=1,tag2=2\ f1=1i 123123`),
		},

		{
			name: `parse with callback: no point`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time: time.Unix(0, 123),
				Callback: func(p models.Point) (models.Point, error) {
					return nil, nil
				},
			},
			fail: true,
		},

		{
			name: `parse with callback failed`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time: time.Unix(0, 123),
				Callback: func(p models.Point) (models.Point, error) {
					return nil, fmt.Errorf("callback failed")
				},
			},
			fail: true,
		},

		{
			name: `parse with callback`,
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time: time.Unix(0, 123),
				Callback: func(p models.Point) (models.Point, error) {
					if string(p.Name()) == "abc" {
						t.Logf("haha, we get measurement `abc'")
					}
					p.AddTag("callback-added-tag", "callback-added-tag-value")
					return p, nil
				},
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"callback-added-tag": "callback-added-tag-value"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pts, err := ParsePoints(tc.data, tc.opt)
			if tc.fail {
				testutil.NotOk(t, err, "")
				t.Logf("expect error: %s", err)
			} else {
				testutil.Ok(t, err)

				for idx, pt := range pts {
					if len(tc.expect) > 0 {
						_ = idx

						got := pt.String()

						pts, err := parseLineProto(t, []byte(got), "n")
						if err != nil {
							t.Logf("parseLineProto failed")
							continue
						}

						for _, pt := range pts {
							fields, err := pt.Fields()
							testutil.Ok(t, err)

							for k, v := range fields {
								switch x := v.(type) {
								case string:
									t.Logf("%s: %s", k, cliutils.StringTrim(x, 32))
								default:
									t.Logf("%s: %v", k, x)
								}
							}
						}
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
			name: `nil data`,
			data: nil,
			prec: "n",
			fail: true,
		},

		{
			name: `no data`,
			data: []byte(""),
			prec: "n",
			fail: true,
		},

		{
			name: `with multiple empty lines`,
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"

		`),
			prec: "n",
		},

		{
			name: `missing field`,
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
		abc
		`),
			prec: "n",
			fail: true,
		},

		{
			name: `missing tag`,
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
				testutil.NotOk(t, err, "")
				t.Logf("expect error: %s", cliutils.LeftStringTrim(err.Error(), 64))
			} else {
				testutil.Ok(t, err)
			}

			if tc.check != nil {
				testutil.Ok(t, tc.check(pts))
			}
		})
	}
}
