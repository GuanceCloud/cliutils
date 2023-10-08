// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package metrics

import (
	"path/filepath"
	T "testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergePoint(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		pts := []*point.Point{
			point.NewPointV2("m1",
				append(point.NewTags(map[string]string{"t1": "v1", "t2": "v2"}),
					point.NewKVs(map[string]any{"f1": 123})...),
				point.DefaultLoggingOptions()...,
			),

			point.NewPointV2("m1",
				append(point.NewTags(map[string]string{"t1": "v1", "t2": "v2"}),
					point.NewKVs(map[string]any{"f2": "hello"})...),
				point.DefaultLoggingOptions()...,
			),
		}

		mPts := mergePts(pts)
		assert.Len(t, mPts, 1)
		mfs := mPts[0].Fields()
		assert.Equal(t, []byte(`hello`), mfs.Get(`f2`).GetD())

		t.Logf("point: %s", mPts[0].LineProto())
	})

	t.Run(`merge-multiple-time-series`, func(t *T.T) {
		pts := []*point.Point{
			point.NewPointV2("m1",
				append(point.NewTags(map[string]string{"t1": "v1", "t2": "v2"}),
					point.NewKVs(map[string]any{"f1": 123})...),
				point.DefaultLoggingOptions()...,
			),

			point.NewPointV2("m1",
				append(point.NewTags(map[string]string{"t1": "v1", "t2": "v2"}),
					point.NewKVs(map[string]any{"f2": "hello"})...),
				point.DefaultLoggingOptions()...,
			),

			point.NewPointV2("m1",
				append(point.NewTags(map[string]string{"tag1": "v1", "tag2": "v2"}),
					point.NewKVs(map[string]any{"f1": 123})...),
				point.DefaultLoggingOptions()...,
			),

			point.NewPointV2("m1",
				append(point.NewTags(map[string]string{"tag1": "v1", "tag2": "v2"}),
					point.NewKVs(map[string]any{"f2": "hello"})...),
				point.DefaultLoggingOptions()...,
			),
		}

		mPts := mergePts(pts)
		require.Len(t, mPts, 2)
		for _, pt := range mPts {
			t.Logf("point: %s", pt.LineProto())
		}

		mfs := mPts[0].Fields()
		assert.Equal(t, []byte(`hello`), mfs.Get(`f2`).GetD())

		mfs = mPts[1].Fields()
		assert.Equal(t, []byte(`hello`), mfs.Get(`f2`).GetD())
	})
}

func TestGatherPoint(t *T.T) {
	ns := "testing"

	t.Run(`basic`, func(t *T.T) {
		cnt := prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: ns,
				Name:      filepath.Base(t.Name()),
			}, []string{"tag1", "tag2"},
		)
		reg := prometheus.NewRegistry()
		reg.MustRegister(cnt)

		cnt.WithLabelValues("v1", "v2").Add(1.0)
		cnt.WithLabelValues("v1", "v2").Add(1.0)
		cnt.WithLabelValues("v1", "v2").Add(1.0)

		cnt.WithLabelValues("v3", "v4").Add(1.0)
		cnt.WithLabelValues("v3", "v4").Add(1.0)
		cnt.WithLabelValues("v3", "v4").Add(1.0)

		pts, err := doGatherPoints(reg)
		assert.NoError(t, err)

		require.Len(t, pts, 2)

		tags := pts[0].Tags()
		fs := pts[0].Fields()
		assert.Equal(t, []byte(ns), pts[0].Name())
		assert.Equal(t, []byte("v1"), tags.Get("tag1").GetD())
		assert.Equal(t, []byte("v2"), tags.Get("tag2").GetD())
		assert.Equal(t, 3.0, fs.Get(filepath.Base(t.Name())).GetF())

		tags = pts[1].Tags()
		fs = pts[1].Fields()
		assert.Equal(t, []byte(ns), pts[0].Name())
		assert.Equal(t, []byte("v3"), tags.Get("tag1").GetD())
		assert.Equal(t, []byte("v4"), tags.Get("tag2").GetD())
		assert.Equal(t, 3.0, fs.Get(filepath.Base(t.Name())).GetF())

		for _, pt := range pts {
			t.Logf("point: %s", pt.LineProto())
		}
	})
}
