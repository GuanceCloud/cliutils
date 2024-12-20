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

func TestPtKvsTag(t *testing.T) {
	funcs := []*Function{
		FnPtKvsGet,
		FnPtKvsDel,
		FnPtKvsSet,
		FnPtKvsKeys,
	}

	cases := []struct {
		name, pl, in string
		keyName      string
		expect       interface{}
		fail         bool
	}{
		{
			name: "set1",
			pl: `
			pt_kvs_set("key0", "abc", true)
			pt_kvs_set("key1", pt_kvs_get("key0"))
			pt_kvs_del("key0")
			if pt_kvs_get("key0") == nil {
				for k in pt_kvs_keys() {
					if k == "key1" {
						pt_kvs_set("key2", pt_kvs_get("key1"))
					}
				}
			}
			`,
			keyName: "key2",
			expect:  "abc",
		},
		{
			name: "set2",
			pl: `
			pt_kvs_set("key1", 1, true)
			pt_kvs_set("key2", 2)
			count = 0
			if "key1"  in pt_kvs_keys(tags=true, fields=false) {
				count += 1
			} 
			
			fields_key = pt_kvs_keys(false)

			if "key1" in fields_key {
				count = -1
			}

			if "key2" in fields_key {
				count +=2
			}

			if count == 3 {
				pt_kvs_set("test_ok", 1, true)
			}

			`,
			keyName: "test_ok",
			expect:  "1",
		},
		{
			name: "set4",
			pl: `
			pt_kvs_set("key1",  as_tag=true, value=1.1)
			`,
			keyName: "key1",
			expect:  "1.1",
		},
		{
			name: "set5",
			pl: `
			pt_kvs_set("key1", true, true)
			`,
			keyName: "key1",
			expect:  "true",
		},
		{
			name: "set6",
			pl: `
			key_name = "key1"
			pt_kvs_set(key_name, [1,2], true)
			`,
			keyName: "key1",
			expect:  "[1,2]",
		},
		{
			name: "set7",
			pl: `
			pt_kvs_set("key1", {"a":1, "b":2}, true)
			`,
			keyName: "key1",
			expect:  `{"a":1,"b":2}`,
		},
		{
			name: "set8",
			pl: `
			pt_kvs_set("key1", nil, true)
			`,
			keyName: "key1",
			expect:  "",
		},
		{
			name: "set8",
			pl: `
			pt_kvs_set("_", "1", true)
			`,
			keyName: "_",
			expect:  "1",
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

func TestPtKvsSet(t *testing.T) {
	funcs := []*Function{
		FnPtKvsGet,
		FnPtKvsDel,
		FnPtKvsSet,
		FnPtKvsKeys,
	}

	cases := []struct {
		name, pl, in string
		keyName      string
		expect       interface{}
		fail         bool
	}{
		{
			name: "set1",
			pl: `
			pt_kvs_set("key1", "abc")
			`,
			keyName: "key1",
			expect:  "abc",
		},
		{
			name: "set2",
			pl: `
			pt_kvs_set("key1", 1)
			`,
			keyName: "key1",
			expect:  int64(1),
		},
		{
			name: "set3",
			pl: `
			pt_kvs_set("key1", 1)
			`,
			keyName: "key1",
			expect:  int64(1),
		},
		{
			name: "set4",
			pl: `
			pt_kvs_set("key1", 1.)
			`,
			keyName: "key1",
			expect:  float64(1.),
		},
		{
			name: "set5",
			pl: `
			pt_kvs_set("key1", true)
			`,
			keyName: "key1",
			expect:  true,
		},
		{
			name: "set6",
			pl: `
			pt_kvs_set("key1", [1,2])
			`,
			keyName: "key1",
			expect:  "[1,2]",
		},
		{
			name: "set7",
			pl: `
			pt_kvs_set("key1", {"a":1, "b":2})
			`,
			keyName: "key1",
			expect:  `{"a":1,"b":2}`,
		},
		{
			name: "set8",
			pl: `
			pt_kvs_set("key1", nil)
			`,
			keyName: "key1",
			expect:  nil,
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
