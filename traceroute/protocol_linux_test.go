// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

//go:build linux

package traceroute

import (
	"context"
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/bpf"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func TestProbeUDPValidation(t *testing.T) {
	canceledContext, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ProbeUDP(canceledContext, net.ParseIP("192.0.2.1"), 53, 1, time.Second)
	require.ErrorIs(t, err, context.Canceled)

	_, err = ProbeUDP(context.Background(), net.ParseIP("2001:db8::1"), 53, 1, time.Second)
	require.ErrorContains(t, err, "must be IPv4")

	for _, target := range []string{"0.0.0.0", "224.0.0.1", "255.255.255.255"} {
		_, err = ProbeUDP(context.Background(), net.ParseIP(target), 53, 1, time.Second)
		require.ErrorContains(t, err, "unicast")
	}

	_, err = ProbeUDP(context.Background(), net.ParseIP("192.0.2.1"), 0, 1, time.Second)
	require.ErrorContains(t, err, "destination port")

	_, err = ProbeUDP(context.Background(), net.ParseIP("192.0.2.1"), 53, MaxUDPHops+1, time.Second)
	require.ErrorContains(t, err, "TTL is out of range")
}

func TestClosedUDPProber(t *testing.T) {
	prober := &UDPProber{}
	require.NoError(t, prober.Close())
	_, err := prober.Probe(context.Background(), net.ParseIP("192.0.2.1"), 53, 1, time.Second)
	require.ErrorContains(t, err, "closed")
}

func TestParseUDPICMPReply(t *testing.T) {
	target := net.ParseIP("203.0.113.10").To4()
	quoted := quotedTransportPacket(t, 17, target, 31000, 33434, 9, 0)
	packet := marshalICMP(t, ipv4.ICMPTypeTimeExceeded, &icmp.TimeExceeded{Data: quoted})

	reply, matched := parseUDPICMPReply(packet, &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
		target, 31000, 33434, 9, 2*time.Millisecond)
	require.True(t, matched)
	assert.Equal(t, "192.0.2.1", reply.ip.String())
	assert.Equal(t, 2*time.Millisecond, reply.rtt)
	assert.False(t, reply.reached)

	_, matched = parseUDPICMPReply(packet, &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
		target, 31001, 33434, 9, time.Millisecond)
	assert.False(t, matched)

	_, matched = parseUDPICMPReply(packet, &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
		target, 31000, 33434, 10, time.Millisecond)
	assert.False(t, matched)

	unreachable := marshalICMP(t, ipv4.ICMPTypeDestinationUnreachable, &icmp.DstUnreach{Data: quoted})
	reply, matched = parseUDPICMPReply(unreachable, &net.IPAddr{IP: target},
		target, 31000, 33434, 9, time.Millisecond)
	require.True(t, matched)
	assert.True(t, reply.reached)
}

func TestUDPPayloadUsesProbeIDAsQuotedLength(t *testing.T) {
	payload, udpLength, err := udpPayload(7)
	require.NoError(t, err)
	assert.Len(t, payload, 7)
	assert.Equal(t, uint16(15), udpLength)

	_, _, err = udpPayload(0)
	require.Error(t, err)
	_, _, err = udpPayload(MaxUDPProbePayload + 1)
	require.ErrorContains(t, err, "invalid")
}

func TestUDPLeaseRegistryQuarantine(t *testing.T) {
	now := time.Unix(100, 0)
	registry := udpLeaseRegistry{leases: make(map[udpLeaseKey]time.Time)}
	key := udpLeaseKey{
		target:     [net.IPv4len]byte{127, 0, 0, 1},
		targetPort: 33434,
		sourcePort: 31000,
	}

	require.True(t, registry.reserve(key, now))
	assert.False(t, registry.reserve(key, now))
	registry.finish(key, true, now)
	assert.False(t, registry.reserve(key, now.Add(MaxTimeout-time.Nanosecond)))

	differentEndpoint := key
	differentEndpoint.targetPort++
	require.True(t, registry.reserve(differentEndpoint, now))
	registry.finish(differentEndpoint, false, now)
	assert.True(t, registry.reserve(key, now.Add(MaxTimeout)))
}

func TestUDPLeaseRegistryConcurrentReservation(t *testing.T) {
	registry := udpLeaseRegistry{leases: make(map[udpLeaseKey]time.Time)}
	key := udpLeaseKey{
		target:     [net.IPv4len]byte{127, 0, 0, 1},
		targetPort: 33434,
		sourcePort: 31000,
	}
	start := make(chan struct{})
	var successes atomic.Int32
	var waitGroup sync.WaitGroup
	for range 32 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			if registry.reserve(key, time.Unix(100, 0)) {
				successes.Add(1)
			}
		}()
	}
	close(start)
	waitGroup.Wait()
	assert.Equal(t, int32(1), successes.Load())
}

func TestUDPLeaseRegistryDefersBulkCleanup(t *testing.T) {
	now := time.Unix(100, 0)
	registry := udpLeaseRegistry{leases: make(map[udpLeaseKey]time.Time)}
	for port := 1; port <= 10000; port++ {
		registry.leases[udpLeaseKey{sourcePort: uint16(port)}] = now.Add(-time.Second)
	}
	newKey := udpLeaseKey{sourcePort: 31000}
	require.True(t, registry.reserve(newKey, now))
	assert.Len(t, registry.leases, 10001, "reserve must not scan all unrelated leases")

	registry.cleanupExpiredAt(now)
	assert.Equal(t, map[udpLeaseKey]time.Time{newKey: {}}, registry.leases)
}

func TestUDPProbeSocketsUseDistinctPorts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	target := net.ParseIP("127.0.0.1").To4()
	ports := make(map[uint16]struct{})

	for range 64 {
		socket, err := openUDPProbeSocket(ctx, target, 33434)
		require.NoError(t, err)
		_, duplicate := ports[socket.key.sourcePort]
		assert.False(t, duplicate)
		ports[socket.key.sourcePort] = struct{}{}
		socket.sent = true
		socket.close()
	}
	assert.Len(t, ports, 64)
}

func TestLateUDPApplicationReplyDoesNotReachNextProbe(t *testing.T) {
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Close() })
	serverAddr := server.LocalAddr().(*net.UDPAddr)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	first, err := openUDPProbeSocket(ctx, serverAddr.IP, uint16(serverAddr.Port))
	require.NoError(t, err)
	firstPort := first.key.sourcePort
	first.sent = true
	first.close()

	second, err := openUDPProbeSocket(ctx, serverAddr.IP, uint16(serverAddr.Port))
	require.NoError(t, err)
	t.Cleanup(second.close)
	require.NotEqual(t, firstPort, second.key.sourcePort)

	writeErr := make(chan error, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		_, err := server.WriteToUDP([]byte("late"), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: int(firstPort)})
		writeErr <- err
	}()
	reply, err := waitUDPApplicationReply(ctx, second.conn, serverAddr.IP,
		uint16(serverAddr.Port), time.Now(), 50*time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, <-writeErr)
	assert.True(t, reply.timedOut)
}

func TestParseTCPICMPReply(t *testing.T) {
	target := net.ParseIP("203.0.113.10").To4()
	const sequence = 0x12345678
	quoted := quotedTransportPacket(t, 6, target, 31000, 443, 0, sequence)
	packet := marshalICMP(t, ipv4.ICMPTypeTimeExceeded, &icmp.TimeExceeded{Data: quoted})

	reply, matched := parseTCPICMPReply(packet, &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
		target, 31000, 443, sequence, 2*time.Millisecond)
	require.True(t, matched)
	assert.Equal(t, "192.0.2.1", reply.ip.String())
	assert.False(t, reply.reached)

	_, matched = parseTCPICMPReply(packet, &net.IPAddr{IP: net.ParseIP("192.0.2.1")},
		target, 31000, 443, sequence+1, time.Millisecond)
	assert.False(t, matched)
}

func TestMatchesICMPProbeUsesFullTokenAndTarget(t *testing.T) {
	target := net.ParseIP("203.0.113.10").To4()
	peer := net.ParseIP("192.0.2.1").To4()
	const id, sequence = 17, 23
	echoReply := marshalICMP(t, ipv4.ICMPTypeEchoReply, &icmp.Echo{ID: id, Seq: sequence})
	assert.True(t, matchesICMPProbe(echoReply, target, target, id, sequence))
	assert.False(t, matchesICMPProbe(echoReply, peer, target, id, sequence))
	assert.False(t, matchesICMPProbe(echoReply, target, target, id, sequence+1))

	inner, err := (&icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Body: &icmp.Echo{ID: id, Seq: sequence},
	}).Marshal(nil)
	require.NoError(t, err)
	header := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TotalLen: ipv4.HeaderLen + len(inner),
		Protocol: 1,
		Src:      net.ParseIP("192.0.2.20").To4(),
		Dst:      target,
	}
	encodedHeader, err := header.Marshal()
	require.NoError(t, err)
	timeExceeded := marshalICMP(t, ipv4.ICMPTypeTimeExceeded,
		&icmp.TimeExceeded{Data: append(encodedHeader, inner...)})
	assert.True(t, matchesICMPProbe(timeExceeded, peer, target, id, sequence))
	assert.False(t, matchesICMPProbe(timeExceeded, peer,
		net.ParseIP("203.0.113.11").To4(), id, sequence))
}

func TestTCPPacketEncodingAndResponseMatching(t *testing.T) {
	source := net.ParseIP("192.0.2.20").To4()
	target := net.ParseIP("203.0.113.10").To4()
	const sequence = 0x12345678
	segment := marshalTCPSYN(source, target, 31000, 443, sequence)

	assert.Equal(t, uint16(31000), binary.BigEndian.Uint16(segment[0:2]))
	assert.Equal(t, uint16(443), binary.BigEndian.Uint16(segment[2:4]))
	assert.Equal(t, uint32(sequence), binary.BigEndian.Uint32(segment[4:8]))
	assert.Equal(t, byte(tcpFlagSYN), segment[13])
	assert.Equal(t, uint16(0), tcpChecksum(source, target, segment))

	response := make([]byte, 20)
	binary.BigEndian.PutUint16(response[0:2], 443)
	binary.BigEndian.PutUint16(response[2:4], 31000)
	binary.BigEndian.PutUint32(response[8:12], sequence+1)
	response[12] = 5 << 4
	response[13] = tcpFlagSYN | tcpFlagACK
	header := &ipv4.Header{Src: target, Dst: source}
	assert.True(t, matchesTCPResponse(header, response, source, target, 31000, 443, sequence))

	response[13] = tcpFlagACK
	assert.False(t, matchesTCPResponse(header, response, source, target, 31000, 443, sequence))

	response[13] = tcpFlagRST | tcpFlagACK
	assert.True(t, matchesTCPResponse(header, response, source, target, 31000, 443, sequence))

	response[8]++
	assert.False(t, matchesTCPResponse(header, response, source, target, 31000, 443, sequence))
}

func TestTCPPacketFilter(t *testing.T) {
	local := net.ParseIP("192.0.2.20").To4()
	target := net.ParseIP("203.0.113.10").To4()
	instructions, err := tcpPacketFilter(local, target, 31000, 443)
	require.NoError(t, err)
	vm, err := bpf.NewVM(instructions)
	require.NoError(t, err)

	packet := tcpResponsePacket(t, target, local, 443, 31000, tcpFlagSYN|tcpFlagACK)
	accepted, err := vm.Run(packet)
	require.NoError(t, err)
	assert.Equal(t, tcpFilterSnapshotLength, accepted)

	packet = tcpResponsePacket(t, net.ParseIP("203.0.113.11").To4(), local,
		443, 31000, tcpFlagSYN|tcpFlagACK)
	accepted, err = vm.Run(packet)
	require.NoError(t, err)
	assert.Zero(t, accepted)

	packet = tcpResponsePacket(t, target, local, 443, 31000, tcpFlagACK)
	accepted, err = vm.Run(packet)
	require.NoError(t, err)
	assert.Zero(t, accepted)

	packet = tcpResponsePacket(t, target, local, 443, 31000, tcpFlagSYN|tcpFlagACK)
	packet[7] = 1
	accepted, err = vm.Run(packet)
	require.NoError(t, err)
	assert.Zero(t, accepted)
}

func tcpResponsePacket(t *testing.T, source, target net.IP, sourcePort, targetPort uint16,
	flags byte,
) []byte {
	t.Helper()
	header := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TotalLen: ipv4.HeaderLen + 20,
		Protocol: 6,
		Src:      source,
		Dst:      target,
	}
	encodedHeader, err := header.Marshal()
	require.NoError(t, err)
	tcpHeader := make([]byte, 20)
	binary.BigEndian.PutUint16(tcpHeader[0:2], sourcePort)
	binary.BigEndian.PutUint16(tcpHeader[2:4], targetPort)
	tcpHeader[12] = 5 << 4
	tcpHeader[13] = flags
	return append(encodedHeader, tcpHeader...)
}

func quotedTransportPacket(t *testing.T, protocol int, target net.IP,
	sourcePort, targetPort, udpLength uint16, sequence uint32,
) []byte {
	t.Helper()
	header := &ipv4.Header{
		Version:  ipv4.Version,
		Len:      ipv4.HeaderLen,
		TotalLen: ipv4.HeaderLen + 8,
		Protocol: protocol,
		Src:      net.ParseIP("192.0.2.20").To4(),
		Dst:      target,
	}
	encoded, err := header.Marshal()
	require.NoError(t, err)
	transport := make([]byte, 8)
	binary.BigEndian.PutUint16(transport[0:2], sourcePort)
	binary.BigEndian.PutUint16(transport[2:4], targetPort)
	if protocol == 17 {
		binary.BigEndian.PutUint16(transport[4:6], udpLength)
	} else {
		binary.BigEndian.PutUint32(transport[4:8], sequence)
	}
	return append(encoded, transport...)
}

func marshalICMP(t *testing.T, messageType icmp.Type, body icmp.MessageBody) []byte {
	t.Helper()
	packet, err := (&icmp.Message{Type: messageType, Body: body}).Marshal(nil)
	require.NoError(t, err)
	return packet
}
