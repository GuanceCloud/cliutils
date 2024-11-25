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

func TestHash(t *testing.T) {
	funcs := []*Function{
		FnPtKvsGet,
		FnPtKvsDel,
		FnPtKvsSet,
		FnPtKvsKeys,

		FnHash,
	}

	cases := []struct {
		name, pl, in string
		keyName      string
		expect       interface{}
		fail         bool
	}{
		{
			name: "md5",
			pl: `
			sum = hash("abc", "md5")
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "900150983cd24fb0d6963f7d28e17f72",
		},
		{
			name: "xx",
			pl: `
			sum = hash("abc", "xx")
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "",
		},
		{
			name: "sha1",
			pl: `
			sum = hash("abc", "sha1")
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "a9993e364706816aba3e25717850c26c9cd0d89d",
		},
		{
			name: "sha256",
			pl: `
			sum = hash("abc", "sha256")
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name: "sha512",
			pl: `
			sum = hash("abc", "sha512")
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
		},
		{
			name: "sha512",
			pl: `
			sum = hash("abc", )
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
			fail:    true,
		},
		{
			name: "sha512",
			pl: `
			sum = hash(method= "abc", )
			pt_kvs_set("result", sum)
			`,
			keyName: "result",
			expect:  "ddaf35a193617abacc417349ae20413112e6fa4e89a97ea20a9eeee64b55d39a2192992a274fc1a836ba3c23a3feebbd454d4423643ce80e2a9ac94fa54ca49f",
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
