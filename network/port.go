package network

import (
	"fmt"
	"net"
	"strconv"
	"strings"
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

func ParseListen(listen string) (string, int, error) {
	parts := strings.Split(listen, `:`)

	if len(parts) == 1 { // 只有 port 部分
		port, err := strconv.ParseInt(parts[0], 10, 16)
		if err != nil {
			return "", -1, fmt.Errorf("invalid listen addr: %s", listen)
		}
		return "", int(port), nil
	}

	if len(parts) != 2 {
		return "", -1, fmt.Errorf("invalid listen addr: %s", listen)
	}

	port, err := strconv.ParseInt(parts[1], 10, 16)
	if err != nil {
		return "", -1, fmt.Errorf("invalid listen addr: %s", listen)
	}

	return parts[0], int(port), nil
}
