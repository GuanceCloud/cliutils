package aggregate

import (
	"strconv"
	T "testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestBatch(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
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
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], npts) // forked into 2X points

		for _, pt := range groups[0] {
			assert.NotEmpty(t, pt.GetTag("idx"))

			_, ok := pt.GetF("f1")
			assert.True(t, ok)
		}

		batches := a.AggregateRules[0].GroupbyBatch(&a, groups[0])
		assert.Len(t, batches, 3)

		// build protobuf
		var pbs [][]byte
		for _, b := range batches {
			pb, err := b.Marshal()
			assert.NoError(t, err)
			t.Logf("%d points, hash: %d, pb: %d, size: %d",
				len(b.Points.Arr), b.RoutingKey, len(pb), b.Size())
			pbs = append(pbs, pb)
		}

		// load from protobuf
		for _, pb := range pbs {
			var batch AggregationBatch
			assert.NoError(t, batch.Unmarshal(pb))
		}
	})
}

func TestAggregator(t *T.T) {
	t.Run("select-multiple-field-on-regex", func(t *T.T) {
		r := point.NewRander()
		npts := 10
		pts := r.Rand(npts)

		for idx, pt := range pts {
			pt.SetName("basic") // override point name for better hash
			pt.SetTag("idx", strconv.Itoa(idx%3))
			pt.Set("f_12345", float64(idx)/3.14)
		}

		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"idx"},
					Selector: &ruleSelector{
						Category: point.Metric.String(),
						Fields:   []string{"reg:f_.*"},
					},
				},
			},
		}

		assert.NoError(t, a.Setup())

		groups := a.SelectPoints(pts)
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], npts) // forked into 2X points

		for _, pt := range groups[0] {
			assert.NotEmpty(t, pt.GetTag("idx"))

			_, ok := pt.GetF("f_12345")
			assert.True(t, ok)
		}

		groupby := a.AggregateRules[0].GroupbyPoints(groups[0])
		assert.Len(t, groupby, 3)
		for h, arr := range groupby {
			t.Logf("%d: %d points", h, len(arr))
		}
	})

	t.Run("basic", func(t *T.T) {
		r := point.NewRander()
		pts := r.Rand(10)

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
				},
			},
		}
		assert.NoError(t, a.Setup())

		groups := a.SelectPoints(pts)
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], 10)

		t.Logf("selected point...")
		for _, pt := range groups[0] {
			assert.NotEmpty(t, pt.GetTag("idx"))
		}

		groupby := a.AggregateRules[0].GroupbyPoints(groups[0])
		assert.Len(t, groupby, 3)
		for h, arr := range groupby {
			t.Logf("%d: %d points", h, len(arr))
		}
	})

	t.Run("select-multiple-field", func(t *T.T) {
		r := point.NewRander()
		npts := 10
		pts := r.Rand(npts)

		for idx, pt := range pts {
			pt.SetName("basic") // override point name for better hash
			pt.SetTag("idx", strconv.Itoa(idx%3))
			pt.Set("f1", float64(idx)/3.14)
			pt.Set("f2", float64(idx)/2.414)
		}

		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"idx"},
					Selector: &ruleSelector{
						Category: point.Metric.String(),
						Fields:   []string{"f1", "f2"},
					},
				},
			},
		}
		assert.NoError(t, a.Setup())

		groups := a.SelectPoints(pts)
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], npts*2) // forked into 2X points

		for _, pt := range groups[0] {
			assert.NotEmpty(t, pt.GetTag("idx"))

			// f1 or f2 exist
			_, ok1 := pt.GetF("f1")
			_, ok2 := pt.GetF("f2")
			assert.True(t, ok1 || ok2)
			assert.False(t, ok1 && ok2) // should not exist at the same time.
		}

		groupby := a.AggregateRules[0].GroupbyPoints(groups[0])
		assert.Len(t, groupby, 3*2)
		for h, arr := range groupby {
			t.Logf("%d: %d points", h, len(arr))
		}
	})

	t.Run("with-condition", func(t *T.T) {
		r := point.NewRander()
		pts := r.Rand(10)

		for idx, pt := range pts {
			pt.SetTag("idx", strconv.Itoa(idx%3))
			pt.Set("f1", idx)
		}

		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Selector: &ruleSelector{
						Category:  point.Metric.String(),
						Fields:    []string{"f1"},
						Condition: `{f1 IN [1,2,0]}`,
					},
				},
			},
		}
		assert.NoError(t, a.Setup())

		groups := a.SelectPoints(pts)
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], 3)
		for _, pt := range groups[0] {
			t.Logf("%s", pt.Pretty())
		}
	})
}
