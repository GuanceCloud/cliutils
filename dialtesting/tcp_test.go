// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"net"
	"strings"
	"testing"
	"text/template"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestTCPRunUsesFirstDNSResultForPrimaryDial(t *testing.T) {
	server, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is not available: %s", err)
	}
	defer server.Close()

	go func() {
		conn, err := server.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}()

	_, port, err := net.SplitHostPort(server.Addr().String())
	assert.NoError(t, err)

	oldLookupIP := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		assert.Equal(t, "dual-stack.example", host)
		return []net.IP{
			net.ParseIP("::1"),
			net.ParseIP("127.0.0.1"),
		}, nil
	}
	defer func() {
		lookupIP = oldLookupIP
	}()

	task := &TCPTask{
		Host: "dual-stack.example",
		Port: port,
		SuccessWhen: []*TCPSuccess{
			{
				ResponseTime: []*TCPResponseTime{{
					Target: "10s",
				}},
			},
		},
		Task: &Task{
			ExternalID: "xxxx",
			Frequency:  "10s",
			Name:       "dual-stack",
		},
	}
	task.SetChild(task)

	assert.NoError(t, task.Check())
	assert.NoError(t, task.Run())
	assert.Empty(t, task.reqError)
	assert.Equal(t, "::1", task.destIP)
}

func TestTCPRunTracerouteUsesSelectedDialIP(t *testing.T) {
	server, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback is not available: %s", err)
	}
	defer server.Close()

	go func() {
		conn, err := server.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}()

	_, port, err := net.SplitHostPort(server.Addr().String())
	assert.NoError(t, err)

	oldLookupIP := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		assert.Equal(t, "dual-stack.example", host)
		return []net.IP{
			net.ParseIP("::1"),
			net.ParseIP("127.0.0.1"),
		}, nil
	}
	defer func() {
		lookupIP = oldLookupIP
	}()

	var tracerouteTarget string
	oldRunTracerouteIP := runTracerouteIP
	runTracerouteIP = func(ip string, opt *TracerouteOption) ([]*Route, error) {
		tracerouteTarget = ip
		return []*Route{
			{
				Total: 1,
				Items: []*RouteItem{{IP: ip}},
			},
		}, nil
	}
	defer func() {
		runTracerouteIP = oldRunTracerouteIP
	}()

	task := &TCPTask{
		Host:             "dual-stack.example",
		Port:             port,
		EnableTraceroute: true,
		SuccessWhen: []*TCPSuccess{
			{
				ResponseTime: []*TCPResponseTime{{
					Target: "10s",
				}},
			},
		},
		Task: &Task{
			ExternalID: "xxxx",
			Frequency:  "10s",
			Name:       "dual-stack-traceroute",
		},
	}
	task.SetChild(task)

	assert.NoError(t, task.Check())
	assert.NoError(t, task.Run())
	assert.Empty(t, task.reqError)
	assert.Equal(t, "::1", task.destIP)
	assert.Equal(t, task.destIP, tracerouteTarget)
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

func TestTCPRenderTemplate(t *testing.T) {
	ct := &TCPTask{
		Host:    "{{host}}",
		Port:    "{{port}}",
		Message: "{{message}}",
	}

	fm := template.FuncMap{
		"host": func() string {
			return "localhost"
		},
		"port": func() string {
			return "8080"
		},
		"message": func() string {
			return "hello"
		},
	}

	task, err := NewTask("", ct)
	assert.NoError(t, err)

	ct, ok := task.(*TCPTask)
	assert.True(t, ok)

	assert.NoError(t, ct.renderTemplate(fm))
	assert.Equal(t, "localhost", ct.Host)
	assert.Equal(t, "8080", ct.Port)
	assert.Equal(t, "hello", ct.Message)
}
