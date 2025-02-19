// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package manager

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestScript(t *testing.T) {
	ret, retErr := NewScripts(map[string]string{
		"abc.p": "if true {}",
	}, nil, NSGitRepo, point.Logging)

	if len(retErr) > 0 {
		t.Fatal(retErr)
	}

	s := ret["abc.p"]

	if ng := s.Engine(); ng == nil {
		t.Fatalf("no engine")
	}
	plpt := ptinput.NewPlPt(point.Logging, "ng", nil, nil, time.Now())
	err := s.Run(plpt, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, plpt.Fields(), map[string]interface{}{"status": DefaultStatus})
	assert.Equal(t, 0, len(plpt.Tags()))
	assert.Equal(t, "abc.p", s.Name())
	assert.Equal(t, point.Logging, s.Category())
	assert.Equal(t, s.NS(), NSGitRepo)

	//nolint:dogsled
	plpt = ptinput.NewPlPt(point.Logging, "ng", nil, nil, time.Now())
	err = s.Run(plpt, nil, &Option{DisableAddStatusField: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(plpt.Fields()) != 1 {
		t.Fatal(plpt.Fields())
	} else {
		if _, ok := plpt.Fields()["status"]; !ok {
			t.Fatal("without status")
		}
	}

	//nolint:dogsled
	plpt = ptinput.NewPlPt(point.Logging, "ng", nil, nil, time.Now())
	err = s.Run(plpt, nil, &Option{
		DisableAddStatusField: false,
		IgnoreStatus:          []string{DefaultStatus},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plpt.Dropped() != true {
		t.Fatal("!drop")
	}
}

func TestDrop(t *testing.T) {
	ret, retErr := NewScripts(map[string]string{"abc.p": "add_key(a, \"a\"); add_key(status, \"debug\"); drop(); add_key(b, \"b\")"},
		nil, NSGitRepo, point.Logging)
	if len(retErr) > 0 {
		t.Fatal(retErr)
	}

	s := ret["abc.p"]

	plpt := ptinput.NewPlPt(point.Logging, "ng", nil, nil, time.Now())
	if err := s.Run(plpt, nil, nil); err != nil {
		t.Fatal(err)
	}

	if plpt.Dropped() != true {
		t.Error("drop != true")
	}
}
