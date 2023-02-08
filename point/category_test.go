// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestURL(t *testing.T) {
	cases := []struct {
		c      string
		expect Category
	}{
		{c: "/v1/write/x", expect: UnknownCategory},
		{c: "/v1/write/metric", expect: Metric},
		{c: "/v1/write/logging", expect: Logging},
		{c: "/v1/write/object", expect: Object},
		{c: "/v1/write/metrics", expect: MetricDeprecated},
		{c: "/v1/write/network", expect: Network},
		{c: "/v1/write/rum", expect: RUM},
		{c: "/v1/write/security", expect: Security},
		{c: "/v1/write/profiling", expect: Profiling},
		{c: "/v1/write/event", expect: KeyEvent},
		{c: "/v1/write/tracing", expect: Tracing},
		{c: "/v1/write/dynamic_dw", expect: DynamicDWCategory},
	}

	for _, tc := range cases {
		t.Run(tc.c, func(t *testing.T) {
			assert.Equal(t, tc.expect, CatURL(tc.c))
		})
	}
}

func TestAlias(t *testing.T) {
	cases := []struct {
		c      string
		expect Category
	}{
		{c: "X", expect: UnknownCategory},
		{c: "M", expect: Metric},
		{c: "L", expect: Logging},
		{c: "O", expect: Object},
		{c: "N", expect: Network},
		{c: "R", expect: RUM},
		{c: "S", expect: Security},
		{c: "P", expect: Profiling},
		{c: "E", expect: KeyEvent},
		{c: "T", expect: Tracing},
		{c: "Dynamic_dw", expect: UnknownCategory},
	}

	for _, tc := range cases {
		t.Run(tc.c, func(t *testing.T) {
			assert.Equal(t, tc.expect, CatAlias(tc.c))
		})
	}
}

func TestString(t *testing.T) {
	cases := []struct {
		c      string
		expect Category
	}{
		{c: "balabala", expect: UnknownCategory},
		{c: "metric", expect: Metric},
		{c: "metrics", expect: MetricDeprecated},
		{c: "logging", expect: Logging},
		{c: "object", expect: Object},
		{c: "network", expect: Network},
		{c: "rum", expect: RUM},
		{c: "security", expect: Security},
		{c: "profiling", expect: Profiling},
		{c: "event", expect: KeyEvent},
		{c: "tracing", expect: Tracing},
		{c: "dynamic_dw", expect: DynamicDWCategory},
	}

	for _, tc := range cases {
		t.Run(tc.c, func(t *testing.T) {
			assert.Equal(t, tc.expect, CatString(tc.c))
		})
	}
}
