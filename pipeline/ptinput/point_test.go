// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package ptinput

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/GuanceCloud/platypus/pkg/ast"
)

func TestPt(t *testing.T) {
	pt := NewPlPt(point.Logging, "t", nil, nil, time.Now())
	if _, _, err := pt.Get("a"); err == nil {
		t.Fatal("err == nil")
	}

	if _, _, e := pt.Get("a"); e == nil {
		t.Fatal("ok")
	}

	if ok := pt.Set("a", 1, ast.Int); !ok {
		t.Fatal(ok)
	}

	if ok := pt.Set("a1", []any{1}, ast.List); !ok {
		t.Fatal(ok)
	}

	if ok := pt.Set("xx2", []any{1}, ast.List); !ok {
		t.Fatal(ok)
	}

	if ok := pt.Set("xx2", 1.2, ast.Float); !ok {
		t.Fatal(ok)
	}

	if _, _, err := pt.Get("xx2"); err != nil {
		t.Fatal(err)
	}

	if err := pt.RenameKey("xx2", "xxb"); err != nil {
		t.Fatal(err)
	}

	if ok := pt.SetTag("a", 1., ast.Float); !ok {
		t.Fatal(ok)
	}

	if ok := pt.Set("a", 1, ast.Int); !ok {
		t.Fatal(ok)
	}

	if _, ok := pt.Fields()["a"]; ok {
		t.Fatal("a in fields")
	}

	if err := pt.RenameKey("a", "b"); err != nil {
		t.Fatal(err)
	}

	if pt.PtTime().UnixNano() == 0 {
		t.Fatal("time == 0")
	}

	pt.GetAggBuckets()
	pt.SetAggBuckets(nil)

	pt.Set("time", 1, ast.Int)
	pt.KeyTime2Time()
	ppt := pt.Point()
	if ppt.Time().UnixNano() != 1 {
		t.Fatal("time != 1")
	}

	pt.MarkDrop(true)
	if !pt.Dropped() {
		t.Fatal("!dropped")
	}

	dpt := pt.Point()

	pt = WrapPoint(point.Logging, dpt)

	if _, _, err := pt.Get("b"); err != nil {
		t.Fatal(err.Error())
	}

	if _, ok := pt.Tags()["b"]; !ok {
		t.Fatal("b not in tags")
	}

	if _, dtyp, e := pt.Get("b"); e != nil || dtyp != ast.String {
		t.Fatal("not tag")
	}

	if ok := pt.Set("b", []any{}, ast.List); !ok {
		t.Fatal(ok)
	}

	if _, ok := pt.Fields()["xxb"]; !ok {
		t.Fatal("xxb not in field")
	}

	if pt.GetPtName() != "t" {
		t.Fatal("name != \"t\"")
	}

	pt.SetPtName("t2")
	if pt.GetPtName() != "t2" {
		t.Fatal("name != \"t2\"")
	}
}
