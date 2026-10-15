// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func connectionTask(t *testing.T, op, target string) *TCPTask {
	t.Helper()
	task := &TCPTask{
		Task: &Task{}, Host: "127.0.0.1", Port: "1", Timeout: "100ms",
		SuccessWhen: []*TCPSuccess{{Connection: []*TCPConnectionSuccess{{Op: op, Target: target}}}},
	}
	task.SetChild(task)
	require.NoError(t, task.CheckTask())
	return task
}

func connectionError(state string) error {
	switch state {
	case "refused":
		return &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.ECONNREFUSED)}
	case "timeout":
		return &net.OpError{Op: "dial", Err: context.DeadlineExceeded}
	default:
		return nil
	}
}

func TestTCPConnectionMatrix(t *testing.T) {
	for _, actual := range []string{"established", "refused", "timeout"} {
		for _, expected := range []string{"established", "refused", "timeout"} {
			for _, op := range []string{"eq", "ne"} {
				for _, logic := range []string{"and", "or"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s", actual, op, expected, logic), func(t *testing.T) {
						task := connectionTask(t, op, expected)
						task.SuccessWhenLogic = logic
						err := task.runWithNetwork(context.Background(), nil,
							func(context.Context, string, string) (net.Conn, error) {
								if actual == "established" {
									client, server := net.Pipe()
									require.NoError(t, server.Close())
									return client, nil
								}
								return nil, connectionError(actual)
							})
						require.NoError(t, err)
						tags, fields := task.GetResults()
						want := (actual == expected) == (op == "eq")
						assert.Equal(t, want, tags["status"] == "OK")
						assert.Equal(t, want, fields["success"] == int64(1))
						assert.Equal(t, actual, fields["connection_status"])
						assert.Equal(t, int64(100000), fields["effective_timeout"])
						assert.GreaterOrEqual(t, task.reqCost, time.Duration(0))
						if want {
							assert.NotContains(t, fields, "fail_reason")
						}
					})
				}
			}
		}
	}
}

func TestTCPConnectionValidation(t *testing.T) {
	for _, rule := range []string{
		`null`, `{}`, `{"op":"lt","target":"established"}`, `{"op":"eq","target":"unknown"}`,
		`{"op":"ne","target":""}`, `{"op":"is","target":"refused"}`,
	} {
		t.Run(rule, func(t *testing.T) {
			task, err := NewTask(`{"host":"localhost","port":"80","success_when":[{"connection":[`+rule+`]}]}`, &TCPTask{})
			require.NoError(t, err)
			require.Error(t, task.CheckTask())
		})
	}
	for _, timeout := range []string{"0s", "-1s", "invalid"} {
		task := connectionTask(t, "eq", "timeout")
		task.Timeout = timeout
		require.Error(t, task.CheckTask())
	}
}

func TestTCPConnectionLogicAndExecutionErrors(t *testing.T) {
	for _, logic := range []string{"and", "or"} {
		task := connectionTask(t, "eq", "refused")
		task.SuccessWhenLogic = logic
		task.SuccessWhen = append(task.SuccessWhen, &TCPSuccess{ResponseTime: []*TCPResponseTime{{Target: "1ms"}}})
		require.NoError(t, task.CheckTask())
		task.connectionStatus, task.reqCost = "refused", time.Second
		tags, _ := task.GetResults()
		assert.Equal(t, logic == "or", tags["status"] == "OK")
		for _, phase := range []string{"request", "message", "traceroute"} {
			task.reqError, task.messageError, task.tracerouteError = "", "", ""
			switch phase {
			case "request":
				task.reqError = "DNS/platform error"
			case "message":
				task.messageError = "read error"
			case "traceroute":
				task.tracerouteError = "route error"
			}
			tags, fields := task.GetResults()
			assert.Equal(t, "FAIL", tags["status"], "%s/%s", logic, phase)
			assert.Equal(t, int64(-1), fields["success"])
		}
	}
}

func TestTCPConnectionErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{nil, "established"}, {connectionError("refused"), "refused"}, {connectionError("timeout"), "timeout"},
		{context.Canceled, ""}, {syscall.ENETUNREACH, ""}, {syscall.EHOSTUNREACH, ""},
		{syscall.ETIMEDOUT, "timeout"}, {syscall.EAGAIN, ""},
		{&net.OpError{Op: "dial", Err: os.NewSyscallError("connect", syscall.EAGAIN)}, ""},
		{&net.DNSError{Err: "timeout", IsTimeout: true}, ""}, {errors.New("connection refused"), ""},
	} {
		assert.Equal(t, tc.want, tcpConnectionStatus(tc.err), "%v", tc.err)
	}
	if runtime.GOOS == "windows" {
		assert.Equal(t, "refused", tcpConnectionStatus(os.NewSyscallError("connect", syscall.Errno(10061))))
		assert.Equal(t, "timeout", tcpConnectionStatus(os.NewSyscallError("connect", syscall.Errno(10060))))
	}
}

func TestTCPConnectionDialErrorsCannotBeHidden(t *testing.T) {
	for _, dialErr := range []error{syscall.EAGAIN, syscall.ENETUNREACH, context.Canceled} {
		task := connectionTask(t, "eq", "timeout")
		task.SuccessWhenLogic = "or"
		task.SuccessWhen[0].ResponseTime = []*TCPResponseTime{{Target: "1s"}}
		require.NoError(t, task.CheckTask())
		require.NoError(t, task.runWithNetwork(context.Background(), nil,
			func(context.Context, string, string) (net.Conn, error) {
				return nil, &net.OpError{Op: "dial", Err: os.NewSyscallError("connect", dialErr)}
			}))
		tags, fields := task.GetResults()
		assert.Equal(t, "FAIL", tags["status"])
		assert.Empty(t, fields["connection_status"])
		assert.NotEmpty(t, fields["fail_reason"])
	}
}

func TestTCPConnectionSharedBudget(t *testing.T) {
	task := connectionTask(t, "eq", "timeout")
	task.Host = "budget.example"
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	deadline, _ := ctx.Deadline()
	err := task.runWithNetwork(ctx,
		func(dnsCtx context.Context, network, host string) ([]net.IP, error) {
			end, _ := dnsCtx.Deadline()
			assert.Equal(t, deadline, end)
			time.Sleep(20 * time.Millisecond)
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		}, func(dialCtx context.Context, network, address string) (net.Conn, error) {
			end, _ := dialCtx.Deadline()
			assert.Equal(t, deadline, end)
			assert.Equal(t, "127.0.0.1:1", address)
			<-dialCtx.Done()
			return nil, dialCtx.Err()
		})
	require.NoError(t, err)
	assert.Positive(t, task.reqDNSCost)
	assert.Positive(t, task.reqCost)
	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Greater(t, fields["response_time_with_dns"], fields["response_time"])
}

func TestTCPConnectionDNSFailureIsNotConnectionTimeout(t *testing.T) {
	for _, dnsErr := range []error{context.DeadlineExceeded, context.Canceled, errors.New("no records"), nil} {
		task := connectionTask(t, "ne", "established")
		task.Host, task.SuccessWhenLogic = "broken.example", "or"
		task.SuccessWhen[0].ResponseTime = []*TCPResponseTime{{Target: "1s"}}
		require.NoError(t, task.CheckTask())
		err := task.runWithNetwork(context.Background(),
			func(context.Context, string, string) ([]net.IP, error) { return nil, dnsErr },
			func(context.Context, string, string) (net.Conn, error) {
				t.Fatal("DNS failure must prevent dialing")
				return nil, nil
			})
		require.NoError(t, err)
		tags, _ := task.GetResults()
		assert.Equal(t, "FAIL", tags["status"])
		assert.Empty(t, task.connectionStatus)
		assert.GreaterOrEqual(t, task.reqDNSCost, time.Duration(0))
	}
}

func TestTCPConnectionLegacyCompatibility(t *testing.T) {
	oldLookup := lookupIP
	t.Cleanup(func() { lookupIP = oldLookup })
	lookupIP = func(string) ([]net.IP, error) {
		time.Sleep(time.Millisecond)
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	}
	for _, state := range []string{"refused", "timeout"} {
		task := connectionTask(t, "eq", state)
		task.Host = "legacy.example"
		task.SuccessWhen = []*TCPSuccess{{ResponseTime: []*TCPResponseTime{{Target: "1s"}}}}
		require.NoError(t, task.CheckTask())
		assert.False(t, task.sharedTimeout)
		require.NoError(t, task.runWithNetwork(context.Background(), nil,
			func(context.Context, string, string) (net.Conn, error) { return nil, connectionError(state) }))
		tags, fields := task.GetResults()
		assert.Equal(t, "FAIL", tags["status"])
		assert.Equal(t, state, fields["connection_status"])
		assert.Positive(t, task.reqDNSCost)
	}
}

func TestTCPConnectionDNSDeadline(t *testing.T) {
	task := connectionTask(t, "eq", "timeout")
	task.Host = "slow.example"
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.NoError(t, task.runWithNetwork(ctx,
		func(ctx context.Context, _, _ string) ([]net.IP, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}, func(context.Context, string, string) (net.Conn, error) {
			t.Fatal("DNS deadline must prevent dialing")
			return nil, nil
		}))
	tags, fields := task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Empty(t, fields["connection_status"])
	assert.Positive(t, fields["response_time_with_dns"])
}

func TestTCPConnectionMessageDeadline(t *testing.T) {
	task := connectionTask(t, "eq", "established")
	task.Message, task.SuccessWhenLogic = "ping", "or"
	client, server := net.Pipe()
	t.Cleanup(func() { assert.NoError(t, server.Close()) })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	// A blocked write must expire within the shared budget and preserve established.
	require.NoError(t, task.runWithNetwork(ctx, nil,
		func(context.Context, string, string) (net.Conn, error) { return client, nil }))
	tags, fields := task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Equal(t, "established", fields["connection_status"])
	assert.NotEmpty(t, fields["message_error"])
	assert.NotContains(t, fields, "connection_error")
	_, err := server.Read(make([]byte, 1))
	require.Error(t, err, "client must be closed after deadline")
}

func TestTCPConnectionTracerouteErrorDoesNotOverwriteConnection(t *testing.T) {
	previousTrace := runTracerouteIP
	t.Cleanup(func() { runTracerouteIP = previousTrace })
	runTracerouteIP = func(string, *TracerouteOption) ([]*Route, error) { return nil, errors.New("route unavailable") }
	task := connectionTask(t, "eq", "refused")
	task.EnableTraceroute, task.SuccessWhenLogic = true, "or"
	require.NoError(t, task.runWithNetwork(context.Background(), nil,
		func(context.Context, string, string) (net.Conn, error) { return nil, connectionError("refused") }))
	tags, fields := task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Equal(t, "refused", fields["connection_status"])
	assert.NotEmpty(t, fields["connection_error"])
	assert.Equal(t, "route unavailable", fields["traceroute_error"])
}

func TestTCPConnectionRealDroppedPackets(t *testing.T) {
	address := os.Getenv("TCP_CONNECTION_DROP_ADDRESS")
	if address == "" {
		t.Skip("set TCP_CONNECTION_DROP_ADDRESS to a controlled endpoint that drops TCP SYN packets")
	}
	task := connectionTask(t, "eq", "timeout")
	var err error
	task.Host, task.Port, err = net.SplitHostPort(address)
	require.NoError(t, err)
	require.NoError(t, task.Run())
	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, "timeout", fields["connection_status"])
	assert.Positive(t, fields["response_time"])
}

func TestTCPConnectionRealNetworkAndRepeatedRun(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, listener.Close()) })
	task := connectionTask(t, "eq", "established")
	task.Host, task.Port, err = net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err)
	require.NoError(t, task.Run())
	tags, _ := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	conn, err := listener.Accept()
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	closed, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, task.Port, err = net.SplitHostPort(closed.Addr().String())
	require.NoError(t, err)
	require.NoError(t, closed.Close())
	task.SuccessWhen[0].Connection[0].Target = "refused"
	task.reqDNSCost, task.responseMessage, task.tracerouteError = time.Second, "stale", "stale"
	require.NoError(t, task.Run())
	tags, fields := task.GetResults()
	assert.Equal(t, "OK", tags["status"])
	assert.Equal(t, "refused", fields["connection_status"])
	assert.Zero(t, task.reqDNSCost)
	assert.NotContains(t, fields, "response_message")
	assert.NotContains(t, fields, "traceroute_error")

	task.SetBeforeRun(func(*Task) error { return errors.New("platform unavailable") })
	require.NoError(t, task.Run())
	tags, fields = task.GetResults()
	assert.Equal(t, "FAIL", tags["status"])
	assert.Empty(t, fields["connection_status"])
	assert.Empty(t, tags["dest_ip"])
	assert.Zero(t, fields["response_time"])
	assert.NotContains(t, fields, "connection_error")
}
