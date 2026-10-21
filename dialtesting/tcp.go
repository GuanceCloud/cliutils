// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"text/template"
	"time"
)

var (
	_ TaskChild = (*TCPTask)(nil)
	_ ITask     = (*TCPTask)(nil)
)

const defaultTCPTimeout = 30 * time.Second

type TCPResponseTime struct {
	IsContainDNS bool   `json:"is_contain_dns"`
	Target       string `json:"target"`

	targetTime time.Duration
}

type TCPSuccess struct {
	Connection      []*TCPConnectionSuccess `json:"connection,omitempty"`
	ResponseTime    []*TCPResponseTime      `json:"response_time,omitempty"`
	Hops            []*ValueSuccess         `json:"hops,omitempty"`
	ResponseMessage []*SuccessOption        `json:"response_message,omitempty"`
}

// TCPConnectionSuccess compares the outcome of a connection attempt.
type TCPConnectionSuccess struct {
	Op     string `json:"op"`
	Target string `json:"target"`
}

func (c *TCPConnectionSuccess) check(status string) error {
	if status == "" || (c.Op == "eq" && status != c.Target) || (c.Op == "ne" && status == c.Target) {
		return fmt.Errorf("TCP connection: expect %s %s, got %q", c.Op, c.Target, status)
	}
	return nil
}

type TCPTask struct {
	*Task
	Host             string            `json:"host"`
	Port             string            `json:"port"`
	Message          string            `json:"message"`
	Timeout          string            `json:"timeout"`
	EnableTraceroute bool              `json:"enable_traceroute"`
	TracerouteConfig *TracerouteOption `json:"traceroute_config"`
	SuccessWhen      []*TCPSuccess     `json:"success_when"`
	SuccessWhenLogic string            `json:"success_when_logic"`

	reqCost          time.Duration
	reqCostAvailable bool
	reqDNSCost       time.Duration
	reqError         string
	destIP           string
	responseMessage  string
	messageAvailable bool
	timeout          time.Duration
	traceroute       []*Route
	connectionStatus string
	connectionError  string
	messageError     string
	tracerouteError  string
	sharedTimeout    bool
	cancelMu         sync.Mutex
	cancelRun        context.CancelFunc

	rawTask *TCPTask
}

func (t *TCPTask) init() error {
	t.sharedTimeout = false
	if len(t.Timeout) == 0 {
		t.timeout = 10 * time.Second
	} else {
		if timeout, err := time.ParseDuration(t.Timeout); err != nil {
			return err
		} else {
			t.timeout = timeout
		}
	}

	if len(t.SuccessWhen) == 0 {
		return errors.New(`no any check rule`)
	}

	for _, checker := range t.SuccessWhen {
		if checker == nil {
			return errors.New("TCP success rule must not be null")
		}
		for _, c := range checker.Connection {
			if c == nil || (c.Op != "eq" && c.Op != "ne") {
				return errors.New("TCP connection operator must be eq or ne")
			}
			switch c.Target {
			case "established", "refused", "timeout":
			default:
				return fmt.Errorf("invalid TCP connection target %q", c.Target)
			}
			t.sharedTimeout = true
		}
		if checker.ResponseTime != nil {
			for _, v := range checker.ResponseTime {
				du, err := time.ParseDuration(v.Target)
				if err != nil {
					return err
				}
				v.targetTime = du
			}
		}

		// if [checker.Hops] is not nil, set traceroute to be true
		if checker.Hops != nil {
			t.EnableTraceroute = true
		}

		t.EnableTraceroute = IsEnabledTraceroute(t.EnableTraceroute)

		for _, v := range checker.ResponseMessage {
			err := genReg(v)
			if err != nil {
				return err
			}
		}
	}

	if t.sharedTimeout && t.timeout <= 0 {
		return errors.New("TCP timeout must be positive")
	}

	return nil
}

func (t *TCPTask) check() error {
	if len(t.Host) == 0 {
		return errors.New("host should not be empty")
	}

	if len(t.Port) == 0 {
		return errors.New("port should not be empty")
	}

	return nil
}

func (t *TCPTask) checkResult() (reasons []string, succFlag bool) {
	// Execution failures must never be hidden by a passing OR assertion.
	for _, err := range []string{t.reqError, t.messageError, t.tracerouteError} {
		if err != "" {
			reasons = append(reasons, err)
		}
	}
	if len(reasons) > 0 {
		return reasons, false
	}
	for _, chk := range t.SuccessWhen {
		for _, c := range chk.Connection {
			if err := c.check(t.connectionStatus); err != nil {
				reasons = append(reasons, err.Error())
			} else {
				succFlag = true
			}
		}
		// check response time
		if chk.ResponseTime != nil {
			for _, v := range chk.ResponseTime {
				if t.sharedTimeout && !t.reqCostAvailable {
					reasons = append(reasons, "TCP response time unavailable: no connection attempt")
					continue
				}
				reqCost := t.reqCost

				if v.IsContainDNS {
					reqCost += t.reqDNSCost
				}

				if reqCost >= v.targetTime {
					reasons = append(reasons,
						fmt.Sprintf("TCP response time(%v) larger equal than %v", reqCost, v.targetTime))
				} else if v.targetTime > 0 {
					succFlag = true
				}
			}
		}

		// check message
		if chk.ResponseMessage != nil {
			for _, v := range chk.ResponseMessage {
				if t.sharedTimeout && !t.messageAvailable {
					reasons = append(reasons, "TCP response message unavailable: no response received")
					continue
				}
				if err := v.check(t.responseMessage, "response message"); err != nil {
					reasons = append(reasons, err.Error())
				} else {
					succFlag = true
				}
			}
		}

		// check traceroute
		if t.EnableTraceroute {
			hops := float64(len(t.traceroute))

			if hops == 0 {
				reasons = append(reasons, "traceroute failed with no hops")
			} else {
				for _, v := range chk.Hops {
					if err := v.check(hops); err != nil {
						reasons = append(reasons, fmt.Sprintf("traceroute hops check failed: %s", err.Error()))
					} else {
						succFlag = true
					}
				}
			}
		}
	}

	return reasons, succFlag
}

func (t *TCPTask) getResults() (tags map[string]string, fields map[string]interface{}) {
	tags = map[string]string{
		"name":      t.Name,
		"dest_host": t.Host,
		"dest_port": t.Port,
		"dest_ip":   t.destIP,
		"status":    "FAIL",
		"proto":     "tcp",
	}

	responseTime := int64(t.reqCost) / 1000                     // us
	responseTimeWithDNS := int64(t.reqCost+t.reqDNSCost) / 1000 // us

	fields = map[string]interface{}{
		"response_time":          responseTime,
		"response_time_with_dns": responseTimeWithDNS,
		"success":                int64(-1),
		"connection_status":      t.connectionStatus,
		"effective_timeout":      t.timeout.Microseconds(),
	}
	for key, value := range map[string]string{
		"connection_error": t.connectionError,
		"message_error":    t.messageError,
		"traceroute_error": t.tracerouteError,
	} {
		if value != "" {
			fields[key] = value
		}
	}

	if t.responseMessage != "" {
		fields["response_message"] = t.responseMessage
	}

	if t.EnableTraceroute {
		fields["hops"] = 0
		if t.traceroute == nil {
			fields["traceroute"] = "[]"
		} else {
			tracerouteData, err := json.Marshal(t.traceroute)
			if err == nil && len(tracerouteData) > 0 {
				fields["traceroute"] = string(tracerouteData)
				fields["hops"] = len(t.traceroute)
			} else {
				fields["traceroute"] = "[]"
			}
		}
	}

	for k, v := range t.Tags {
		tags[k] = v
	}

	message := map[string]interface{}{}

	reasons, succFlag := t.checkResult()

	switch t.SuccessWhenLogic {
	case "or":
		if succFlag && t.reqError == "" {
			tags["status"] = "OK"
			fields["success"] = int64(1)
			message["response_time"] = responseTime
		} else {
			message[`fail_reason`] = strings.Join(reasons, `;`)
			fields[`fail_reason`] = strings.Join(reasons, `;`)
		}
	default:
		if len(reasons) != 0 {
			message[`fail_reason`] = strings.Join(reasons, `;`)
			fields[`fail_reason`] = strings.Join(reasons, `;`)
		} else {
			message["response_time"] = responseTime
		}

		if t.reqError == "" && len(reasons) == 0 {
			tags["status"] = "OK"
			fields["success"] = int64(1)
		}
	}

	data, err := json.Marshal(message)
	if err != nil {
		fields[`message`] = err.Error()
	}

	if len(data) > MaxMsgSize {
		fields[`message`] = string(data[:MaxMsgSize])
	} else {
		fields[`message`] = string(data)
	}

	return tags, fields
}

func (t *TCPTask) metricName() string {
	return `tcp_dial_testing`
}

func (t *TCPTask) clear() {
	t.reqCost = 0
	t.reqCostAvailable = false
	t.reqDNSCost = 0
	t.reqError = ""
	t.destIP = ""
	t.responseMessage = ""
	t.messageAvailable = false
	t.connectionStatus = ""
	t.connectionError = ""
	t.messageError = ""
	t.tracerouteError = ""
	t.traceroute = nil
}

func (t *TCPTask) run() error {
	var d net.Dialer
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	if t.sharedTimeout {
		t.cancelMu.Lock()
		t.cancelRun = cancelRun
		t.cancelMu.Unlock()
		defer func() {
			t.cancelMu.Lock()
			defer t.cancelMu.Unlock()
			if err := runCtx.Err(); err != nil {
				t.reqError = err.Error()
			}
			t.cancelRun = nil
		}()
	}
	ctx, cancel := context.WithTimeout(runCtx, t.timeout)
	defer cancel()
	var stopClose func() bool
	defer func() {
		if stopClose != nil {
			stopClose()
		}
	}()
	return t.runWithNetwork(ctx, net.DefaultResolver.LookupIP,
		func(ctx context.Context, network, address string) (net.Conn, error) {
			conn, err := d.DialContext(ctx, network, address)
			if err == nil && t.sharedTimeout {
				// Explicit cancellation also interrupts message I/O, independently of the connection budget.
				stopClose = context.AfterFunc(runCtx, func() { _ = conn.Close() }) //nolint:errcheck
			}
			return conn, err
		})
}

func (t *TCPTask) runWithNetwork(ctx context.Context,
	resolve func(context.Context, string, string) ([]net.IP, error),
	dial func(context.Context, string, string) (net.Conn, error),
) error {
	hostIP := net.ParseIP(t.Host)

	if hostIsName := hostIP == nil; hostIsName { // host name
		start := time.Now()
		var ips []net.IP
		var err error
		if t.sharedTimeout {
			ips, err = resolve(ctx, "ip", t.Host)
		} else {
			// Preserve the resolver and timing semantics of tasks without connection assertions.
			ips, err = lookupIP(t.Host)
		}
		t.reqDNSCost = time.Since(start)
		if err != nil {
			t.reqError = fmt.Sprintf("DNS lookup failed: %s", err)
			return nil
		}
		if len(ips) == 0 {
			t.reqError = fmt.Sprintf("invalid host: %s, found no ip record", t.Host)
			return nil
		}
		hostIP = ips[0]
	}

	t.destIP = hostIP.String()
	tcpIPAddr := net.JoinHostPort(hostIP.String(), t.Port)
	if t.sharedTimeout && ctx.Err() != nil {
		t.reqError = fmt.Sprintf("TCP connection budget exhausted before dialing: %s", ctx.Err())
		return nil
	}

	start := time.Now()
	conn, err := dial(ctx, "tcp", tcpIPAddr)
	t.reqCost = time.Since(start)
	t.reqCostAvailable = true

	if err != nil {
		t.connectionStatus = tcpConnectionStatus(err)
		t.connectionError = err.Error()
		if !t.sharedTimeout || t.connectionStatus == "" {
			t.reqError = err.Error()
		}
	} else {
		t.connectionStatus = "established"
		if t.Message != "" { // send message and get response
			if err := conn.SetDeadline(time.Now().Add(defaultTCPTimeout)); err != nil {
				t.messageError = err.Error()
			} else if _, err := conn.Write([]byte(t.Message)); err != nil {
				t.messageError = err.Error()
			} else {
				buf := make([]byte, 1024)
				if n, err := conn.Read(buf); err != nil {
					t.messageError = err.Error()
				} else {
					t.responseMessage = string(buf[:n])
					t.messageAvailable = true
				}
			}
		}

		_ = conn.Close() //nolint:errcheck
	}

	if t.EnableTraceroute {
		routes, err := runTracerouteIP(hostIP.String(), t.TracerouteConfig)
		if err != nil {
			t.tracerouteError = err.Error()
		} else {
			t.traceroute = routes
		}
	}

	return nil
}

func tcpConnectionStatus(err error) string {
	if err == nil {
		return "established"
	}
	if errors.Is(err, context.Canceled) {
		return ""
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return ""
	}
	// Windows reports Winsock WSAECONNREFUSED (10061), not syscall.ECONNREFUSED.
	if errors.Is(err, syscall.ECONNREFUSED) || (runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10061))) {
		return "refused"
	}
	// Winsock WSAETIMEDOUT (10060) does not implement syscall.Errno.Timeout.
	if errors.Is(err, syscall.ETIMEDOUT) || (runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(10060))) {
		return "timeout"
	}
	// Other syscall failures, including EAGAIN, are execution errors even if Timeout reports true.
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return ""
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "timeout"
	}
	return ""
}

func (t *TCPTask) stop() {
	t.cancelMu.Lock()
	defer t.cancelMu.Unlock()
	if t.cancelRun != nil {
		t.cancelRun()
	}
}

func (t *TCPTask) class() string {
	return ClassTCP
}

func (t *TCPTask) getHostName() ([]string, error) {
	return []string{t.Host}, nil
}

func (t *TCPTask) getVariableValue(variable Variable) (string, error) {
	return "", errors.New("not support")
}

func (t *TCPTask) getRawTask(taskString string) (string, error) {
	task := TCPTask{}

	if err := json.Unmarshal([]byte(taskString), &task); err != nil {
		return "", fmt.Errorf("unmarshal tcp task failed: %w", err)
	}

	task.Task = nil

	bytes, _ := json.Marshal(&task)
	return string(bytes), nil
}

func (t *TCPTask) initTask() {
	if t.Task == nil {
		t.Task = &Task{}
	}
}

func (t *TCPTask) setReqError(err string) {
	t.reqError = err
}

func (t *TCPTask) renderTemplate(fm template.FuncMap) error {
	if t.rawTask == nil {
		task := &TCPTask{}
		if err := t.NewRawTask(task); err != nil {
			return fmt.Errorf("new raw task failed: %w", err)
		}
		t.rawTask = task
	}

	task := t.rawTask
	if task == nil {
		return errors.New("raw task is nil")
	}

	// host
	if text, err := t.GetParsedString(task.Host, fm); err != nil {
		return fmt.Errorf("render host failed: %w", err)
	} else {
		t.Host = text
	}

	// port
	if text, err := t.GetParsedString(task.Port, fm); err != nil {
		return fmt.Errorf("render port failed: %w", err)
	} else {
		t.Port = text
	}

	// message
	if text, err := t.GetParsedString(task.Message, fm); err != nil {
		return fmt.Errorf("render message failed: %w", err)
	} else {
		t.Message = text
	}

	return nil
}
