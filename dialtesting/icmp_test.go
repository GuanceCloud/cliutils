// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"strings"
	"testing"
)

var icmpCases = []struct {
	t         *ICMPTask
	fail      bool
	reasonCnt int
}{
	{
		fail:      false,
		reasonCnt: 0,
		t: &ICMPTask{
			Host:        "localhost",
			PacketCount: 5,
			SuccessWhen: []*ICMPSuccess{
				{
					ResponseTime: []*ResponseTimeSucess{
						{
							Func:   "avg",
							Op:     "lt",
							Target: "10ms",
						},
					},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "success-ipv4",
			},
		},
	},
	{
		fail:      false,
		reasonCnt: 0,
		t: &ICMPTask{
			Host:        "::1",
			PacketCount: 5,
			SuccessWhen: []*ICMPSuccess{
				{
					ResponseTime: []*ResponseTimeSucess{
						{
							Func:   "avg",
							Op:     "lt",
							Target: "10ms",
						},
					},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "success-ipv6",
			},
		},
	},
}

func TestIcmp(t *testing.T) {
	for _, c := range icmpCases {
		c.t.SetChild(c.t)
		if err := c.t.Check(); err != nil {
			if c.fail == false {
				t.Errorf("case: %s, failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		err := c.t.Run()
		if err != nil {
			if c.fail == false {
				t.Errorf("case %s failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		tags, fields := c.t.GetResults()

		t.Logf("ts: %+#v \n fs: %+#v \n ", tags, fields)

		reasons, _ := c.t.CheckResult()
		if len(reasons) != c.reasonCnt {
			t.Errorf("case %s expect %d reasons, but got %d reasons:\n\t%s",
				c.t.Name, c.reasonCnt, len(reasons), strings.Join(reasons, "\n\t"))
		} else if len(reasons) > 0 {
			t.Logf("case %s reasons:\n\t%s",
				c.t.Name, strings.Join(reasons, "\n\t"))
		}
	}
}
