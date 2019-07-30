package network

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"
)

func PortInUse(ipport string, timeout time.Duration) bool {
	c, err := net.DialTimeout(`tcp`, ipport, timeout)
	if err != nil {
		log.Printf("[error] %s", err.Error())
		return false
	}

	log.Printf("[debug] port %s used under TCP", ipport)

	defer c.Close()
	return true
}

func ParseListen(listen string) (string, int, error) {
	parts := strings.Split(listen, `:`)

	if len(parts) == 1 { // 只有 port 部分
		port, err := strconv.ParseUint(parts[0], 10, 16)
		if err != nil {
			return "", -1, fmt.Errorf("invalid listen addr: %s", listen)
		}
		return "", int(port), nil
	}

	if len(parts) != 2 {
		return "", -1, fmt.Errorf("invalid listen addr: %s", listen)
	}

	port, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return "", -1, fmt.Errorf("invalid listen addr: %s", listen)
	}

	return parts[0], int(port), nil
}
