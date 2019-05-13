package sys

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
