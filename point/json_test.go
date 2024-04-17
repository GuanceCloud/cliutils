// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	"fmt"
	"testing"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONPointMarshal(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
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

func TestJSONUnmarshal(t *T.T) {
	t.Run(`unmarshal-point-array`, func(t *T.T) {
		j := `[
{"name":"abc","fields":[{"key":"f1","i":"123"},{"key":"f2","b":false},{"key":"t1","s":"tv1","is_tag":true},{"key":"t2","s":"tv2","is_tag":true}],"time":"123"}
]`
		pts := make([]*Point, 0)
		require.NoError(t, json.Unmarshal([]byte(j), &pts))
		t.Logf("pt: %s", pts[0].Pretty())
	})
}

func TestJSONPoint2Point(t *testing.T) {
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
		t.Run(tc.name, func(t *testing.T) {
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

	t.Run("array-field", func(t *T.T) {
		jp := JSONPoint{
			Measurement: "some",
			Fields: map[string]any{
				"f_i_arr": []int{1, 2, 3},
				"f_d_arr": [][]byte{[]byte("hello"), []byte("world")},
			},
		}

		pt, err := jp.Point()
		assert.NoError(t, err)
		t.Logf("pt: %s", pt.Pretty())

		EnableMixedArrayField = true
		defer func() {
			EnableMixedArrayField = false
		}()

		j := `{
	"measurement": "some",
	"fields": {
		"f_i_arr": [1,2,3],
		"f_f_arr": [1.0,2.0,3.14],
		"f_mix_arr": [1.0, "string", false, 3]
	}
}`
		// NOTE: simple json do not support:
		//  - signed/unsigned int
		//  - []byte
		var jp2 JSONPoint
		assert.NoError(t, json.Unmarshal([]byte(j), &jp2))

		t.Logf("jp2 fields: %+#v", jp2.Fields)

		pt, err = jp2.Point()
		assert.NoError(t, err)
		assert.NotNil(t, pt.Get("f_i_arr"))
		assert.NotNil(t, pt.Get("f_f_arr"))
		assert.NotNil(t, pt.Get("f_mix_arr"))

		t.Logf("pt: %s", pt.Pretty())
	})
}
