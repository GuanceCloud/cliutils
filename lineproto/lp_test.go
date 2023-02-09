// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package lineproto

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/testutil"
	influxdb "github.com/influxdata/influxdb1-client/v2"
)

func TestUnsafeBytesToString(t *testing.T) {
	runes := []byte{'h', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd', '!'}
	fmt.Printf("%p\n", runes)
	s := unsafeBytesToString(runes)
	runes = []byte{'g', 'o', 'l', 'a', 'n', 'g'}
	fmt.Printf("%p\n", runes)
	runtime.GC()
	testutil.Equals(t, s, "hello world!")
}

func TestInterfaceToValue(t *testing.T) {
	ints := []interface{}{
		-5,
		int8(-5),
		int16(-5),
		int32(-5),
		int64(-5),

		uint(5),
		uint8(5),
		uint16(5),
		uint32(5),
		uint64(5),

		float32(3.14),
		3.14,

		true,
		"hello world",
		[]byte{'f', 'o', 'o'},

		[]string{"hello", "world"},
		map[string]int{"foo": 333333, "bar": 44444},

		struct {
			Name string
			Age  int
		}{"foobar", 16},
	}

	for _, i := range ints {
		v, ok := InterfaceToValue(i)
		if !ok {
			t.Errorf("can not new value from interface: %T, %v", i, i)
		}
		t.Logf("%T, %v -> %v", i, i, v)
	}
}

func TestPointString(t *testing.T) {
	tags := map[string]string{
		"t1": cliutils.CreateRandomString(100),
		"t2": cliutils.CreateRandomString(100),
	}
	fields := map[string]interface{}{
		"f1": int64(1024),
		"f2": 1024.2048,
		"f3": cliutils.CreateRandomString(128),
	}

	pt, _ := NewPoint("foobar", tags, fields, time.Now())
	line, err := pt.String()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("[%s]\n", line)
}

func TestParse(t *testing.T) {
	lines := `


error,t1=tag1,t2=tag2 f1=1.0,f2=2i,f3="abc"
   view,t1=tag2,t2=tag2 f1=1.0,f2=2i,f3="abc" 1625823259000000000
 resource,t1=tag3,t2=tag2 f1=1.0,f2=2i,f3="abc" 1621239130
  long_task,t1=tag4,t2=tag2 f1=1.0,f2=2i,f3="abc"
 action,t1=tag5,t2=tag2 f1=1.0,f2=2i,f3="abc"



`

	pts, err := Parse([]byte(lines), DefaultOption)
	if err != nil {
		t.Error(err)
	}

	if len(pts) != 5 {
		t.Errorf("expected %d points, actually get %d points", 5, len(pts))
	}

	for _, pt := range pts {
		t.Logf("%+#v", pt)
	}
}

func BenchmarkParse(b *testing.B) {
	cases := []struct {
		name       string
		data       []byte
		opt        *Option
		optSetters []OptionSetter
	}{
		{
			name: "parse-multi-short",
			data: []byte(strings.Join([]string{
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
			}, "\n")),
			opt:        &Option{PrecisionV2: Microsecond},
			optSetters: []OptionSetter{WithPrecisionV2(Microsecond)},
		},

		{
			name: "parse-multi-long-with-32k-field",
			data: []byte(strings.Join([]string{
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
			}, "\n")),
			opt:        &Option{PrecisionV2: Microsecond},
			optSetters: []OptionSetter{WithPrecisionV2(Microsecond)},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name+"-New", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				pts, err := ParseWithOptionSetter(tc.data, tc.optSetters...)
				if err != nil {
					b.Errorf("Parse: %s", err.Error())
				}
				_ = pts
			}
		})

		b.Run(tc.name+"-Old", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				pts, err := ParsePoints(tc.data, tc.opt)
				if err != nil {
					b.Errorf("Parse: %s", err.Error())
				}
				_ = pts
			}
		})
	}
}

func TestNewLineEncoder(t *testing.T) {
	cases := []struct {
		name string
		pts  []struct {
			measurement string
			tags        map[string]string
			fields      map[string]interface{}
			time        time.Time
		}
		opt *Option
	}{
		{
			name: "encode-multi-short",
			opt:  &Option{PrecisionV2: Microsecond},
			pts: []struct {
				measurement string
				tags        map[string]string
				fields      map[string]interface{}
				time        time.Time
			}{
				{
					measurement: "foo",
					tags: map[string]string{
						"t2": cliutils.CreateRandomString(100),
						"t1": cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f3": int64(1024),
						"f1": 1024.2048,
						"f2": cliutils.CreateRandomString(128),
					},
					time: time.Now(),
				},
				{
					measurement: "foo2",
					tags: map[string]string{
						"aaa": cliutils.CreateRandomString(110),
						"AAA": cliutils.CreateRandomString(110),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f5": 1024.2048,
						"f3": cliutils.CreateRandomString(138),
					},
					time: time.Now(),
				},

				{
					measurement: "foo3",
					tags: map[string]string{
						"t31": cliutils.CreateRandomString(100),
						"t22": cliutils.CreateRandomString(120),
					},
					fields: map[string]interface{}{
						"f1":   int64(1024),
						"f222": 1024.2048,
						"f3":   cliutils.CreateRandomString(148),
					},
					time: time.Now(),
				},
			},
		},
	}

	encoder := NewLineEncoder()

	var point *Point
	for _, tc := range cases {
		for _, pt := range tc.pts {
			newPoint, err := NewPoint(pt.measurement,
				pt.tags,
				pt.fields,
				pt.time)
			if err != nil {
				t.Fatal(err)
			}
			point = newPoint

			err = encoder.AppendPoint(newPoint)
			if err != nil {
				t.Fatal(err)
			}

			bts, _ := encoder.Bytes()
			fmt.Println(len(bts))
		}
	}

	chars, _ := encoder.Bytes()

	fmt.Println(string(chars))

	lineBytes, err := encoder.BytesWithoutLn()
	if err != nil {
		t.Fatal(err)
	}

	fmt.Printf("[%s]\n", lineBytes)

	encoder.SetBuffer(chars[807:])
	if err := encoder.AppendPoint(point); err != nil {
		t.Fatal(err)
	}

	chars, _ = encoder.Bytes()

	fmt.Println(string(chars))

	encoder = NewLineEncoder()
	chars, err = encoder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(chars)
	str, err := encoder.UnsafeString()
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("%q\n", str)
}

func TestEncode(t *testing.T) {
	cases := []struct {
		name string
		pts  []struct {
			measurement string
			tags        map[string]string
			fields      map[string]interface{}
			time        time.Time
		}
		opt *Option
	}{
		{
			name: "encode-multi-short",
			opt:  &Option{PrecisionV2: Microsecond},
			pts: []struct {
				measurement string
				tags        map[string]string
				fields      map[string]interface{}
				time        time.Time
			}{
				{
					measurement: "foo",
					tags: map[string]string{
						"tttttt": "",
						"t1":     cliutils.CreateRandomString(100),
						"t2":     cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f2": 1024.2048,
						"f3": cliutils.CreateRandomString(128),
						"f4": "",
					},
					time: time.Now(),
				},
				{
					measurement: "foo2",
					tags: map[string]string{
						"tttt": "",
						"t1":   cliutils.CreateRandomString(100),
						"t2":   cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"ffff": "",
						"f1":   int64(1024),
						"f2":   1024.2048,
						"f3":   cliutils.CreateRandomString(128),
					},
					time: time.Now(),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name+"-New", func(t *testing.T) {
			pts := make([]*Point, 0, len(tc.pts))
			for _, pt := range tc.pts {
				newPoint, err := NewPoint(pt.measurement,
					pt.tags,
					pt.fields,
					pt.time)
				if err != nil {
					t.Error(err)
					return
				}

				pts = append(pts, newPoint)
			}

			lines, err := Encode(pts)
			if err != nil {
				t.Error(err)
			} else {
				t.Logf("[%s]", string(lines))
			}
		})
	}
}

func BenchmarkEncode(b *testing.B) {
	cases := []struct {
		name string
		pts  []struct {
			measurement string
			tags        map[string]string
			fields      map[string]interface{}
			time        time.Time
		}
		opt *Option
	}{
		{
			name: "encode-multi-short",
			opt:  &Option{PrecisionV2: Microsecond},
			pts: []struct {
				measurement string
				tags        map[string]string
				fields      map[string]interface{}
				time        time.Time
			}{
				{
					measurement: "foo",
					tags: map[string]string{
						"t1": cliutils.CreateRandomString(100),
						"t2": cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f2": 1024.2048,
						"f3": cliutils.CreateRandomString(128),
					},
					time: time.Now(),
				},
			},
		},

		{
			name: "encode-multi-long-with-32k-fields",
			opt:  &Option{PrecisionV2: Microsecond},
			pts: []struct {
				measurement string
				tags        map[string]string
				fields      map[string]interface{}
				time        time.Time
			}{
				{
					measurement: "foo",
					tags: map[string]string{
						"t1": cliutils.CreateRandomString(100),
						"t2": cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f2": 1024.2048,
						"f3": cliutils.CreateRandomString(1024),
						"f4": cliutils.CreateRandomString(32 * 1024),
					},
					time: time.Now(),
				},

				{
					measurement: "foo",
					tags: map[string]string{
						"t1": cliutils.CreateRandomString(100),
						"t2": cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f2": 1024.2048,
						"f3": cliutils.CreateRandomString(1024),
						"f4": cliutils.CreateRandomString(32 * 1024),
					},
					time: time.Now(),
				},

				{
					measurement: "foo",
					tags: map[string]string{
						"t1": cliutils.CreateRandomString(100),
						"t2": cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f2": 1024.2048,
						"f3": cliutils.CreateRandomString(1024),
						"f4": cliutils.CreateRandomString(32 * 1024),
					},
					time: time.Now(),
				},
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name+"-New", func(b *testing.B) {
			encoder := NewLineEncoder()

			for i := 0; i < b.N; i++ {
				for _, pt := range tc.pts {
					newPoint, err := NewPoint(pt.measurement,
						pt.tags,
						pt.fields,
						pt.time)
					if err != nil {
						b.Error(err)
						return
					}
					encoder.Reset()
					_ = encoder.AppendPoint(newPoint)
					_, err = encoder.Bytes()
					if err != nil {
						b.Error(err)
						return
					}
				}
			}
		})

		b.Run(tc.name+"-Old", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				for _, pt := range tc.pts {
					oldPoint, err := influxdb.NewPoint(pt.measurement,
						pt.tags,
						pt.fields,
						pt.time)
					if err != nil {
						b.Error(err)
						return
					}

					_ = oldPoint.String()
				}
			}
		})
	}
}

func TestLineEncoderBytesWithoutLn(t *testing.T) {
	cases := []struct {
		name string
		pts  []struct {
			measurement string
			tags        map[string]string
			fields      map[string]interface{}
			time        time.Time
		}
		opt *Option
	}{
		{
			name: "encode-multi-short",
			opt:  &Option{PrecisionV2: Microsecond},
			pts: []struct {
				measurement string
				tags        map[string]string
				fields      map[string]interface{}
				time        time.Time
			}{
				{
					measurement: "foo",
					tags: map[string]string{
						"t2": cliutils.CreateRandomString(100),
						"t1": cliutils.CreateRandomString(100),
					},
					fields: map[string]interface{}{
						"f3": int64(1024),
						"f1": 1024.2048,
						"f2": cliutils.CreateRandomString(128),
					},
					time: time.Now(),
				},
				{
					measurement: "foo2",
					tags: map[string]string{
						"aaa": cliutils.CreateRandomString(110),
						"AAA": cliutils.CreateRandomString(110),
					},
					fields: map[string]interface{}{
						"f1": int64(1024),
						"f5": 1024.2048,
						"f3": cliutils.CreateRandomString(138),
					},
					time: time.Now(),
				},

				{
					measurement: "foo3",
					tags: map[string]string{
						"t31": cliutils.CreateRandomString(100),
						"t22": cliutils.CreateRandomString(120),
					},
					fields: map[string]interface{}{
						"f1":   int64(1024),
						"f222": 1024.2048,
						"f3":   cliutils.CreateRandomString(148),
					},
					time: time.Now(),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoder := NewLineEncoder()

			for _, pt := range tc.pts {
				encoder.Reset()
				point, _ := NewPoint(pt.measurement, pt.tags, pt.fields, pt.time)
				if err := encoder.AppendPoint(point); err != nil {
					t.Fatal(err)
				}
				line, err := encoder.BytesWithoutLn()
				if err != nil {
					t.Fatal(err)
				}

				t.Logf("%q\n", string(line))
			}
		})
	}
}
