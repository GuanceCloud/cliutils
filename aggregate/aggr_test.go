package aggregate

import (
	io "io"
	"net/http"
	"net/http/httptest"
	"strconv"
	T "testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func otelHistograms(n int) []*point.Point {
	pts := make([]*point.Point, 0)
	for i := 0; i < n; i++ {
		var kvs point.KVs
		kvs = kvs.AddTag("service", "tmall").
			AddTag("agent_version", "1.30").
			AddTag("http_method", "GET").
			AddTag("http_route", "/tmall/*").
			AddTag("scope_name", "io.opentelemetry.tomcat-7.0").
			AddTag("host_name", "myClientHost").
			Add("http.server.duration_bucket", i).
			Add("le", float64(i*10))
		opts := point.DefaultMetricOptions()
		opts = append(opts, point.WithTime(time.Now()))
		pts = append(pts, point.NewPoint("otel_service", kvs, opts...))
	}

	return pts
}

func randPoints(npts int) []*point.Point {
	r := point.NewRander()
	pts := r.Rand(npts)

	for idx, pt := range pts {
		pt.SetName("basic") // override point name for better hash
		pt.SetTag("idx", strconv.Itoa(idx%123))
		pt.Set("f1", float64(idx)/3.14)
	}
	return pts
}

func getPoints(n int) []*point.Point {
	// return randPoints()
	return otelHistograms(n)
}

func TestHTTPPostBatch(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		npts := 100
		pts := randPoints(npts)
		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"idx"},
					Selector: &RuleSelector{
						Category:              point.Metric.String(),
						Measurements:          nil,
						measurementsWhitelist: nil,
						Fields:                []string{"f1"},
						fieldsWhitelist:       nil,
						Condition:             "",
						conds:                 nil,
					},
					Algorithms: map[string]*AggregationAlgo{
						"f1": {
							Method:      SUM,
							SourceField: "f1",
							AddTags: map[string]string{
								"extra_tag_1": "some_value",
							},
						},
						"f1_max": {
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
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], npts)

		for _, pt := range groups[0] {
			assert.NotEmpty(t, pt.GetTag("idx"))

			_, ok := pt.GetF("f1")
			assert.True(t, ok)
		}

		batches := a.AggregateRules[0].GroupbyBatch(&a, groups[0])

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeKey := r.Header.Get(GuanceRoutingKey)
			assert.NotEmpty(t, GuanceRoutingKey)
			strconv.ParseUint(routeKey, 10, 64)

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			defer r.Body.Close()

			var batch AggregationBatch
			assert.NoError(t, batch.Unmarshal(body))
			assert.True(t, len(batch.Points.Arr) > 0)

			t.Logf("payload: %d, pts: %d", len(body), len(batch.Points.Arr))
		}))
		defer ts.Close() //nolint:errcheck

		time.Sleep(time.Second)

		cli := http.Client{}

		t.Logf("%d batches", len(batches))

		// build protobuf
		for _, b := range batches {
			req, err := batchRequest(b, ts.URL)
			assert.NoError(t, err)
			resp, err := cli.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
	})

	t.Run(`otel_service`, func(t *T.T) {
		npts := 100
		pts := otelHistograms(npts)
		a := AggregatorConfigure{
			AggregateRules: []*AggregateRule{
				{
					Groupby: []string{"service", "http_method", "http_route", "le"},
					Selector: &RuleSelector{
						Category: point.Metric.String(),
						Fields:   []string{"http.server.duration_bucket"},
					},
					Algorithms: map[string]*AggregationAlgo{
						"otel.histograms": {
							Method:      HISTOGRAM,
							SourceField: "http.server.duration_bucket",
							Options: &AggregationAlgo_HistogramOpts{
								HistogramOpts: &HistogramOptions{
									Buckets: []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
								},
							},
						},
					},
				},
			},
		}

		assert.NoError(t, a.Setup())

		groups := a.SelectPoints(pts)
		assert.Len(t, groups, 1)
		assert.Len(t, groups[0], npts)

		for _, pt := range groups[0] {
			assert.NotEmpty(t, pt.GetTag("service"))

			_, ok := pt.GetF("le")
			assert.True(t, ok)
		}

		batches := a.AggregateRules[0].GroupbyBatch(&a, groups[0])

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeKey := r.Header.Get(GuanceRoutingKey)
			assert.NotEmpty(t, GuanceRoutingKey)
			strconv.ParseUint(routeKey, 10, 64)

			body, err := io.ReadAll(r.Body)
			assert.NoError(t, err)
			defer r.Body.Close()

			var batch AggregationBatch
			assert.NoError(t, batch.Unmarshal(body))
			assert.True(t, len(batch.Points.Arr) > 0)

			t.Logf("payload: %d, pts: %d", len(body), len(batch.Points.Arr))
		}))
		defer ts.Close() //nolint:errcheck

		time.Sleep(time.Second)

		cli := http.Client{}

		t.Logf("%d batches", len(batches))

		// build protobuf
		for _, b := range batches {
			req, err := batchRequest(b, ts.URL)
			assert.NoError(t, err)
			resp, err := cli.Do(req)
			assert.NoError(t, err)
			assert.Equal(t, 200, resp.StatusCode)
		}
	})
}

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
					Selector: &RuleSelector{
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
					Selector: &RuleSelector{
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

		groupby := a.AggregateRules[0].PickPoints(groups[0])
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
					Selector: &RuleSelector{
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

		groupby := a.AggregateRules[0].PickPoints(groups[0])
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
					Selector: &RuleSelector{
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

		groupby := a.AggregateRules[0].PickPoints(groups[0])
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
					Selector: &RuleSelector{
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
