// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package parsing

import (
	"testing"
)

func TestGetPySpySummary(t *testing.T) {

	summaries, err := summary("testdata")

	if err != nil {
		t.Error(err)
	}

	for k, v := range summaries {
		t.Log(k)
		t.Log(v.Type)
		t.Log(v.Unit.Kind, v.Unit.Name, v.Unit.Base)
		t.Log(v.Value)
	}

}

func TestParseRawFlameGraph(t *testing.T) {
	flame, selectMap, err := ParseRawFlameGraph("testdata")

	if err != nil {
		t.Fatal(err)
	}

	for k, v := range selectMap {
		t.Log(k)
		t.Log(v.Mapping)
		for key, opt := range v.Options {
			t.Log(key)
			t.Log("title:", opt.Title, "Value:", opt.Value, "Unit: ", opt.Unit, "MappingValues:", opt.MappingValues)
			t.Log("")
		}
		t.Log("----------------------------------------")
	}

	_ = flame

}
