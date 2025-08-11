// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"testing"
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSet(t *testing.T) {
	t.Run(`set-name`, func(t *T.T) {
		p := NewPoint(`abc`, nil)
		p.SetName("def")
		assert.Equal(t, `def`, p.Name())
	})

	t.Run(`set-time`, func(t *T.T) {
		p := NewPoint(`abc`, nil, WithTime(time.Unix(0, 123)))
		p.SetTime(time.Unix(0, 456))
		assert.Equal(t, time.Unix(0, 456), p.Time())
	})
}
