// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

//go:build !windows
// +build !windows

package dialtesting

import (
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type receivePacket struct {
	from           *net.IPAddr
	packetRecvTime time.Time
	buf            []byte
}

// Traceroute specified host with max hops and timeout.
type Traceroute struct {
	Host    string
	Hops    int
	Retry   int
	Timeout time.Duration

	routes           []*Route
	response         chan *Response
	stopCh           chan interface{}
	packetCh         chan *Packet
	receivePacketsCh chan *receivePacket
	id               uint32
}

// init config: hops, retry, timeout should not be greater than the max value.
func (t *Traceroute) init() {
	if t.Hops <= 0 {
		t.Hops = 30
	} else if t.Hops > MaxHops {
		t.Hops = MaxHops
	}

	if t.Retry <= 0 {
		t.Retry = 3
	} else if t.Retry > MaxRetry {
		t.Retry = MaxRetry
	}

	if t.Timeout <= 0 {
		t.Timeout = 1 * time.Second
	} else if t.Timeout > MaxTimeout {
		t.Timeout = MaxTimeout
	}

	t.routes = make([]*Route, 0)

	t.response = make(chan *Response)
	t.stopCh = make(chan interface{})
	t.packetCh = make(chan *Packet)
	t.receivePacketsCh = make(chan *receivePacket, 5000)

	t.id = t.getRandomID()
}

// getRandomID generate random id, max 60000.
func (t *Traceroute) getRandomID() uint32 {
	rand.Seed(time.Now().UnixNano())
	return uint32(rand.Intn(60000)) //nolint:gosec
}

func (t *Traceroute) Run() error {
	var runError error
	ips, err := net.LookupIP(t.Host)
	if err != nil {
		return err
	}

	t.init()

	if len(ips) == 0 {
		return fmt.Errorf("invalid host: %s", t.Host)
	}
	ip := ips[0]

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := t.startTrace(ip); err != nil {
			runError = fmt.Errorf("start trace error: %w", err)
		}
	}()

	go func() {
		defer wg.Done()
		if err := t.listenICMP(); err != nil {
			runError = fmt.Errorf("listen icmp error: %w", err)
		}
	}()
	wg.Wait()
	return runError
}

func (t *Traceroute) startTrace(ip net.IP) error {
	var icmpResponse *Response

	defer close(t.stopCh)

	for i := 1; i <= t.Hops; i++ {
		isReply := false
		routeItems := []*RouteItem{}
		responseTimes := []float64{}
		var minCost, maxCost time.Duration
		var failed int
		for j := 0; j < t.Retry; j++ {
			if err := t.sendICMP(ip, i); err != nil {
				return err
			}
			icmpResponse = <-t.response
			routeItem := &RouteItem{
				IP:           icmpResponse.From.String(),
				ResponseTime: float64(icmpResponse.ResponseTime.Microseconds()),
			}

			if icmpResponse.fail {
				routeItem.IP = "*"
				failed++
			} else {
				if icmpResponse.From.String() == ip.String() {
					isReply = true
				}

				if icmpResponse.ResponseTime > 0 {
					if minCost == 0 || minCost > icmpResponse.ResponseTime {
						minCost = icmpResponse.ResponseTime
					}

					if maxCost == 0 || maxCost < icmpResponse.ResponseTime {
						maxCost = icmpResponse.ResponseTime
					}

					responseTimes = append(responseTimes, float64(icmpResponse.ResponseTime.Microseconds()))
				}
			}

			routeItems = append(routeItems, routeItem)
		}

		loss, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", float64(failed)*100/float64(t.Retry)), 64)

		route := &Route{
			Total:   t.Retry,
			Failed:  failed,
			Loss:    loss,
			MinCost: float64(minCost.Microseconds()),
			AvgCost: mean(responseTimes),
			MaxCost: float64(maxCost.Microseconds()),
			StdCost: std(responseTimes),
			Items:   routeItems,
		}
		t.routes = append(t.routes, route)

		if isReply {
			return nil
		}
	}

	return nil
}

func (t *Traceroute) dealPacket() {
	for {
		select {
		case <-t.stopCh:
			return
		case packet, ok := <-t.packetCh:
			if ok {
				for {
					p := <-t.receivePacketsCh
					if p.packetRecvTime.Sub(packet.startTime) > t.Timeout {
						t.response <- &Response{fail: true}
						break
					}
					if p.from == nil || p.from.IP == nil || len(p.buf) == 0 {
						continue
					}
					msg, err := icmp.ParseMessage(1, p.buf)
					if err != nil {
						continue
					}

					if msg.Type == ipv4.ICMPTypeEchoReply {
						echo := msg.Body.(*icmp.Echo)

						if echo.ID != packet.ID {
							continue
						}
					} else {
						icmpData := t.getReplyData(msg)
						if len(icmpData) < ipv4.HeaderLen {
							continue
						}

						var packetID int

						func() {
							switch icmpData[0] >> 4 {
							case ipv4.Version:
								header, err := ipv4.ParseHeader(icmpData)
								if err != nil {
									return
								}
								packetID = header.ID
							case ipv6.Version:
								header, err := ipv6.ParseHeader(icmpData)
								if err != nil {
									return
								}

								packetID = header.FlowLabel
							}
						}()
						if packetID != packet.ID {
							continue
						}
					}

					t.response <- &Response{From: p.from.IP, ResponseTime: p.packetRecvTime.Sub(packet.startTime)}
					break
				}
			}
		}
	}
}

func (t *Traceroute) listenICMP() error {
	var addr *net.IPAddr
	conn, err := net.ListenIP("ip4:icmp", addr)
	if err != nil {
		return err
	}

	defer func() {
		if err := conn.Close(); err != nil {
			_ = err // pass
		}
	}()

	go t.dealPacket()

	for {
		select {
		case <-t.stopCh:
			return nil
		default:
		}

		buf := make([]byte, 1500)
		deadLine := time.Now().Add(time.Second)

		if t.Timeout > 0 && t.Timeout < 10*time.Second { // max 10s
			deadLine = time.Now().Add(t.Timeout)
		}

		if err := conn.SetDeadline(deadLine); err != nil {
			return err
		}

		if n, from, err := conn.ReadFromIP(buf); err != nil {
			return err
		} else {
			t.receivePacketsCh <- &receivePacket{
				from:           from,
				packetRecvTime: time.Now(),
				buf:            buf[:n],
			}
		}
	}
}

func (t *Traceroute) getReplyData(msg *icmp.Message) []byte {
	switch b := msg.Body.(type) {
	case *icmp.TimeExceeded:
		return b.Data
	case *icmp.DstUnreach:
		return b.Data
	case *icmp.ParamProb:
		return b.Data
	}

	return nil
}

func (t *Traceroute) sendICMP(ip net.IP, ttl int) error {
	if ip.To4() == nil {
		return fmt.Errorf("support ip version 4 only")
	}
	id := uint16(atomic.AddUint32(&t.id, 1))

	dst := net.ParseIP(ip.String())
	echoBody := &icmp.Echo{
		ID:  int(id),
		Seq: int(id),
	}
	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: echoBody,
	}

	p, err := msg.Marshal(nil)
	if err != nil {
		return err
	}

	ipHeader := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TotalLen: ipv4.HeaderLen + len(p),
		TOS:      16,
		ID:       int(id),
		Dst:      dst,
		Protocol: 1,
		TTL:      ttl,
	}

	buf, err := ipHeader.Marshal()
	if err != nil {
		return err
	}

	buf = append(buf, p...)

	conn, err := net.ListenIP("ip4:icmp", nil)
	if err != nil {
		return err
	}
	defer func() {
		if err := conn.Close(); err != nil {
			_ = err // pass
		}
	}()

	raw, err := conn.SyscallConn()
	if err != nil {
		return err
	}

	_ = raw.Control(func(fd uintptr) {
		err = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)
	})

	if err != nil {
		return err
	}

	t.packetCh <- &Packet{ID: echoBody.ID, Dst: ipHeader.Dst, startTime: time.Now()}

	_, err = conn.WriteToIP(buf, &net.IPAddr{IP: dst})

	if err != nil {
		return err
	}

	return nil
}

func TracerouteIP(ip string, opt *TracerouteOption) (routes []*Route, err error) {
	defaultTimeout := 30 * time.Millisecond
	if opt == nil {
		opt = &TracerouteOption{
			Hops:    30,
			Retry:   2,
			timeout: defaultTimeout,
		}
	} else {
		if timeout, err := time.ParseDuration(opt.Timeout); err != nil {
			opt.timeout = defaultTimeout
		} else {
			opt.timeout = timeout
		}
	}

	traceroute := Traceroute{
		Host:    ip,
		Hops:    opt.Hops,
		Retry:   opt.Retry,
		Timeout: opt.timeout,
	}

	err = traceroute.Run()

	if err != nil {
		return
	}

	routes = traceroute.routes

	return routes, err
}
