// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSortByTime(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		pts := []*Point{
			NewPointV2("p1", nil, WithTime(time.Now())),
			NewPointV2("p2", nil, WithTime(time.Now().Add(-time.Hour))),
		}

		t.Logf("before sort pt[0]: %s", pts[0].Pretty())
		t.Logf("before sort pt[1]: %s", pts[1].Pretty())

		SortByTime(pts)

		t.Logf("pt[0]: %s", pts[0].Pretty())
		t.Logf("pt[1]: %s", pts[1].Pretty())

		assert.Equal(t, "p2", pts[0].Name())
		assert.Equal(t, "p1", pts[1].Name())
	})
}
