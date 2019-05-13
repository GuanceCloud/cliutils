package network

import (
	"net"
	"time"
)

func PortInUse(ipport string, timeout time.Duration) bool {
	c, err := net.DialTimeout(`tcp`, ipport, timeout)
	if err != nil {
		return false
	}

	defer c.Close()
	return false
}
