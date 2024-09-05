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
		println(k)
		println(v.Type)
		println(v.Unit.Kind, v.Unit.Name, v.Unit.Base)
		println(v.Value)
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
