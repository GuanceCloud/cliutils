package aggregate

import (
	"strconv"
	T "testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestAlgo(t *T.T) {
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

		groupby := a.AggregateRules[0].GroupbyPoints(groups[0])
		assert.Len(t, groupby, 3)

		for h, arr := range groupby {
			tagKvs := pointAggrTags(arr[0], a.AggregateRules[0].Groupby)

			if a.calcs[h] == nil {
				a.calcs[h] = &algoSumDelta{
					metricBase: metricBase{
						name:     "basic",
						hash:     h,
						key:      "f1",
						aggrTags: tagKvs,
					},
				}
			}

			a.calcs[h].addNewPoints(arr)

			aggrPts, err := a.calcs[h].aggr()
			assert.NoError(t, err)

			assert.Len(t, aggrPts, 1)
			t.Logf("aggr %d delta point: %s", len(arr), aggrPts[0].Pretty())

			sumDelta := a.calcs[h].(*algoSumDelta)

			assert.True(t, sumDelta.count > 0)
			assert.True(t, sumDelta.maxTime > 0)
			assert.True(t, sumDelta.delta > 0)

			a.calcs[h].reset()
			assert.Equal(t, int64(0), sumDelta.count)
			assert.Equal(t, int64(0), sumDelta.maxTime)
			assert.Equal(t, 0.0, sumDelta.delta)
		}
	})
}
