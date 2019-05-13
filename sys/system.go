package sys

import (
	"syscall"
)

func SetSysLimit(n uint64) error {
	var lmt syscall.Rlimit

	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lmt); err != nil {
		return err
	}

	lmt.Max = n
	lmt.Cur = n

	return syscall.Setrlimit(syscall.RLIMIT_NOFILE, &lmt)
}
