package aggregate

import (
	"container/heap"
	"math/rand"
	"strconv"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func Test_alignNextWallTime(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		now := time.Unix(123, 0)
		wallTime := AlignNextWallTime(now, time.Second*10).Unix()
		assert.Equal(t, int64(130), wallTime)

		wallTime = AlignNextWallTime(now, time.Second).Unix()
		assert.Equal(t, int64(123), wallTime)
	})
}

func Test_heap(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		var (
			r    = point.NewRander()
			npts = 100
			pts  = r.Rand(npts)
			now  = int64(12345)
		)

		for i, pt := range pts {
			pt.SetName("basic")
			pt.SetTag("idx", strconv.Itoa(i%7))
			pt.Set("f1", float64(i)/3.14)

			pt.SetTime(time.Unix(rand.Int63()%now, 0))
		}

		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"idx"},
					Selector: &RuleSelector{
						Category: point.Metric.String(),
						Fields:   []string{"f1"},
					},
					Algorithms: map[string]*AggregationAlgo{
						"f1": {
							Method:      SUM,
							SourceField: "f1",
							Window:      int64(time.Second * 10),
						},
					},
				},
			},
		}

		assert.NoError(t, a.Setup())

		groups := a.SelectPoints(pts)

		assert.Len(t, groups, 1)

		batches := a.AggregateRules[0].GroupbyBatch(&a, groups[0])

		cc := NewCaculatorCache()
		cc.AddBatches(batches...)

		for _, c := range cc.heap {
			t.Logf("base: %s", c.Base())
		}

		var prev int64

		for {
			c := heap.Pop(cc)
			if c == nil {
				break
			}

			sum, ok := c.(*algoSum)
			assert.True(t, ok)
			assert.Equal(t, int64(10*time.Second), sum.window)

			assert.True(t, sum.nextWallTime > prev) // should always larher than previous one
			prev = sum.nextWallTime

			assert.Equal(t, -1, sum.heapIdx) // heap index has set to -1

			// assert.Equal(t, now-int64(npts), sum.nextWallTime/int64(time.Second))
			t.Logf("pop base: %s", sum.Base())
		}
	})
}
