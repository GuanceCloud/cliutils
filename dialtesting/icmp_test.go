package dialtesting

import (
	"strings"
	"testing"
)

var icmpCases = []struct {
	t         *IcmpTask
	fail      bool
	reasonCnt int
}{
	{
		fail:      false,
		reasonCnt: 0,
		t: &IcmpTask{
			Host:        "localhost",
			PacketCount: 5,
			SuccessWhen: []*IcmpSuccess{
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
			ExternalID: "xxxx", Frequency: "10s", Name: "success-ipv4",
		},
	},
	{
		fail:      false,
		reasonCnt: 0,
		t: &IcmpTask{
			Host:        "::1",
			PacketCount: 5,
			SuccessWhen: []*IcmpSuccess{
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
			ExternalID: "xxxx", Frequency: "10s", Name: "success-ipv6",
		},
	},
}

func TestIcmp(t *testing.T) {
	for _, c := range icmpCases {
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
		} else {
			if len(reasons) > 0 {
				t.Logf("case %s reasons:\n\t%s",
					c.t.Name, strings.Join(reasons, "\n\t"))
			}
		}
	}
}
