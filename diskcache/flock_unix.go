// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

//go:build !windows
// +build !windows

package diskcache

import (
	"fmt"
	"os"
	"syscall"
)

func (l *walLock) tryLock() (bool, error) {
	f, err := os.OpenFile(l.file, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return false, err
	}
	l.f = f

	// LOCK_EX = Exclusive, LOCK_NB = Non-blocking
	err = syscall.Flock(int(l.f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		// If the error is EWOULDBLOCK, it means someone else has the lock
		if err == syscall.EWOULDBLOCK {
			l.f.Close()
			return false, fmt.Errorf("locked")
		}
		l.f.Close()
		return false, err
	}
	return true, nil
}

func (l *walLock) unlock() {
	if l.f != nil {
		syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
		l.f.Close()
		os.Remove(l.file) // Optional on Unix
	}
}
