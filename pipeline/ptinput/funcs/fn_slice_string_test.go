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
	"github.com/stretchr/testify/assert"
)

func TestSliceString(t *testing.T) {
	funcs := []*Function{
		FnPtKvsGet,
		FnPtKvsDel,
		FnPtKvsSet,
		FnPtKvsKeys,
		FnSliceString,
	}

	cases := []struct {
		name, pl, in string
		keyName      string
		expect       interface{}
		fail         bool
	}{
		{
			name: "normal1",
			pl: `
			substring = slice_string("15384073392",0,3)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "153",
			fail:    false,
		},
		{
			name: "normal2",
			pl: `
			substring = slice_string("15384073392",5,10)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "07339",
			fail:    false,
		},
		{
			name: "normal3",
			pl: `
			substring = slice_string("abcdefghijklmnop",0,10)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "abcdefghij",
			fail:    false,
		},
		{
			name: "out of range1",
			pl: `
			substring = slice_string("abcdefghijklmnop",-1,10)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    false,
		},
		{
			name: "out of range2",
			pl: `
			substring = slice_string("abcdefghijklmnop",0,100)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    false,
		},
		{
			name: "not integer1",
			pl: `
			substring = slice_string("abcdefghijklmnop","a","b")
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    true,
		},
		{
			name: "not integer2",
			pl: `
			substring = slice_string("abcdefghijklmnop","abc","def")
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    true,
		},
		{
			name: "not string",
			pl: `
			substring = slice_string(12345,0,3)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    true,
		},
		{
			name: "not correct args",
			pl: `
			substring = slice_string("abcdefghijklmnop",0)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    true,
		},
		{
			name: "not correct args",
			pl: `
			substring = slice_string("abcdefghijklmnop",0,1,2)
			pt_kvs_set("result", substring)
			`,
			keyName: "result",
			expect:  "",
			fail:    true,
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script, err := parseScipt(tc.pl, funcs)
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
			errR := script.Run(pt, nil)
			if errR != nil {
				t.Fatal(errR.Error())
			}

			v, _, _ := pt.Get(tc.keyName)
			assert.Equal(t, tc.expect, v)
			t.Logf("[%d] PASS", idx)

		})
	}
}
