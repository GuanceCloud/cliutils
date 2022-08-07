package lineproto

import (
	"fmt"
	"strings"
	"testing"
	"time"

	influxdb "github.com/influxdata/influxdb1-client/v2"
	lp "github.com/influxdata/line-protocol/v2/lineprotocol"
	"gitlab.jiagouyun.com/cloudcare-tools/cliutils"
)

func BenchmarkParse(b *testing.B) {
	cases := []struct {
		name string
		data []byte
		opt  *Option
	}{
		{
			name: "parse-multi-short",
			data: []byte(strings.Join([]string{
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
				`foo,tag1=val1,tag2=val2 x=1,y="hello" 1625823259000000`,
			}, "\n")),
			opt: &Option{PrecisionV2: lp.Microsecond},
		},

		{

			name: "parse-multi-long-with-32k-field",
			data: []byte(strings.Join([]string{
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
				fmt.Sprintf(`foo,tag1=%s,tag2=%s f1=1,f2="%s",f3=3i 1625823259000000`, cliutils.CreateRandomString(100), cliutils.CreateRandomString(100), cliutils.CreateRandomString(32*1024)),
			}, "\n")),
			opt: &Option{PrecisionV2: lp.Microsecond},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name+"-New", func(b *testing.B) {

			for i := 0; i < b.N; i++ {
				pts, err := Parse(tc.data, tc.opt)
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

func BenchmarkEncode(b *testing.B) {
	cases := []struct {
		name string
		pts  []*Point
		opt  *Option
	}{
		{
			name: "encode-multi-short",
			opt:  &Option{PrecisionV2: lp.Microsecond},
			pts: []*Point{
				&Point{
					Measurement: []byte("foo"),
					Tags: []*Tag{
						&Tag{
							Key: []byte("t1"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
						&Tag{
							Key: []byte("t2"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
					},
					Fields: []*Field{
						&Field{
							Key: []byte("f1"),
							Val: lp.MustNewValue(int64(1024)),
						},
						&Field{
							Key: []byte("f2"),
							Val: lp.MustNewValue(1024.2048),
						},

						&Field{
							Key: []byte("f3"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(128)),
						},
					},
					Time: time.Now(),
				},
			},
		},

		{
			name: "encode-multi-long-with-32k-fields",
			opt:  &Option{PrecisionV2: lp.Microsecond},
			pts: []*Point{
				&Point{
					Measurement: []byte("foo"),
					Tags: []*Tag{
						&Tag{
							Key: []byte("t1"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
						&Tag{
							Key: []byte("t2"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
					},
					Fields: []*Field{
						&Field{
							Key: []byte("f1"),
							Val: lp.IntValue(int64(1024)),
						},
						&Field{
							Key: []byte("f2"),
							Val: lp.MustNewValue(1024.2048),
						},

						&Field{
							Key: []byte("f3"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(1024)),
						},

						&Field{
							Key: []byte("f4"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(32 * 1024)),
						},
					},
					Time: time.Now(),
				},

				&Point{
					Measurement: []byte("foo"),
					Tags: []*Tag{
						&Tag{
							Key: []byte("t1"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
						&Tag{
							Key: []byte("t2"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
					},
					Fields: []*Field{
						&Field{
							Key: []byte("f1"),
							Val: lp.IntValue(int64(1024)),
						},
						&Field{
							Key: []byte("f2"),
							Val: lp.MustNewValue(1024.2048),
						},

						&Field{
							Key: []byte("f3"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(1024)),
						},

						&Field{
							Key: []byte("f4"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(32 * 1024)),
						},
					},
					Time: time.Now(),
				},

				&Point{
					Measurement: []byte("foo"),
					Tags: []*Tag{
						&Tag{
							Key: []byte("t1"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
						&Tag{
							Key: []byte("t2"),
							Val: []byte(cliutils.CreateRandomString(100)),
						},
					},
					Fields: []*Field{
						&Field{
							Key: []byte("f1"),
							Val: lp.IntValue(int64(1024)),
						},
						&Field{
							Key: []byte("f2"),
							Val: lp.MustNewValue(1024.2048),
						},

						&Field{
							Key: []byte("f3"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(1024)),
						},

						&Field{
							Key: []byte("f4"),
							Val: lp.MustNewValue(cliutils.CreateRandomString(32 * 1024)),
						},
					},
					Time: time.Now(),
				},
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name+"-New", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				Encode(tc.pts, tc.opt)
			}
		})

		b.Run(tc.name+"-Old", func(b *testing.B) {
			for _, pt := range tc.pts {
				tags := map[string]string{}
				fields := map[string]interface{}{}

				for _, x := range pt.Tags {
					tags[string(x.Key)] = string(x.Val)
				}

				for _, x := range pt.Fields {
					fields[string(x.Key)] = x.Val.Interface()
				}

				for i := 0; i < b.N; i++ {
					oldPoint, err := influxdb.NewPoint(string(pt.Measurement),
						tags,
						fields,
						time.Now().UTC())
					if err != nil {
						b.Error(err)
						return
					}

					oldPoint.String()
				}
			}
		})
	}
}
