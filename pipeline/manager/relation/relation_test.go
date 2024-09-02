// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package relation record source-name relation
package relation

import (
	"testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestRelation(t *testing.T) {
	rl := NewPipelineRelation()
	rl.defaultScript = map[point.Category]string{
		point.Logging: "log.p",
	}
	rl.relation = map[point.Category]map[string]string{
		point.Logging: {
			"abc": "a1.p",
		},
	}
	p, ok := rl.CatRelation(point.Logging, "abc")
	assert.True(t, ok)
	assert.Equal(t, "a1.p", p)

	p, ok = rl.CatRelation(point.Logging, "def")
	assert.False(t, ok)
	assert.Equal(t, "", p)

	p, ok = rl.CatDefault(point.Logging)
	assert.True(t, ok)
	assert.Equal(t, "log.p", p)

	p, ok = rl.CatDefault(point.Metric)
	assert.False(t, ok)
	assert.Equal(t, "", p)
}
