// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.
package point

import (
	T "testing"

	"github.com/GuanceCloud/cliutils/metrics"
	p8s "github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetrics(t *T.T) {
	t.Run(`basic`, func(t *T.T) {

		ResetMetrics()

		defer t.Cleanup(func() {
			ResetMetrics()
		})

		reg := p8s.NewRegistry()
		reg.MustRegister(PointCheckWarnVec)

		pt := NewPointV2("", nil)

		t.Logf("pt: %s", pt.Pretty())

		mfs, err := reg.Gather()
		require.NoError(t, err)

		assert.Equal(t, 1.0, metrics.GetMetricOnLabels(mfs,
			"point_check_point_warning_total",
			"empty measurement, use __default",
			"invalid_measurement",
		).GetCounter().GetValue())

		t.Logf("\n%s", metrics.MetricFamily2Text(mfs))
	})
}
