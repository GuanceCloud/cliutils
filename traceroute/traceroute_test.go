// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package traceroute

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceRejectsNonUnicastTargets(t *testing.T) {
	for _, target := range []string{"0.0.0.0", "224.0.0.1", "255.255.255.255"} {
		_, err := Trace(context.Background(), net.ParseIP(target), Options{})
		require.ErrorContains(t, err, "unicast")
	}
}

func TestNormalizeOptions(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		cfg, err := normalizeOptions(Options{})
		require.NoError(t, err)
		assert.Equal(t, ProtocolICMP, cfg.protocol)
		assert.Equal(t, 30, cfg.maxTTL)
		assert.Equal(t, 1, cfg.attempts)
		assert.Equal(t, defaultTimeout, cfg.timeout)
	})

	t.Run("protocol limits", func(t *testing.T) {
		cfg, err := normalizeOptions(Options{
			Protocol: ProtocolUDP,
			Port:     53,
			MaxTTL:   1000,
			Attempts: 4,
			Timeout:  time.Minute,
		})
		require.NoError(t, err)
		assert.Equal(t, MaxUDPHops, cfg.maxTTL)
		assert.Equal(t, 4, cfg.attempts)
		assert.Equal(t, MaxTimeout, cfg.timeout)
	})

	t.Run("UDP fragmentation guard", func(t *testing.T) {
		_, err := normalizeOptions(Options{
			Protocol: ProtocolUDP,
			Port:     53,
			MaxTTL:   MaxUDPHops,
			Attempts: MaxAttempts,
		})
		require.ErrorContains(t, err, "max TTL times attempts")
	})

	for _, protocol := range []Protocol{ProtocolTCP, ProtocolUDP} {
		_, err := normalizeOptions(Options{Protocol: protocol})
		require.ErrorContains(t, err, "requires a destination port")
	}
}

func TestRepliesToRoute(t *testing.T) {
	route := repliesToRoute([]probeReply{
		{ip: net.ParseIP("192.0.2.1"), rtt: time.Millisecond},
		{timedOut: true},
		{ip: net.ParseIP("192.0.2.1"), rtt: 3 * time.Millisecond},
	})

	assert.Equal(t, 3, route.Total)
	assert.Equal(t, 1, route.Failed)
	assert.InDelta(t, 100.0/3.0, route.Loss, 0.001)
	assert.Equal(t, 1000.0, route.MinCost)
	assert.Equal(t, 2000.0, route.AvgCost)
	assert.Equal(t, 3000.0, route.MaxCost)
	assert.InDelta(t, 1414.213, route.StdCost, 0.001)
	assert.Equal(t, "*", route.Items[1].IP)
}
