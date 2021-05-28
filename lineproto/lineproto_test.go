package lineproto

import (
	"fmt"
	"testing"
	"time"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"

	"github.com/influxdata/influxdb1-client/models"
	influxdb "github.com/influxdata/influxdb1-client/v2"
)

func TestMakeLineProtoPoint(t *testing.T) {
	var cases = []struct {
		name   string
		tags   map[string]string
		fields map[string]interface{}
		ts     time.Time
		opt    *Option
		expect string
		fail   bool
	}{
		{
			name:   "abc",
			tags:   nil,
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123)},
			expect: "abc f1=123i 123",
			fail:   false,
		},

		{
			name:   "abc",
			tags:   nil,
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123)},
			expect: "abc f1=123i 123",
			fail:   false,
		},

		{
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			fail:   true,
		},

		{ // under non-strict: auto fix tag-key, tag-value
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: false},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{ // under strict: error
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{ // under strict: field is nil
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{ // under strict: field is map
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": map[string]interface{}{"a": "b"}},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{ // under strict: field is object
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123, "f2": struct{ a string }{a: "abc"}},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   true,
		},

		{ // under non-strict, ignore nil field
			name:   "abc",
			tags:   map[string]string{"tag1": "val1", `tag2\`: `val2\`},
			fields: map[string]interface{}{"f1": 123, "f2": nil},
			opt:    &Option{Time: time.Unix(0, 123), Strict: false},
			expect: "abc,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{ // under strict, utf8 characters in metric-name
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`},
			fields: map[string]interface{}{"f1": 123},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: "abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2 f1=123i 123",
			fail:   false,
		},

		{ // under strict, utf8 characters in metric-name, fields, tags
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},
			fields: map[string]interface{}{"f1": 123, "f2": "¡™£¢∞§¶•ªº"},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: `abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº" 123`,
			fail:   false,
		},

		{ // missing field
			name:   "abc≈≈≈≈øøππ†®",
			tags:   map[string]string{"tag1": "val1", `tag2`: `val2`, "tag3": `ºª•¶§∞¢£`},
			opt:    &Option{Time: time.Unix(0, 123), Strict: true},
			expect: `abc≈≈≈≈øøππ†®,tag1=val1,tag2=val2,tag3=ºª•¶§∞¢£ f1=123i,f2="¡™£¢∞§¶•ªº" 123`,
			fail:   true,
		},

		{ // new line in field
			name: "abc",
			tags: map[string]string{"tag1": "val1"},
			fields: map[string]interface{}{"f1": `aaa
bbb
ccc`},
			opt: &Option{Time: time.Unix(0, 123), Strict: true},
			expect: `abc,tag1=val1 f1="aaa
bbb
ccc" 123`,
		},
	}

	for i, tc := range cases {
		pt, err := MakeLineProtoPoint(tc.name, tc.tags, tc.fields, tc.opt)

		if tc.fail {
			testutil.NotOk(t, err, "")
			t.Logf("[%d] expect error: %s", i, err)
		} else {
			testutil.Ok(t, err)
			x := pt.String()
			testutil.Equals(t, tc.expect, x)
			testutil.Equals(t, ParseLineProto([]byte(x), "n"), nil)
			fmt.Printf("\n[%d]%s\n", i, x)
		}
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
		data   []byte
		opt    *Option
		expect []*influxdb.Point
		fail   bool
	}{
		{
			data: nil,
			fail: true,
		},
		{
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
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`), // no tags
			opt:  &Option{Time: time.Unix(0, 123)},
			expect: []*influxdb.Point{
				newPoint("abc",
					nil,
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123

abc f1=1i,f2=2,f3="abc" 456

abc f1=1i,f2=2,f3="abc" 789

			`), // multiple empty lines
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
			fail: true,
			data: []byte(`abc,tag1=1,tag2=2 123123`), // no fields
		},

		{
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
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{`tag1\`: `1`, "tag2": `2`}, // extra tag key with `\` suffix
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag1\`: "1", "tag2": `2`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{`tag1`: `1,`, "tag2": `2\`, "tag3": `3`}, // extra tag val with `\` suffix
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag1`: "1,", "tag2": `2\`, "tag3": `3`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`),
			opt: &Option{
				Time:      time.Unix(0, 123),
				ExtraTags: map[string]string{`tag\1`: `1`, "tag2": `2\34`}, // extra tag kv with `\`
			},
			expect: []*influxdb.Point{
				newPoint("abc",
					map[string]string{`tag\1`: "1", "tag2": `2\34`},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			fail: true,
			data: []byte(`abc,tag1\=1,tag2=2\ f1=1i 123123`), // tag kv with `\`: missing tag value
		},

		{
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
					t.Logf("[pass] exp: %s, parse ok? %v", exp, ParseLineProto([]byte(exp), "n"))
				}
			}
		}
	}
}

func TestParseLineProto(t *testing.T) {

	var cases = []struct {
		data []byte
		prec string
		fail bool
	}{
		{
			data: nil,
			prec: "n",
			fail: true,
		},

		{
			data: []byte(""),
			prec: "n",
			fail: true,
		},

		{
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"

`), // with multiple empty lines
			prec: "n",
		},

		{
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc
`), // missing field
			prec: "n",
			fail: true,
		},

		{
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc"
abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc" 123456789
abc f1=1i,f2=2,f3="abc"
`), // missing tag: ok
			prec: "n",
		},
	}

	for _, tc := range cases {
		err := ParseLineProto(tc.data, tc.prec)
		if tc.fail {
			testutil.NotOk(t, err, "")
			t.Logf("expect error: %s", err)
		} else {
			testutil.Ok(t, err)
		}
	}
}
