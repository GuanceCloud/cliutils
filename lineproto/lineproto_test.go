package lineproto

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/influxdata/influxdb1-client/models"
	influxdb "github.com/influxdata/influxdb1-client/v2"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"
)

func parseLineProto(data []byte, precision string) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("empty data")
	}

	_, err := models.ParsePointsWithPrecision(data, time.Now().UTC(), precision)
	return err
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

func TestMakeLineProtoPoint(t *testing.T) {
	var cases = []struct {
		tname  string // test name
		name   string
		tags   map[string]string
		fields map[string]interface{}
		ts     time.Time
		opt    *Option
		expect string
		fail   bool
	}{

		{
			tname:  `enable point in metric point`,
			name:   "abc",
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t.1": "abc", "t2": "32"},
			opt: &Option{
				IsMetric: true,
				Time:     time.Unix(0, 123),
			},
			expect: "abc,t.1=abc,t2=32 f.1=1i,f2=32i 123",
		},

		{
			tname:  `enable point in metric point`,
			name:   "abc",
			fields: map[string]interface{}{"f.1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			opt: &Option{
				IsMetric: true,
				Time:     time.Unix(0, 123),
			},
			expect: "abc,t1=abc,t2=32 f.1=1i,f2=32i 123",
		},

		{
			tname:  `with disabled field keys`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			opt: &Option{
				DisabledFieldKeys: []string{"f1"},
			},

			fail: true,
		},

		{
			tname:  `with disabled tag keys`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			tags:   map[string]string{"t1": "abc", "t2": "32"},
			opt: &Option{
				DisabledTagKeys: []string{"t2"},
			},

			fail: true,
		},

		{
			tname:  `int exceed int64-max under non-strict mode`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(32)},
			expect: "abc f1=1i,f2=32i 123", // f2 dropped
			opt: &Option{
				Time: time.Unix(0, 123),
			},

			fail: false,
		},

		{
			tname:  `int exceed int64-max under non-strict mode`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(math.MaxInt64) + 1},
			expect: "abc f1=1i 123", // f2 dropped
			opt: &Option{
				Time: time.Unix(0, 123),
			},

			fail: false,
		},

		{
			tname:  `int exceed int64-max under strict mode`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": uint64(math.MaxInt64) + 1},
			opt: &Option{
				Time:   time.Unix(0, 123),
				Strict: true,
			},

			fail: true,
		},

		{
			tname:  `extra tags and field exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": "3"},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			opt: &Option{
				Time:      time.Unix(0, 123),
				Strict:    true,
				MaxTags:   3,
				MaxFields: 1,
				ExtraTags: map[string]string{
					"etag1": "1",
					"etag2": "2",
				}},

			fail: true,
		},

		{
			tname:  `extra tags exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			opt: &Option{
				Time:    time.Unix(0, 123),
				Strict:  true,
				MaxTags: 3,
				ExtraTags: map[string]string{
					"etag1": "1",
					"etag2": "2",
				}},

			fail: true,
		},

		{
			tname:  `extra tags not exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			expect: "abc,etag1=1,etag2=2,t1=def,t2=abc f1=1i 123",
			opt: &Option{
				Time:    time.Unix(0, 123),
				Strict:  true,
				MaxTags: 4,
				ExtraTags: map[string]string{
					"etag1": "1",
					"etag2": "2",
				}},

			fail: false,
		},

		{
			tname:  `only extra tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			expect: "abc,etag1=1,etag2=2 f1=1i 123",
			opt: &Option{
				Time:    time.Unix(0, 123),
				Strict:  true,
				MaxTags: 4,
				ExtraTags: map[string]string{
					"etag1": "1",
					"etag2": "2",
				}},

			fail: false,
		},

		{
			tname:  `exceed max tags`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def", "t2": "abc"},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true, MaxTags: 1},
			fail:   true,
		},

		{
			tname:  `exceed max field`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def"},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true, MaxFields: 1},
			fail:   true,
		},

		{
			tname:  `field key with "."`,
			name:   "abc",
			fields: map[string]interface{}{"f1.a": 1},
			tags:   map[string]string{"t1.a": "def"},
			fail:   true,
		},

		{
			tname:  `field key with "."`,
			name:   "abc",
			fields: map[string]interface{}{"f1.a": 1},
			tags:   map[string]string{"t1": "def"},
			fail:   true,
		},

		{
			tname:  `tag key with "."`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1.a": "def"},
			fail:   true,
		},

		{
			tname:  `nil field, not allowed`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1, "f2": nil},
			tags:   map[string]string{"t1": "def"},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			fail:   true,
		},

		{
			tname:  `same key in field and tag`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   map[string]string{"f1": "def"},
			fail:   true,
		},

		{
			tname:  `no tag`,
			name:   "abc",
			fields: map[string]interface{}{"f1": 1},
			tags:   nil,
			opt:    &Option{Time: time.Unix(0, 123)},
			expect: "abc f1=1i 123",
		},

		{
			tname:  `no filed`,
			name:   "abc",
			fields: nil,
			tags:   map[string]string{"f1": "def"},
			fail:   true,
		},

		{
			tname: `field-val with '\n'`,
			name:  "abc",
			fields: map[string]interface{}{"f1": `abc
123`},
			opt: &Option{Time: time.Unix(0, 123), Strict: false},
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
456\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: false},
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
456\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			fail:   true,
		},

		{
			tname:  ``,
			name:   "abc",
			tags:   nil,
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123)},
			expect: "abc f1=123i 123",
			fail:   false,
		},

		{
			tname:  `tag keey with backslash`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			fail:   true,
		},

		{
			tname:  `auto fix tag-key, tag-value under non-strict mode`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: false},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `under strict: error`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under strict: field is nil`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under strict: field is map`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": map[string]interface{}{"a": "b"}},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under strict: field is object`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": struct{ a string }{a: "abc"}},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{
			tname:  `under non-strict, ignore nil field`,
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},
			opt:    &Option{Time: time.Unix(0, 123), Strict: false},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `under strict, utf8 characters in metric-name`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{
			tname:  `under strict, utf8 characters in metric-name, fields, tags`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},
			fields: map[string]interface{}{"f1": 123, "f2": "¡™£¢∞§¶•ªº"},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: `abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº" 123`,
			fail:   false,
		},

		{
			tname:  `missing field`,
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
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
			ccc`},
			opt: &Option{Time: time.Unix(0, 123), Strict: true},
			expect: `abc,tag1=val1 f1="aaa
	bbb
			ccc" 123`},
	}

	for i, tc := range cases {
		t.Run(tc.tname, func(t *testing.T) {
			pt, err := MakeLineProtoPoint(tc.name, tc.tags, tc.fields, tc.opt)

			if tc.fail {
				testutil.NotOk(t, err, "")
				t.Logf("[%d] expect error: %s", i, err)
			} else {
				testutil.Ok(t, err)
				x := pt.String()
				testutil.Equals(t, tc.expect, x)
				testutil.Equals(t, parseLineProto([]byte(x), "n"), nil)
				fmt.Printf("\n[%d]%s\n", i, x)
			}
		})
	}
}

func TestParsePoint(t *testing.T) {
	newPoint := func(m string,
		tags map[string]string,
		fields map[string]interface{},
		ts ...time.Time) *influxdb.Point {
		pt, err := influxdb.NewPoint(m, tags, fields, ts...)
		if err != nil {
			t.Fatal(err) // should never been here
		}

		return pt
	}

	var cases = []struct {
		name   string
		data   []byte
		opt    *Option
		expect []*influxdb.Point
		fail   bool
	}{
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
						exp := tc.expect[idx].String()
						got := pt.String()
						testutil.Equals(t, exp, got)
						t.Logf("[pass] exp: %s, parse ok? %v", exp, parseLineProto([]byte(exp), "n"))
					}
				}
			}
		})
	}
}

func TestParseLineProto(t *testing.T) {
	var cases = []struct {
		data []byte
		prec string
		fail bool
		name string
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
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := parseLineProto(tc.data, tc.prec)
			if tc.fail {
				testutil.NotOk(t, err, "")
				t.Logf("expect error: %s", err)
			} else {
				testutil.Ok(t, err)
			}
		})
	}
}
