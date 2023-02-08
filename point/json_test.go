// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	"fmt"
	"testing"
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
					{"key":"%s","is_tag": true, "d":"%s"},
					{"key":"%s","is_tag": true, "d":"%s"},
				  {"key":"%s","i":"123"},
				  {"key":"%s","b":false}
				],
				"time":"123"
			}`, b64([]byte(`abc`)),
				b64([]byte(`t2`)),
				b64([]byte(`tv2`)),
				b64([]byte(`t1`)),
				b64([]byte(`tv1`)),
				b64([]byte(`f1`)),
				b64([]byte(`f2`))),
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
}
