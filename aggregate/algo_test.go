package aggregate

import (
	"strconv"
	T "testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestAlgo(t *T.T) {
	t.Run("multi-algo-within-single-rule", func(t *T.T) {
		r := point.NewRander()
		npts := 10
		pts := r.Rand(npts)

		for idx, pt := range pts {
			pt.SetName("basic") // override point name for better hash
			pt.SetTag("idx", strconv.Itoa(idx%3))
			pt.Set("f1", float64(idx)/3.14)
		}

		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"idx"},
					Selector: &ruleSelector{
						Category: point.Metric.String(),
						Fields:   []string{"f1"},
					},
					Algorithms: map[string]*AggregationAlgo{
						// sum
						"f1": {
							Method: SUM,
							AddTags: map[string]string{
								"extra_tag_1": "some_value",
							},
						},

						// max
						"f_max": {
							Method:      MAX,
							SourceField: "f1",
							AddTags: map[string]string{
								"extra_tag_1": "some_value",
							},
						},
					},
				},
			},
		}

		assert.NoError(t, a.Setup())
		groups := a.SelectPoints(pts)

		batches := a.AggregateRules[0].GroupbyBatch(&a, groups[0])
		assert.Len(t, batches, 3)

		cc := NewCaculatorCache()

		for _, b := range batches {
			cc.addBatch(b)
		}

		assert.Len(t, cc.cache, 6)

		for _, x := range cc.cache {
			aggrPts, err := x.aggr()
			assert.NoError(t, err)
			assert.Len(t, aggrPts, 1)

			assert.NotNilf(t, aggrPts[0].Get("extra_tag_1"), aggrPts[0].Pretty())

			switch calc := x.(type) {
			case *algoSum:
				assert.NotNil(t, aggrPts[0].Get("f1"))

				assert.True(t, calc.count > 0)
				assert.True(t, calc.maxTime > 0)
				assert.True(t, calc.delta > 0)
				calc.reset()
				assert.Equal(t, int64(0), calc.count)
				assert.Equal(t, int64(0), calc.maxTime)
				assert.Equal(t, 0.0, calc.delta)
			case *algoMax:

				assert.NotNil(t, aggrPts[0].Get("f_max"))

				assert.True(t, calc.count > 0)
				assert.True(t, calc.maxTime > 0)
				assert.True(t, calc.max > 0)
				calc.reset()
				assert.Equal(t, int64(0), calc.count)
				assert.Equal(t, int64(0), calc.maxTime)
				assert.Equal(t, 0.0, calc.max)
			}
		}
	})

	t.Run("sum-delta", func(t *T.T) {
		r := point.NewRander()
		npts := 10
		pts := r.Rand(npts)

		for idx, pt := range pts {
			pt.SetName("basic") // override point name for better hash
			pt.SetTag("idx", strconv.Itoa(idx%3))
			pt.Set("f1", float64(idx)/3.14)
		}

		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"idx"},
					Selector: &ruleSelector{
						Category: point.Metric.String(),
						Fields:   []string{"f1"},
					},
					Algorithms: map[string]*AggregationAlgo{
						"f1": {
							Method:      SUM,
							SourceField: "f1",
							AddTags: map[string]string{
								"extra_tag_1": "some_value",
							},
						},
					},
				},
			},
		}

		assert.NoError(t, a.Setup())
		groups := a.SelectPoints(pts)

		batches := a.AggregateRules[0].GroupbyBatch(&a, groups[0])
		assert.Len(t, batches, 3)

		cc := NewCaculatorCache()

		for _, b := range batches {
			cc.addBatch(b)
		}

		assert.Len(t, cc.cache, 3)

		for _, calc := range cc.cache {
			aggrPts, err := calc.aggr()
			assert.NoError(t, err)
			assert.Len(t, aggrPts, 1)
			t.Logf("delta point: %s", aggrPts[0].Pretty())

			sum := calc.(*algoSum)
			assert.True(t, sum.count > 0)
			assert.True(t, sum.maxTime > 0)
			assert.True(t, sum.delta > 0)
			sum.reset()
			assert.Equal(t, int64(0), sum.count)
			assert.Equal(t, int64(0), sum.maxTime)
			assert.Equal(t, 0.0, sum.delta)
		}
	})
}
