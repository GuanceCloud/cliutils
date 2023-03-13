package main

import (
	"flag"
	"log"
	"math/rand"
	"sync"
	"time"

	dc "github.com/GuanceCloud/cliutils/diskcache"
	"github.com/GuanceCloud/cliutils/metrics"
)

var (
	path             string
	capacity         int64
	disableGoMetrics bool
	putLatency,
	getLatency,
	runtime,
	workers,
	sampleMax,
	sampleMin int

	cache *dc.DiskCache
	wg    sync.WaitGroup

	dataBuf []byte

	tick *time.Ticker
)

const (
	GB = 1024 * 1024 * 1024
)

func init() {
	flag.StringVar(&path, "path", "./disccache", "cache path")
	flag.Int64Var(&capacity, "cap", 32, "cache capacity(GB)")
	flag.IntVar(&workers, "workers", 1, "concurrent Put/Get workers")
	flag.IntVar(&sampleMax, "smax", 32768, "maximum sample size(KB)")
	flag.IntVar(&sampleMin, "smin", 4, "minimal sample size(KB)")
	flag.IntVar(&runtime, "runtime", 5, "run time(minute) for the test")
	flag.BoolVar(&disableGoMetrics, "disable-gom", false, "disable golang metrics")
	flag.IntVar(&putLatency, "put-lat", 100, "put latency(ms) randome range(from 0)")
	flag.IntVar(&getLatency, "get-lat", 100, "get latency(ms) randome range(from 0)")
}

func main() {

	flag.Parse()
	var err error

	dataBuf = make([]byte, 32*1024*1024) // 32MB data buffer
	tick = time.NewTicker(time.Duration(runtime) * time.Minute)

	cache, err = dc.Open(dc.WithPath(path), dc.WithCapacity(capacity*GB))
	if err != nil {
		log.Panic(err)
	}

	run()
}

// get random bytes from dataBuf
func getSamples() []byte {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	n := (sampleMin + (r.Int() % sampleMax)) * 1024 // in KB
	if n >= len(dataBuf) {
		n = len(dataBuf)
	}

	start := r.Int() % len(dataBuf)
	//log.Printf("get %s bytes from %s", humanize.SI(float64(n), ""), humanize.SI(float64(start), ""))

	if start+n > len(dataBuf) {
		return dataBuf[len(dataBuf)-n:] // return last n bytes
	} else {
		return dataBuf[start : start+n]
	}
}

func put(n int) {
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {

			defer wg.Done()
			for {
				select {
				case <-tick.C:
					return
				default:
					r := rand.New(rand.NewSource(time.Now().UnixNano()))
					time.Sleep(time.Millisecond * time.Duration(r.Int()%getLatency))
					cache.Put(getSamples())
				}
			}
		}()
	}
}

func get(n int) {
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()

			for {
				select {
				case <-tick.C:
					for {
						// read them all without weekend.
						if err := cache.Get(func(_ []byte) error {
							r := rand.New(rand.NewSource(time.Now().UnixNano()))
							time.Sleep(time.Millisecond * time.Duration(r.Int()%getLatency))
							return nil
						}); err != nil {
							return
						}
					}
				default:
					cache.Get(func(_ []byte) error {
						r := rand.New(rand.NewSource(time.Now().UnixNano()))
						time.Sleep(time.Millisecond * time.Duration(r.Int()%getLatency))
						return nil
					})
				}
			}
		}()
	}
}

func run() {
	put(workers)
	get(workers)

	ms := metrics.NewMetricServer()
	ms.DisableGoMetrics = disableGoMetrics

	go func() {
		if err := ms.Start(); err != nil {
			log.Panic(err)
		}
	}()

	wg.Wait()
}
