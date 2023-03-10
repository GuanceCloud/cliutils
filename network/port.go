// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package network wraps basic network related implements.
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

	defer func() {
		if err := c.Close(); err != nil {
			_ = err // pass
		}
	}()
	return true
}

func ParseListen(listen string) (ip string, port int64, err error) {
	parts := strings.Split(listen, `:`)

	if len(parts) == 1 { //nolint:gomnd // 只有 port 部分
		port, err = strconv.ParseInt(parts[0], 10, 16)
		if err != nil {
			err = fmt.Errorf("invalid listen addr: %s", listen)
		}

		return
	}

	if len(parts) != 2 { //nolint:gomnd
		err = fmt.Errorf("invalid listen addr: %s", listen)
		return
	}

	port, err = strconv.ParseInt(parts[1], 10, 16)
	if err != nil {
		err = fmt.Errorf("invalid listen addr: %s", listen)
		return
	}

	ip = parts[0]

	return
}
