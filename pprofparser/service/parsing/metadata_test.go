package parsing

import (
	"testing"
)

func TestReadMetaData(t *testing.T) {

	meta, err := ReadMetaDataFile("./testdata/event.json")

	if err != nil {
		t.Fatal(err)
	}

	if meta.Format != RawFlameGraph && meta.Format != Collapsed {
		t.Error("not equal")
	}

	if meta.Profiler != Pyroscope {
		t.Errorf("expected: %s, got: %s", Pyroscope, meta.Profiler)
	}

	t.Log(meta.Format)

}
