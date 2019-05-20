package cliutils

import (
	"math/rand"
	"time"

	uuid "github.com/satori/go.uuid"
)

type Sem struct {
	sem chan interface{}
}

func NewSem() *Sem {
	return &Sem{sem: make(chan interface{})}
}

func (s *Sem) Close() {
	select {
	case <-s.sem:
	// pass: s.sem has been closed before
	default:
		close(s.sem)
	}
}

func (s *Sem) Wait() <-chan interface{} {
	return s.sem
}

var (
	letterBytes   = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	letterIdxBits = uint(6)              // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits
)

func CreateRandomString(n int) string {

	var src = rand.NewSource(time.Now().UnixNano())

	b := make([]byte, n)
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & int64(letterIdxMask)); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

func UUID(p string) string {
	for {
		id, err := uuid.NewV4()
		if err == nil {
			return p + id.String()
		}
	}
}
