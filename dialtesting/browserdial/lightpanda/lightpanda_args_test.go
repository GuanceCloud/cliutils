// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package lightpanda

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/GuanceCloud/cliutils/dialtesting/browserdial/runner"
)

func TestLightpandaArguments(t *testing.T) {
	root := t.TempDir()
	caFile := filepath.Join(root, "internal-ca.pem")
	caDir := filepath.Join(root, "roots")
	arguments, err := lightpandaArgumentsWithSystemCADirectories(runner.EngineOptions{
		CACertFile: caFile,
		CACertDir:  caDir,
		ProxyURL:   "http://user:password@proxy.example.com:8080",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--ca-cert", caFile,
		"--ca-path", caDir,
		"--http-proxy", "http://user:password@proxy.example.com:8080",
	}
	if !reflect.DeepEqual(arguments, want) {
		t.Fatalf("arguments = %#v, want %#v", arguments, want)
	}

	arguments, err = lightpandaArguments(runner.EngineOptions{BlockPrivateNetwork: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(arguments, []string{"--block-private-networks"}) {
		t.Fatalf("private network block argument is missing: %#v", arguments)
	}
	arguments, err = lightpandaArguments(runner.EngineOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(arguments) != 0 {
		t.Fatalf("zero-value options should preserve existing behavior: %#v", arguments)
	}
}

func TestLightpandaArgumentsPreserveSystemCAAndCustomCIDRs(t *testing.T) {
	systemDirectory := t.TempDir()
	customDirectory := t.TempDir()
	arguments, err := lightpandaArgumentsWithSystemCADirectories(runner.EngineOptions{
		CACertDir:           customDirectory,
		PrivateNetworkCIDRs: []string{"10.0.0.0/8", "192.168.0.0/16"},
	}, []string{systemDirectory})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--ca-path", systemDirectory,
		"--ca-path", customDirectory,
		"--block-cidrs", "10.0.0.0/8,192.168.0.0/16",
	}
	if !reflect.DeepEqual(arguments, want) {
		t.Fatalf("arguments = %#v, want %#v", arguments, want)
	}
}

func TestLightpandaArgumentsRejectInvalidValues(t *testing.T) {
	tests := []runner.EngineOptions{
		{CACertFile: "internal-ca.pem"},
		{CACertDir: "certs"},
		{ProxyURL: "https://proxy.example.com:8443"},
		{ProxyURL: "socks5://proxy.example.com:1080"},
		{ProxyURL: "http:///missing-host"},
		{PrivateNetworkCIDRs: []string{"not-a-cidr"}},
	}
	for _, options := range tests {
		if _, err := lightpandaArgumentsWithSystemCADirectories(options, nil); err == nil {
			t.Fatalf("expected invalid options to fail: %#v", options)
		}
	}
}
