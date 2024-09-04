// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package manager for managing pipeline scripts
package manager

import (
	"testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestRelation(t *testing.T) {
	rl := NewPipelineRelation()

	rl.UpdateRelation(0, map[point.Category]map[string]string{
		point.Logging: {
			"abc": "a1.p",
		},
	})
	p, ok := rl.Query(point.Logging, "abc")
	assert.True(t, ok)
	assert.Equal(t, "a1.p", p)

	p, ok = rl.Query(point.Logging, "def")
	assert.False(t, ok)
	assert.Equal(t, "", p)

	name, ok := ScriptName(rl, point.Logging, point.NewPointV2("abc", point.NewKVs(map[string]interface{}{"message@json": "a"})), nil)
	assert.True(t, ok)
	assert.Equal(t, "a1.p", name)

	name, ok = ScriptName(rl, point.Logging, point.NewPointV2("abcd", point.NewKVs(map[string]interface{}{"message@json": "a"})), map[string]string{"abcd": "a2.p"})
	assert.True(t, ok)
	assert.Equal(t, "a2.p", name)
}
