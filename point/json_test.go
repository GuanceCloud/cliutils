// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	"fmt"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPointMarshal(t *T.T) {
	cases := []struct {
		name   string
		p      *Point
		opts   []Option
		expect string
	}{
		{
			name: "basic",
			opts: []Option{WithTime(time.Unix(0, 123))},
			p: func() *Point {
				pt, err := NewPoint("abc",
					map[string]string{"t1": "tv1", "t2": "tv2"},
					map[string]interface{}{"f1": 123, "f2": false},
					WithTime(time.Unix(0, 123)))
				assert.NoError(t, err)

				pt.SetFlag(Ppb) // setup pb

				return pt
			}(),

			expect: fmt.Sprintf(`{
				"name":"%s",
				"fields":[
					{"key":"%s","is_tag": true, "s":"%s"},
					{"key":"%s","is_tag": true, "s":"%s"},
				  {"key":"%s","i":"123"},
				  {"key":"%s","b":false}
				],
				"time":"123"
			}`,
				`abc`,
				`t2`,
				`tv2`,
				`t1`,
				`tv1`,
				`f1`,
				`f2`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			j, err := json.Marshal(tc.p)
			assert.NoError(t, err, "marshal %s to json failed: %s, json: %v", tc.p.Pretty(), err, j)

			t.Logf("json: %s", string(j))

			var pt Point
			require.NoError(t, json.Unmarshal(j, &pt), "unmarshal %s failed", string(j))

			var pt2 Point
			require.NoError(t, json.Unmarshal([]byte(tc.expect), &pt2))

			require.True(t, pt2.Equal(&pt), "deep-not-equal:\n%s\n%s", pt.Pretty(), pt2.Pretty())

			t.Logf("pt: %+#v, pt.pbPoint: %s", pt2, pt.Pretty())
		})
	}
}

func TestJSONPointMarhsal(t *T.T) {
	var kvs KVs

	EnableMixedArrayField = true
	defer func() {
		EnableMixedArrayField = false
	}()

	kvs = kvs.AddV2("f1", 123, false)
	kvs = kvs.AddV2("f2", uint(123), false)
	kvs = kvs.AddV2("f3", "hello", false)
	kvs = kvs.AddV2("f4", []byte("world"), false)
	kvs = kvs.AddV2("f5", false, false)
	kvs = kvs.AddV2("f6", 3.14, false, WithKVUnit("kb"), WithKVType(GAUGE))
	kvs = kvs.AddV2("f7", []int{1, 2, 3, 4, 5}, false)
	kvs = kvs.AddV2("f8", MustNewAnyArray(1.0, 2, uint(3), "hello", []byte("world"), false, 3.14), false)

	kvs = kvs.AddV2("t1", "some-tag-value", false, WithKVTagSet(true))

	pt := NewPointV2("json-point", kvs)

	j, err := json.MarshalIndent(pt, "", "  ")
	assert.NoError(t, err)

	t.Logf("old json:\n%s", string(j))

	pt.SetFlag(Ppb)

	j, err = json.MarshalIndent(pt, "", "  ")
	assert.NoError(t, err)

	t.Logf("new json:\n%s", string(j))

	t.Logf("line-protocol:\n%s", pt.LineProto())
}

func TestJSONPoint2Point(t *T.T) {
	cases := []struct {
		name   string
		p      *JSONPoint
		opts   []Option
		expect string
		fail   bool
	}{
		{
			name: "basic",
			p: &JSONPoint{
				Measurement: "abc",
				Tags:        nil,
				Fields:      map[string]interface{}{"f1": 123, "f2": false},
			},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			expect: "abc f1=123i,f2=false 123",
		},

		{
			name: "no-fields",
			p: &JSONPoint{
				Measurement: "abc",
				Fields:      nil,
			},
			fail: true,
			opts: []Option{WithTime(time.Unix(0, 123))},
		},

		{
			name: "no-measurement",
			p: &JSONPoint{
				Measurement: "", // not set
				Tags:        nil,
				Fields:      map[string]interface{}{"f1": 123, "f2": false},
			},
			opts:   []Option{WithTime(time.Unix(0, 123))},
			expect: fmt.Sprintf("%s f1=123i,f2=false 123", DefaultMeasurementName),
		},

		{
			name: "minus-time", // it's ok!
			p: &JSONPoint{
				Measurement: "minus-time",
				Tags:        nil,
				Fields:      map[string]interface{}{"f1": 123, "f2": false},
			},
			opts:   []Option{WithTime(time.Unix(0, -123))},
			expect: "minus-time f1=123i,f2=false -123",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *T.T) {
			pt, err := tc.p.Point(tc.opts...)
			if tc.fail {
				assert.Error(t, err)
				t.Logf("expect err: %s", err)
				return
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expect, pt.LineProto())
		})
	}
}

func TestFromJSONPoint(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		jp := JSONPoint{
			Measurement: "m",
			Tags: map[string]string{
				"t1": "v1",
				"t2": "v2",
			},
			Fields: map[string]any{
				"f1": 1,
				"f2": 3.14,
			},
			Time: 123,
		}

		pt := FromJSONPoint(&jp)
		assert.Equal(t, "m", pt.Name())
		assert.Equal(t, "v1", pt.Get("t1"))
		assert.Equal(t, "v2", pt.Get("t2"))
		assert.Equal(t, int64(1), pt.Get("f1"))
		assert.Equal(t, 3.14, pt.Get("f2"))
		assert.Equal(t, int64(123), pt.Time().UnixNano())
	})

	t.Run(`from-raw-json`, func(t *T.T) {
		j := `
{
	"measurement": "m",
	"tags": {
		"t1": "v1",
		"t2": "v2"
	},
	"fields": {
		"f1": 1,
		"f2": 3.14
	},

	"time": 123
}
`

		var pt Point
		assert.NoError(t, json.Unmarshal([]byte(j), &pt))

		assert.Equal(t, "m", pt.Name())
		assert.Equal(t, "v1", pt.Get("t1"))
		assert.Equal(t, "v2", pt.Get("t2"))
		assert.Equal(t, 1.0, pt.Get("f1")) // NOTE: here 1 in json unmarshaled as float
		assert.Equal(t, 3.14, pt.Get("f2"))
		assert.Equal(t, int64(123), pt.Time().UnixNano())
	})
}
