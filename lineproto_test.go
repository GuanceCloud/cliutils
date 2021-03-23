package cliutils

import (
	"testing"
	"time"

	"gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"

	influxdb "github.com/influxdata/influxdb1-client/v2"
)

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
		data     []byte
		opt      *Option
		expected []*influxdb.Point
		fail     bool
	}{
		{
			data: nil,
			fail: true,
		},
		{
			data: []byte(`abc,tag1=1,tag2=2 f1=1i,f2=2,f3="abc" 123`),
			opt:  &Option{Time: time.Unix(0, 123)},
			expected: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag1": "1", "tag2": "2"},
					map[string]interface{}{"f1": 1, "f2": 2.0, "f3": "abc"},
					time.Unix(0, 123)),
			},
		},

		{
			data: []byte(`abc f1=1i,f2=2,f3="abc" 123`), // no tags
			opt:  &Option{Time: time.Unix(0, 123)},
			expected: []*influxdb.Point{
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
			expected: []*influxdb.Point{
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
			expected: []*influxdb.Point{
				newPoint("abc",
					map[string]string{"tag1": "1", "tag2": "2"},
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
				exp := tc.expected[idx].String()
				got := pt.String()
				testutil.Equals(t, exp, got)
				t.Logf("exp: %s", exp)
				t.Logf("got: %s", got)
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
