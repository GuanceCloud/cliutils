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
	"github.com/stretchr/testify/assert"
)

func TestGJSON(t *testing.T) {
	Cases := []*funcCase{
		{
			in: `{
				"name": {"first": "Tom", "last": "Anderson"},
				"age": 37,
				"children": ["Sara","Alex","Jack"],
				"fav.movie": "Deer Hunter",
				"friends": [
				  {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
				  {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
				  {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
				]
			  }`,
			script:   `gjson(_, "age")`,
			expected: float64(37),
			key:      "age",
		},
		{
			in: `{
				"name": {"first": "Tom", "last": "Anderson"},
				"age":37,
				"children": ["Sara","Alex","Jack"],
				"fav.movie": "Deer Hunter",
				"friends": [
				  {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
				  {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
				  {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
				]
			  }`,
			script:   `gjson(_, "children")`,
			expected: `["Sara","Alex","Jack"]`,
			key:      "children",
		},
		{
			in: `{
				"name": {"first": "Tom", "last": "Anderson"},
				"age":37,
				"children": ["Sara","Alex","Jack"],
				"fav.movie": "Deer Hunter",
				"friends": [
				  {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
				  {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
				  {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
				]
			  }`,
			script:   `gjson(_, "name")`,
			expected: `{"first": "Tom", "last": "Anderson"}`,
			key:      "name",
		},
		{
			in: `{
				"name": {"first": "Tom", "last": "Anderson"},
				"age":37,
				"children": ["Sara","Alex","Jack"],
				"fav.movie": "Deer Hunter",
				"friends": [
				  {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
				  {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
				  {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
				]
			  }`,
			script: `gjson(_, "name")
			gjson(name, "first")`,
			expected: `Tom`,
			key:      "first",
		},
		{
			in: `{
			  "name": {"first": "Tom", "last": "Anderson"},
			  "age":37,
			  "children": ["Sara","Alex","Jack"],
			  "fav.movie": "Deer Hunter",
			  "friends": [
			    {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
			    {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
			    {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
			  ]
			}`,
			script: `gjson(_, "friends")
			gjson(friends, "1.first", "f_first")`,
			expected: "Roger",
			key:      "f_first",
		},
		{
			in: `{
			  "name": {"first": "Tom", "last": "Anderson"},
			  "age":37,
			  "children": ["Sara","Alex","Jack"],
			  "fav.movie": "Deer Hunter",
			  "friends": [
			    {"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
			    {"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
			    {"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
			  ]
			}`,
			script: `gjson(_, "friends", "friends")
			gjson(friends, "1.nets.1", "f_nets")`,
			expected: "tw",
			key:      "f_nets",
		},
		{
			in: `[
				{"first": "Dale", "last": "Murphy", "age": 44, "nets": ["ig", "fb", "tw"]},
				{"first": "Roger", "last": "Craig", "age": 68, "nets": ["fb", "tw"]},
				{"first": "Jane", "last": "Murphy", "age": 47, "nets": ["ig", "tw"]}
				]`,
			script:   `gjson(_, "0.nets.2", "f_nets")`,
			expected: "tw",
			key:      "f_nets",
		},
	}
	for idx, tc := range Cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewTestingRunner(tc.script)

			if err != nil && tc.fail {
				return
			} else if err != nil || tc.fail {
				tu.Equals(t, nil, err)
				tu.Equals(t, tc.fail, err != nil)
			}

			pt := ptinput.NewPlPoint(
				point.Logging, "test", nil, map[string]any{"message": tc.in}, time.Now())
			errR := runScript(runner, pt)
			if errR != nil {
				t.Fatal(errR.Error())
			}

			r, _, ok := pt.GetWithIsTag(tc.key)
			tu.Equals(t, true, ok)
			if tc.key == "[2].age" {
				t.Log(1)
			}
			assert.Equal(t, tc.expected, r)

			t.Logf("[%d] PASS", idx)
		})
	}
}
