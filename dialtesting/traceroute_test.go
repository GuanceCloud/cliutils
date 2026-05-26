// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package dialtesting

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTraceroute(t *testing.T) {
	routes, err := TracerouteIP("180.101.49.13", &TracerouteOption{
		Hops:  30,
		Retry: 3,
	})

	fmt.Println(routes, err)

	for index, route := range routes {
		fmt.Printf("%d ", index)
		for _, item := range route.Items {
			fmt.Printf("%s %f ", item.IP, item.ResponseTime)
		}
		fmt.Printf(" total: %d, failed: %d, loss: %f, avg: %f, max: %f, min: %f, std: %f\n", route.Total, route.Failed, route.Loss, route.AvgCost, route.MaxCost, route.MinCost, route.StdCost)
	}
}

func TestPreferredIP(t *testing.T) {
	tests := []struct {
		name string
		ips  []net.IP
		want string
	}{
		{
			name: "prefers IPv4 from dual-stack DNS result",
			ips: []net.IP{
				net.ParseIP("2001:db8::1"),
				net.ParseIP("192.0.2.10"),
			},
			want: "192.0.2.10",
		},
		{
			name: "returns IPv4 when IPv4 is first",
			ips: []net.IP{
				net.ParseIP("192.0.2.20"),
				net.ParseIP("2001:db8::2"),
			},
			want: "192.0.2.20",
		},
		{
			name: "falls back to IPv6 when no IPv4 exists",
			ips: []net.IP{
				net.ParseIP("2001:db8::3"),
			},
			want: "2001:db8::3",
		},
		{
			name: "returns nil for empty result",
			ips:  nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := preferredIP(tt.ips)
			if tt.want == "" {
				assert.Nil(t, got)
				return
			}

			assert.Equal(t, tt.want, got.String())
		})
	}
}

func TestTracerouteResolveHostIPPrefersIPv4ForDualStackDNS(t *testing.T) {
	oldLookupIP := lookupIP
	lookupIP = func(host string) ([]net.IP, error) {
		assert.Equal(t, "dual-stack.example", host)
		return []net.IP{
			net.ParseIP("2001:db8::1"),
			net.ParseIP("192.0.2.10"),
		}, nil
	}
	defer func() {
		lookupIP = oldLookupIP
	}()

	traceroute := &Traceroute{Host: "dual-stack.example"}
	ip, err := traceroute.resolveHostIP()
	assert.NoError(t, err)
	assert.Equal(t, "192.0.2.10", ip.String())
}
