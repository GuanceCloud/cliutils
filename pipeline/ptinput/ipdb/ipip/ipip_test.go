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

func Test(t *testing.T) {
	dir := "./testdata/"
	var ipip IPIP
	ipip.Init(dir, nil)

	rec, err := ipip.Geo("221.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, ipdb.IPdbRecord{
		Country: "中国",
		Region:  "山东",
		City:    "济南",
		Isp:     "unknown",
	}, *rec)

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
				"ipip_language": "CN",
			},
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
				"ipip_language": "CN",
				"ipip_file":     "xxx",
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
