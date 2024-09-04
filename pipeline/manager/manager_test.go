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

func TestManger(t *testing.T) {
	m := NewManager(NewManagerCfg(nil, nil))
	m.LoadScripts(NSDefault, map[point.Category]map[string]string{
		point.Logging: {
			"abc.p": "if true {}",
			"def.p": "if true {}",
		},
		point.DialTesting: {
			"abc.p": "if true {}",
		},
		point.Profiling: {
			"abc.p": "if true {}",
		},
	}, nil)

	m.LoadScripts(NSRemote, map[point.Category]map[string]string{
		point.Logging: {
			"xyz.p": "if true {}",
			"def.p": "if true {}",
		},
	}, nil)

	rl := m.GetScriptRelation()
	rl.relation = map[point.Category]map[string]string{
		point.DialTesting: {
			"x1": "1.p",
			"x2": "2.p",
		},
	}

	rl.UpdateRelation(0, map[point.Category]map[string]string{
		point.Logging: {
			"x1": "a1.p",
			"x2": "abc.p",
		},
	})

	m.UpdateDefaultScript(map[point.Category]string{
		point.Logging: "def.p",
	})

	// L: xyz.p, def.p (R), abc.p Df: def.p Rl: x1 -> a1, x2 -> abc
	// T: abc.p,
	cases := []struct {
		cat        point.Category
		source, ns string
		name       [2]string
		notFound   bool
	}{
		{
			cat:    point.Logging,
			source: "abc",
			name:   [2]string{"abc.p", "abc.p"},
			ns:     NSDefault,
		},
		{
			cat:    point.Logging,
			source: "def",
			name:   [2]string{"def.p", "def.p"},
			ns:     NSRemote,
		},
		{
			cat:    point.Logging,
			source: "xyz",
			name:   [2]string{"xyz.p", "xyz.p"},
			ns:     NSRemote,
		},
		{
			cat:    point.Logging,
			source: "x1",
			name:   [2]string{"a1.p", "def.p"},
			ns:     NSRemote,
		},
		{
			cat:    point.Logging,
			source: "x2",
			name:   [2]string{"abc.p", "abc.p"},
			ns:     NSDefault,
		},

		{
			cat:    point.Logging,
			source: "x3",
			name:   [2]string{"x3.p", "def.p"},
			ns:     NSRemote,
		},
		{
			cat:      point.DialTesting,
			source:   "x3",
			name:     [2]string{"x3.p", ""},
			notFound: true,
		},
	}

	t.Run("GetScriptName", func(t *testing.T) {
		for _, tt := range cases {
			t.Run(tt.source, func(t *testing.T) {
				if tt.source == "x1" {
					a := 1
					_ = a
				}
				name, _ := ScriptName(rl, tt.cat, point.NewPointV2(tt.source, point.NewKVs(map[string]interface{}{
					"ns": tt.ns,
				})), nil)
				assert.Equal(t, tt.name[0], name)
				if s, ok := m.QueryScript(tt.cat, name); ok {
					if tt.notFound {
						t.Error("not found")
						return
					}
					assert.Equal(t, tt.name[1], s.name)
					assert.Equal(t, tt.ns, s.ns)
				} else {
					if !tt.notFound {
						t.Error("found")
					}
				}
			})
		}
	})

}
