// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package funcs

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	tu "github.com/GuanceCloud/cliutils/testutil"
)

func TestSetopt(t *testing.T) {
	cases := []struct {
		name, pl, in string
		expected     bool
		fail         bool
	}{
		{
			name: "1",
			pl: `1+1 ; setopt(status_mapping=false)
			`,
			expected: false,
		},
		{
			name: "2",
			pl: `setopt(status_mapping=true)
			`,
			expected: true,
		},
		{
			name: "2",
			pl: `setopt(true)
			`,
			fail: true,
		},
		{
			name: "3",
			pl: `
			if false {
				setopt(status_mapping=false)
			}
			`,
			expected: true,
		},
		{
			name: "3.1",
			pl: `
			if true {
				setopt(status_mapping=false)
			}
			`,
			expected: false,
		},
		{
			name: "4",
			pl: `
			if false {
				setopt(status_mapping=false, x=1)
			}
			`,
			fail: true,
		},
		{
			name: "5",
			pl: `
				setopt()
			`,
			expected: true,
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewTestingRunner(tc.pl)
			if err != nil {
				if tc.fail {
					t.Logf("[%d]expect error: %s", idx, err)
				} else {
					t.Errorf("[%d] failed: %s", idx, err)
				}
				return
			}
			pt := ptinput.NewPlPt(
				point.Logging, "test", nil, map[string]any{"message": tc.in}, time.Now())
			errR := runScript(runner, pt)

			if errR != nil {
				t.Fatal(errR.Error())
			}

			tu.Equals(t, tc.expected, pt.GetStatusMapping())

			t.Logf("[%d] PASS", idx)
		})
	}
}
