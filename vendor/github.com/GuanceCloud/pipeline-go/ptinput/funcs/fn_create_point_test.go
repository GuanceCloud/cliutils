// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package funcs

import (
	"testing"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestPtCategory(t *testing.T) {
	t.Run("llm-agent", func(t *testing.T) {
		assert.Equal(t, point.LLMAgent, ptCategory(point.SLLMAgent))
		assert.Equal(t, point.LLMAgent, ptCategory(point.CLLMAgent))
	})

	t.Run("unknown", func(t *testing.T) {
		assert.Equal(t, point.UnknownCategory, ptCategory("llm-agent-x"))
	})
}
