// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

//go:build linux

package traceroute

import (
	"context"
	"errors"
	"net"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTCPTraceIntegration(t *testing.T) {
	if os.Getenv("CLIUTILS_TRACEROUTE_INTEGRATION") == "" {
		t.Skip("set CLIUTILS_TRACEROUTE_INTEGRATION=1 to run raw socket integration tests")
	}
	target := net.ParseIP("127.0.0.1").To4()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: target})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := Trace(ctx, target, Options{
		Protocol: ProtocolTCP,
		Port:     uint16(port),
		MaxTTL:   1,
		Attempts: 1,
		Timeout:  time.Second,
	})
	require.NoError(t, err)
	require.Len(t, result.Routes, 1)
	require.Len(t, result.Routes[0].Items, 1)
	assert.True(t, result.Reached)
	assert.Equal(t, target.String(), result.Routes[0].Items[0].IP)
}

func TestTCPTraceRemoteHopIntegration(t *testing.T) {
	address := os.Getenv("CLIUTILS_TRACEROUTE_REMOTE")
	if address == "" {
		t.Skip("set CLIUTILS_TRACEROUTE_REMOTE=IPv4:port to test a live intermediate hop")
	}
	host, portText, err := net.SplitHostPort(address)
	require.NoError(t, err)
	target := net.ParseIP(host).To4()
	require.NotNil(t, target)
	port, err := strconv.ParseUint(portText, 10, 16)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := Trace(ctx, target, Options{
		Protocol: ProtocolTCP,
		Port:     uint16(port),
		MaxTTL:   2,
		Attempts: 1,
		Timeout:  2 * time.Second,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Routes)
	require.NotEmpty(t, result.Routes[0].Items)
	assert.NotEqual(t, "*", result.Routes[0].Items[0].IP)
}

func TestTCPTraceConcurrentIntegration(t *testing.T) {
	if os.Getenv("CLIUTILS_TRACEROUTE_INTEGRATION") == "" {
		t.Skip("set CLIUTILS_TRACEROUTE_INTEGRATION=1 to run raw socket integration tests")
	}

	target := net.ParseIP("127.0.0.1").To4()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: target})
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	port := uint16(listener.Addr().(*net.TCPAddr).Port)

	const workers = 64
	start := make(chan struct{})
	errorsCh := make(chan error, workers)
	var waitGroup sync.WaitGroup
	for range workers {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			result, err := Trace(ctx, target, Options{
				Protocol: ProtocolTCP,
				Port:     port,
				MaxTTL:   1,
				Attempts: 1,
				Timeout:  time.Second,
			})
			if err != nil {
				errorsCh <- err
				return
			}
			if !result.Reached {
				errorsCh <- errors.New("TCP traceroute did not reach local listener")
			}
		}()
	}
	close(start)
	waitGroup.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
}

func TestUDPTraceLateApplicationResponseIntegration(t *testing.T) {
	if os.Getenv("CLIUTILS_TRACEROUTE_INTEGRATION") == "" {
		t.Skip("set CLIUTILS_TRACEROUTE_INTEGRATION=1 to run raw socket integration tests")
	}

	target := net.ParseIP("127.0.0.1").To4()
	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: target})
	require.NoError(t, err)
	t.Cleanup(func() { _ = server.Close() })
	require.NoError(t, server.SetReadDeadline(time.Now().Add(2*time.Second)))
	serverPort := uint16(server.LocalAddr().(*net.UDPAddr).Port)

	peers := make(chan *net.UDPAddr, 2)
	serverErrors := make(chan error, 2)
	go func() {
		var buffer [MaxUDPProbePayload]byte
		for probe := 0; probe < 2; probe++ {
			_, peer, err := server.ReadFromUDP(buffer[:])
			if err != nil {
				serverErrors <- err
				return
			}
			peer = &net.UDPAddr{IP: append(net.IP(nil), peer.IP...), Port: peer.Port, Zone: peer.Zone}
			peers <- peer
			if probe == 0 {
				go func() {
					time.Sleep(220 * time.Millisecond)
					_, err := server.WriteToUDP([]byte("late"), peer)
					serverErrors <- err
				}()
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := Trace(ctx, target, Options{
		Protocol: ProtocolUDP,
		Port:     serverPort,
		MaxTTL:   1,
		Attempts: 2,
		Timeout:  150 * time.Millisecond,
	})
	require.NoError(t, err)
	require.Len(t, result.Routes, 1)
	assert.False(t, result.Reached)
	assert.Equal(t, 2, result.Routes[0].Failed)

	firstPeer := <-peers
	secondPeer := <-peers
	assert.NotEqual(t, firstPeer.Port, secondPeer.Port)
	require.NoError(t, <-serverErrors)
}
