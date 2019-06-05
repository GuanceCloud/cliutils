package cliutils

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
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

func WgWait(wg *sync.WaitGroup, timeout int) {

	c := make(chan interface{})

	go func() {
		defer close(c)
		wg.Wait()
	}()

	if timeout > 0 {
		select {
		case <-c:
		case <-time.After(time.Second * time.Duration(timeout)):
		}
	} else {
		select {
		case <-c:
		}
	}
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
		if err != nil {
			continue
		}

		return p + id.String()
	}
}

func SizeFmt(n int64) string {
	f := float64(n)

	unit := []string{"", "K", "M", "G", "T", "P", "E", "Z"}
	for _, u := range unit {
		if math.Abs(f) < 1024.0 {
			return fmt.Sprintf("%3.4f%sB", f, u)
		}
		f /= 1024.0
	}
	return fmt.Sprintf("%3.4fYB", f)
}
