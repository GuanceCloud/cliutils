// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"net"
	"strings"
	"testing"
	"time"
)

var tcpCases = []struct {
	t         *TCPTask
	fail      bool
	reasonCnt int
}{
	{
		fail:      false,
		reasonCnt: 0,
		t: &TCPTask{
			SuccessWhen: []*TCPSuccess{
				{
					ResponseTime: []*TCPResponseTime{{
						Target: "10s",
					}},
				},
			}, Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "success",
			},
		},
	},
	{
		fail:      false,
		reasonCnt: 1,
		t: &TCPTask{
			SuccessWhen: []*TCPSuccess{
				{
					ResponseTime: []*TCPResponseTime{{
						Target: "1us",
					}},
				},
			}, Task: &Task{

				ExternalID: "xxxx", Frequency: "10s", Name: "response_time_large",
			},
		},
	},
	{
		fail:      false,
		reasonCnt: 0,
		t: &TCPTask{
			Message: "hello",
			SuccessWhen: []*TCPSuccess{
				{
					ResponseMessage: []*SuccessOption{{
						Contains: "hello",
					}},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "response_message_valid",
			},
		},
	},
	{
		fail:      false,
		reasonCnt: 1,
		t: &TCPTask{
			Message: "hello",
			SuccessWhen: []*TCPSuccess{
				{
					ResponseMessage: []*SuccessOption{{
						Contains: "invalid",
					}},
				},
			},
			Task: &Task{
				ExternalID: "xxxx", Frequency: "10s", Name: "response_message_invalid",
			},
		},
	},
}

func TestTcp(t *testing.T) {
	for _, c := range tcpCases {
		server, err := tcpServer()
		if err != nil {
			t.Fail()
		}
		defer server.Close()

		addr := server.Addr().String()

		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			t.Fatal(err.Error())
			continue
		}
		c.t.Host = host
		c.t.Port = port

		c.t.SetChild(c.t)
		if err := c.t.Check(); err != nil {
			if c.fail == false {
				t.Errorf("case: %s, failed: %s", c.t.Name, err)
			} else {
				t.Logf("expected: %s", err.Error())
			}
			continue
		}

		err = c.t.Run()

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

func tcpServer() (server net.Listener, err error) {
	server, err = net.Listen("tcp", "")
	if err != nil {
		return
	}

	go func() {
		time.Sleep(30 * time.Second)
		server.Close()
	}()

	go func() {
		if conn, err := server.Accept(); err != nil {
			return
		} else {
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}

			_, _ = conn.Write(buf[:n])
		}
	}()

	return
}
