// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package ipip implement ipdb.
package ipip

import (
	"strconv"
	"testing"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb"
	"github.com/stretchr/testify/assert"
)

func TestISP(t *testing.T) {
	dir := "./testdata/"
	var ipip IPIP
	ipip.Init(dir, nil)

	assert.Equal(t, "", ipip.SearchIsp("221.0.0.0"))
	assert.Equal(t, "unknown", ipip.SearchIsp("aaa"))
}

func TestGEO(t *testing.T) {
	dir := "./testdata/"

	cases := []struct {
		ip     string
		dir    string
		cfg    map[string]string
		val    ipdb.IPdbRecord
		failed bool
	}{
		{
			ip:  "221.0.0.0",
			dir: dir,
			cfg: map[string]string{
				CfgIPIPLanguage: "CN",
			},
			val: ipdb.IPdbRecord{
				Country: "中国",
				Region:  "山东",
				City:    "济南",
				Isp:     "unknown",
			},
		},
		{
			ip:  "aaa",
			dir: dir,
			cfg: map[string]string{
				CfgIPIPLanguage: "CN",
			},
			failed: true,
		},
		{
			ip:  "221.0.0.0",
			dir: dir + "-xxxx-test",
			cfg: map[string]string{
				CfgIPIPLanguage: "CN",
			},
			failed: true,
		},
		{
			ip:  "221.0.0.0",
			dir: dir,
			cfg: map[string]string{
				CfgIPIPLanguage: "CN",
				CfgIPIPFile:     "xxx",
			},
			failed: true,
		},
		{
			ip:  "221.0.0.0",
			dir: dir,
			cfg: map[string]string{},
			val: ipdb.IPdbRecord{
				Country: "中国",
				Region:  "山东",
				City:    "济南",
				Isp:     "unknown",
			},
		},
		{
			ip:  "221.0.0.0",
			dir: dir,
			cfg: map[string]string{
				"ipip_language": "sv",
			},
			val: ipdb.IPdbRecord{
				Country: "中国",
				Region:  "山东",
				City:    "济南",
				Isp:     "unknown",
			}},
	}

	for i, c := range cases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			var ipip IPIP
			ipip.Init(c.dir, c.cfg)
			if v, err := ipip.Geo(c.ip); err != nil {
				if !c.failed {
					t.Error(err)
				}
			} else {
				if c.failed {
					t.Error("should be err")
				} else {
					assert.Equal(t, c.val, *v)
				}
			}
		})
	}
}
