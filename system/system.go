// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package system used to wrap basic system related settings.
package system

import (
	"syscall"
)

func SetSysLimit(x int, n uint64) error {
	var lmt syscall.Rlimit

	if err := syscall.Getrlimit(x, &lmt); err != nil {
		return err
	}

	lmt.Max = n
	lmt.Cur = n

	return syscall.Setrlimit(x, &lmt)
}
