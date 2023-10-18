// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package offload

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
)

type sender4test struct {
	d []*point.Point
}

func (sender *sender4test) Send(s uint64, cat point.Category, pts []*point.Point) error {
	sender.d = append(sender.d, pts...)
	return nil
}

func TestWkr(t *testing.T) {
	td := func(n []int) [][]*point.Point {
		r := make([][]*point.Point, len(n))
		c := 0
		for idx, v := range n {
			r[idx] = make([]*point.Point, v)
			for i := 0; i < v; i++ {
				kvs := point.NewTags(nil)
				kvs = append(kvs, point.NewKVs(map[string]interface{}{
					"n": c + i,
				})...)
				r[idx][i] = point.NewPointV2(strconv.FormatInt(int64(c+i), 10), kvs, point.CommonLoggingOptions()...)
			}
			c += v
		}
		return r
	}

	cases := []struct {
		n []int
	}{
		{
			n: []int{10, 20, 40},
		},
		{
			n: []int{0, 10, ptsBuf, 20, 40, ptsBuf, 100},
		},
		{
			n: []int{ptsBuf, ptsBuf * 2, ptsBuf*3 - 3},
		},
	}

	for _, v := range cases {
		t.Run("t1", func(t *testing.T) {
			ptsLi := td(v.n)
			s := &sender4test{d: []*point.Point{}}
			wkr := OffloadWorker{
				ch:       newDataChan(),
				stopChan: make(chan struct{}),
				sender:   s,
			}

			ctx := context.Background()

			go func() {
				time.Sleep(time.Millisecond * 20)
				for _, pts := range ptsLi {
					wkr.Send(point.Logging, pts)
				}
				time.Sleep(time.Millisecond * 50)
				wkr.Stop()
			}()

			wkr.Customer(ctx, point.Logging)

			total := 0
			for _, v := range v.n {
				total += v
			}
			if len(s.d) != total {
				t.Fatal(len(s.d), total)
			}

			for i := 0; i < total; i++ {
				if strconv.FormatInt(int64(i), 10) != s.d[i].Name() {
					t.Fatal(strconv.FormatInt(int64(i), 10), s.d[i].Name())
				}
			}
		})
	}
}

func TestDataKitHTTP(t *testing.T) {
	_, err := NewOffloader(&OffloadConfig{
		Receiver:  DKRcv,
		Addresses: []string{"aaa"},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewOffloader(&OffloadConfig{
		Receiver:  "abc",
		Addresses: []string{"aaa"},
	})

	if err == nil {
		t.Fatal("err == nil")
	} else {
		t.Log(err)
	}
}
