// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

var name = "abc"

func BenchmarkMarshal(b *testing.B) {
	cases := []struct {
		name   string
		repeat int
	}{
		{
			name:   "10-points",
			repeat: 10,
		},

		{
			name:   "1000-points",
			repeat: 1000,
		},
	}

	for _, tc := range cases {
		b.Run(tc.name+"_pb-marshal", func(b *testing.B) {
			pbpts := RandPBPoints(tc.repeat)

			for i := 0; i < b.N; i++ {
				if _, err := proto.Marshal(pbpts); err != nil {
					b.Error(err)
				}
			}
		})

		b.Run(tc.name+"_lp-marshal", func(b *testing.B) {
			pts := RandPoints(tc.repeat)
			for i := 0; i < b.N; i++ {
				arr := []string{}
				for i := 0; i < len(pts); i++ {
					arr = append(arr, pts[i].LineProto())
				}
				x := strings.Join(arr, "\n")
				_ = x
			}
		})

		b.Logf("----------------------------------------------")

		b.Run(tc.name+"_pb-unmarshal", func(b *testing.B) {
			pbpts := RandPBPoints(tc.repeat)

			pb, err := proto.Marshal(pbpts)
			assert.NoError(b, err)

			for i := 0; i < b.N; i++ {
				var pt PBPoints
				if err := proto.Unmarshal(pb, &pt); err != nil {
					b.Error(err)
				}
			}
		})

		b.Run(tc.name+"_lp-unmarshal", func(b *testing.B) {
			pts := RandPoints(tc.repeat)
			arr := []string{}
			for i := 0; i < len(pts); i++ {
				arr = append(arr, pts[i].LineProto())
			}
			ptbytes := []byte(strings.Join(arr, "\n"))

			for i := 0; i < b.N; i++ {
				if _, err := parseLPPoints(ptbytes, nil); err != nil {
					b.Error(err)
				}
			}
		})
	}
}

func TestPBPointJSON(t *testing.T) {
	cases := []struct {
		name   string
		tags   map[string]string
		fields map[string]interface{}
		time   time.Time
		warns  []*Warn
		debugs []*Debug
	}{
		{
			name: "simple",
			tags: nil,
			fields: map[string]interface{}{
				"f1": int64(123),
				"f2": 123.4,
				"f3": false,
				"f4": "abc",
				"f5": []byte("xyz"),
				"f6": uint64(1234567890),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "simple-2",
			tags: map[string]string{
				"t1": "123",
				"t2": "xyz",
			},
			fields: map[string]interface{}{
				"f1": int64(123),
				"f2": 123.4,
				"f3": false,
				"f4": "abc",
				"f5": []byte("xyz"),
				"f6": uint64(1234567890),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "with-anypb",
			tags: map[string]string{
				"t1": "123",
				"t2": "xyz",
			},
			fields: map[string]interface{}{
				"f1": int64(123),
				"f2": 123.4,
				"f3": false,
				"f4": "abc",
				"f5": []byte("xyz"),
				"f6": uint64(1234567890),
				"f7": func() *anypb.Any {
					x, err := anypb.New(&AnyDemo{Demo: "this is a any field"})
					if err != nil {
						t.Errorf("anypb.New: %s", err)
					}
					return x
				}(),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "with-warnings",
			tags: map[string]string{
				"t1": "123",
			},
			fields: map[string]interface{}{
				"t1": "dulicated key in tags", // triger warnning
				"f1": int64(123),
				"f2": func() *anypb.Any {
					x, err := anypb.New(&AnyDemo{Demo: "this is a any field"})
					if err != nil {
						t.Errorf("anypb.New: %s", err)
					}
					return x
				}(),
			},
			time: time.Unix(0, 123),
		},

		{
			name: "with-debugs",
			tags: map[string]string{
				"t1": "123",
			},
			fields: map[string]interface{}{
				"t1": "dulicated key in tags",
				"f1": int64(123),
				"f2": func() *anypb.Any {
					x, err := anypb.New(&AnyDemo{Demo: "this is a any field"})
					if err != nil {
						t.Errorf("anypb.New: %s", err)
					}
					return x
				}(),
			},
			time:   time.Unix(0, 123),
			debugs: []*Debug{{Info: "some debug info"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pt, err := NewPoint(tc.name, tc.tags, tc.fields,
				WithEncoding(Protobuf), WithTime(tc.time))

			assert.NoError(t, err)

			for _, d := range tc.debugs {
				pt.AddDebug(d)
			}

			// json
			pbjson, err := json.Marshal(pt)
			assert.NoError(t, err)

			// test if debug/warns included in json
			if len(pt.debugs) > 0 {
				assert.Contains(t, string(pbjson), "debugs", "%s not include `debugs'", string(pbjson))
			}

			if len(pt.warns) > 0 {
				assert.Contains(t, string(pbjson), "warns", "%s not include `warns'", string(pbjson))
			}

			// unmarshal
			var unmarshalPt Point
			assert.NoError(t, json.Unmarshal(pbjson, &unmarshalPt))

			// json unmarshaled point should equal to origin point
			assert.True(t, pt.Equal(&unmarshalPt))

			// encode to pb
			enc := GetEncoder(WithEncEncoding(Protobuf))
			defer PutEncoder(enc)

			batches, err := enc.Encode([]*Point{&unmarshalPt})
			assert.NoError(t, err)
			assert.Equal(t, 1, len(batches))

			// decode the pb
			// test equality: decoded pb point equal(json format) the point before encode
			dec := GetDecoder(WithDecEncoding(Protobuf))

			pts, err := dec.Decode(batches[0])
			assert.NoError(t, err)
			assert.Equal(t, 1, len(pts))

			// test equality on origin json
			assert.Equal(t, string(pbjson),
				func() string {
					j, err := json.Marshal(pts[0])
					assert.NoError(t, err)
					return string(j)
				}())

			t.Logf("pb json after:\n%s", pts[0].Pretty())

			if len(pt.warns) > 0 {
				assert.Contains(t, string(pbjson), "warns")
			}

			if len(tc.debugs) > 0 {
				assert.Contains(t, string(pbjson), "debugs")
			}
		})
	}
}

func TestPBPointPayload(t *testing.T) {
	cases := []struct {
		name   string
		repeat int
	}{
		{
			name:   "100-point",
			repeat: 100,
		},

		{
			name:   "1000-point",
			repeat: 1000,
		},

		{
			name:   "10000-point",
			repeat: 10000,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lppts := RandPoints(tc.repeat)
			pbpts := RandPBPoints(tc.repeat)

			pb, err := proto.Marshal(pbpts)
			assert.NoError(t, err)
			t.Logf("pb len: %d", len(pb))

			t.Logf("pb len(gz): %f", func() float64 {
				x, err := cliutils.GZip(pb)
				assert.NoError(t, err)
				return float64(len(x)) / float64(len(pb))
			}())

			// line-protocol
			ptStrArr := []string{}
			for i := 0; i < tc.repeat; i++ {
				str := lppts[i].LineProto()

				ptStrArr = append(ptStrArr, str)
			}

			ptstr := strings.Join(ptStrArr, "\n")

			t.Logf("lp len: %d", len(ptstr))
			t.Logf("lp len(gz): %f", func() float64 {
				x, err := cliutils.GZipStr(ptstr)
				assert.NoError(t, err)
				return float64(len(x)) / float64(len(ptstr))
			}())
		})
	}
}
