// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

//go:build linux

package traceroute

import (
	"net"
	"testing"
	"time"

	"golang.org/x/net/ipv4"
)

func FuzzNetworkReplyParsers(f *testing.F) {
	f.Add([]byte{})
	f.Add(make([]byte, ipv4.HeaderLen))
	f.Add(make([]byte, 128))
	target := net.ParseIP("203.0.113.10").To4()
	local := net.ParseIP("192.0.2.20").To4()
	peer := &net.IPAddr{IP: net.ParseIP("192.0.2.1")}

	f.Fuzz(func(_ *testing.T, packet []byte) {
		parseUDPICMPReply(packet, peer, target, 31000, 33434, 9, time.Millisecond)
		parseTCPICMPReply(packet, peer, target, 31000, 443, 0x12345678, time.Millisecond)
		matchesICMPProbe(packet, peer.IP, target, 17, 23)
		matchesTCPResponse(&ipv4.Header{Src: target, Dst: local}, packet,
			local, target, 31000, 443, 0x12345678)
	})
}
