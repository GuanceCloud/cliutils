// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package logger

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"testing"
	"time"

	tu "gitlab.jiagouyun.com/cloudcare-tools/cliutils/testutil"
)

type listener interface {
	Close() error
}

var stopListenMsg = "__stop_server"

func serve(t *testing.T,
	expectLogCnt int,
	proto string,
	listen string,
	ch chan interface{},
) (listener, error) {
	t.Helper()
	switch proto {
	case "udp":
		l, err := net.ListenPacket("udp", listen)
		if err != nil {
			t.Logf("net.ListenPacket: %s", err)
			return nil, err
		}

		go func() {
			readBuf := make([]byte, 1024)
			lines := []string{}
			for {
				n, _, err := l.ReadFrom(readBuf)
				if err != nil {
					t.Error(err)
					ch <- nil
					return
				}

				if bytes.Contains(readBuf, []byte(stopListenMsg)) {
					for _, line := range lines {
						t.Logf("log data: %s", line)
					}

					tu.Equals(t, expectLogCnt, len(lines))
					ch <- nil
					return
				} else {
					t.Logf("get log data: %s", string(readBuf[:n]))
					lines = append(lines, string(readBuf[:n]))
				}
			}
		}()
		return l, nil

	case "tcp":
		l, err := net.Listen("tcp", listen)
		if err != nil {
			return nil, err
		}

		t.Logf("listen to %s ok", listen)

		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					t.Logf("l.Accept: %s", err)
					ch <- nil
					return
				}

				go func(c net.Conn) {
					lines := []string{}
					readBuf := make([]byte, 1024)
					defer c.Close()

					for {
						n, err := c.Read(readBuf)
						if err != nil {
							t.Error(err)
						}

						if bytes.Contains(readBuf, []byte(stopListenMsg)) {
							for _, line := range lines {
								t.Logf("log data: %s", line)
							}

							tu.Equals(t, expectLogCnt, len(lines))
							return
						} else {
							t.Logf("get log data: %s", string(readBuf[:n]))
							lines = append(lines, string(readBuf[:n]))
						}
					}
				}(conn)
			}
		}()

		return l, nil
	}

	return nil, fmt.Errorf("should not been here")
}

func TestRemoteLogger(t *testing.T) {
	cases := []struct {
		name         string
		file         string
		level        string
		options      int
		logs         [][2]string
		expectLogCnt int
		ch           chan interface{}
		fail         bool
	}{
		{
			name:    "tcp-logger-server",
			file:    "tcp://0.0.0.0:12345",
			options: OPT_DEFAULT,
			level:   DEBUG,
			logs: [][2]string{
				{DEBUG, "this is debug msg"},
				{INFO, "this is info msg"},
			},
			expectLogCnt: 2,
			ch:           make(chan interface{}),
		},

		{
			name:    "udp-logger-server",
			file:    "udp://0.0.0.0:12345",
			options: OPT_DEFAULT,
			level:   DEBUG,
			logs: [][2]string{
				{DEBUG, "this is debug msg"},
				{INFO, "this is info msg"},
			},
			expectLogCnt: 2,
			ch:           make(chan interface{}),
		},

		{
			name:    "udp-logger-server-with-color-log",
			file:    "udp://0.0.0.0:12345",
			options: OPT_DEFAULT | OPT_COLOR,
			level:   DEBUG,
			logs: [][2]string{
				{DEBUG, "this is debug msg"},
				{INFO, "this is info msg"},
			},
			expectLogCnt: 2,
			ch:           make(chan interface{}),
		},

		{
			name:    "invalid-remote-logger",
			file:    "http://0.0.0.0:12345",
			options: OPT_DEFAULT | OPT_COLOR,
			fail:    true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.file)
			if err != nil {
				t.Error(err)
			}

			var listen listener

			switch u.Scheme {
			case "tcp", "udp":
				listen, err = serve(t, tc.expectLogCnt, u.Scheme, u.Host, tc.ch)
				if err != nil {
					t.Error(err)
				}
				time.Sleep(time.Second) // wait server ok
			default:
				t.Logf("invalid schema")
				return
			}

			rs, err := newRemoteSync(u.Scheme, u.Host)
			if err != nil {
				t.Error(err)
			}

			rl, err := newCustomizeRootLogger(tc.level, tc.options, rs)
			root = rl

			if err != nil {
				t.Error(err)
			}

			l := SLogger(tc.name)

			for _, arr := range tc.logs {
				if len(arr) != 2 {
					t.Error("expect length 2")
				}

				switch arr[0] {
				case DEBUG:
					l.Debugf("-> %s", arr[1])
				case INFO:
					l.Infof("-> %s", arr[1])
				}

				time.Sleep(time.Millisecond * 10)
			}

			// end of log message: shutdown server
			l.Info(stopListenMsg)
			time.Sleep(time.Millisecond * 10) // wait server receive ok

			listen.Close()
			<-tc.ch
		})
	}
}
