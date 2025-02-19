package ipipnet

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIpIpNet(t *testing.T) {
	ipNet := &IpIpNet{}
	ipNet.Init("./testdata", nil)
	record, err := ipNet.Geo("120.253.192.179")
	if err != nil {
		t.Fatal(err)
	}
	t.Log("country: ", record.Country)
	t.Log("region: ", record.Region)
	t.Log("city: ", record.City)

	ips, err := net.LookupIP("www.ntu.edu.tw")
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, len(ips) > 0)

	t.Log("ip:", ips[0].String())
	record, err = ipNet.Geo(ips[0].String())
	if err != nil {
		t.Fatal(err)
	}
	t.Log("country: ", record.Country)
	t.Log("region: ", record.Region)
	t.Log("city: ", record.City)

	assert.True(t, strings.Contains(record.Country, "中国"))
	assert.True(t, strings.Contains(record.Region, "台湾"))
	assert.True(t, strings.Contains(record.City, "台北"))
}
