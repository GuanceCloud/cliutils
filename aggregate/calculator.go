package aggregate

import (
	"container/heap"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"go.uber.org/zap/zapcore"
)

// CaculatorCache cache all Calculators.
type CaculatorCache struct {
	cache map[uint64]Calculator
	heap  []Calculator
	mtx   sync.RWMutex
}

type Calculator interface {
	add(any)
	aggr() ([]*point.Point, error)
	reset()
	base() *metricBase
}

type metricBase struct {
	pt *point.PBPoint

	aggrTags [][2]string // hash tags
	key,
	name string

	hash uint64

	window,
	nextWallTime int64
	heapIdx int
}

func NewCaculatorCache() *CaculatorCache {
	return &CaculatorCache{
		cache: map[uint64]Calculator{},
	}
}

func (cc *CaculatorCache) addBatch(batch *AggregationBatch) {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	for _, c := range newCalculators(batch) {
		calcHash := c.base().hash

		if calc, ok := cc.cache[calcHash]; ok {
			l.Debug("append new instance")
			calc.add(c)
		} else {
			l.Debug("create new instance")
			c.base().build()
			cc.cache[calcHash] = c
			heap.Push(cc, c)
		}
	}
}

func (cc *CaculatorCache) delBatch(id uint64) bool {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	if c, ok := cc.cache[id]; !ok {
		return false // not exist
	} else {
		heap.Remove(cc, c.base().heapIdx)
		delete(cc.cache, id)
		return true
	}
}

func (cc *CaculatorCache) peekNext() (Calculator, bool) {
	cc.mtx.RLock()
	defer cc.mtx.RUnlock()

	if len(cc.heap) == 0 {
		return nil, false
	}

	return cc.heap[0], true
}

func (cc *CaculatorCache) scheduleJob(c Calculator) {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()
	// TODO
}

func (cc *CaculatorCache) Len() int {
	return len(cc.heap)
}

func (cc *CaculatorCache) Less(i, j int) bool {
	return cc.heap[i].base().nextWallTime < cc.heap[j].base().nextWallTime
}

func (cc *CaculatorCache) Swap(i, j int) {
	cc.heap[i], cc.heap[j] = cc.heap[j], cc.heap[i]
	cc.heap[i].base().heapIdx = i
	cc.heap[j].base().heapIdx = j
}

func (cc *CaculatorCache) Push(x any) {
	c := x.(Calculator)
	c.base().heapIdx = len(cc.heap)
	cc.heap = append(cc.heap, c)
}

func (cc *CaculatorCache) Pop() any {
	old := cc.heap
	n := len(old)
	c := old[n-1]
	c.base().heapIdx = -1 // label removed
	cc.heap = old[0 : n-1]
	return c
}

func alignNextWallTime(t time.Time, align time.Duration) time.Time {
	truncated := t.Truncate(align)

	if truncated.Equal(t) {
		return t
	}

	return truncated.Add(align)
}

func newCalculators(batch *AggregationBatch) (res []Calculator) {
	var (
		ptwrap *point.Point
		now    = time.Now()
	)

	for key, algo := range batch.AggregationOpts {
		var (
			extraTags    [][2]string
			nextWallTime = alignNextWallTime(now, time.Duration(algo.Window))
		)

		if len(algo.AddTags) > 0 {
			// NOTE: we first add these extra-tags from algorithm configure. If origin
			// data got same-name tag key, we override the algorithm configured tags by
			// using kv.SetTag().
			for k, v := range algo.AddTags {
				extraTags = append(extraTags, [2]string{k, v})
			}
		}

		for _, pt := range batch.Points.Arr {
			var (
				keyName string
				val     any
				ptwrap  = point.WrapPB(ptwrap, pt)
			)

			if algo.SourceField != "" {
				if val = ptwrap.Get(algo.SourceField); val == nil {
					continue
				}

				keyName = key
			} else {
				if val = ptwrap.Get(key); val == nil {
					continue
				}

				keyName = key
			}

			if keyName == "" {
				if l.Level() == zapcore.DebugLevel {
					l.Debugf("ignore point %s", ptwrap.Pretty())
				}
				continue
			}

			mb := metricBase{
				pt:       pt,
				key:      keyName,
				name:     ptwrap.Name(),
				aggrTags: extraTags,
				// align to next wall-time
				nextWallTime: nextWallTime.UnixNano(),
				window:       algo.Window,
			}

			// we get the kv for current algorithm.
			switch algo.Method {
			case MAX:
				if f64, ok := val.(float64); ok {
					calc := &algoMax{
						max:        f64,
						maxTime:    ptwrap.Time().UnixNano(),
						metricBase: mb,
					}

					calc.doHash(batch.RoutingKey)

					res = append(res, calc)
				} else {
					l.Warnf("non-numeric type(%s) for algorithm MAX, ignored", reflect.TypeOf(val))
				}

			case SUM:
				if f64, ok := val.(float64); ok {
					calc := &algoSum{
						delta:      f64,
						maxTime:    ptwrap.Time().UnixNano(),
						metricBase: mb,
					}

					calc.doHash(batch.RoutingKey)

					res = append(res, calc)
				} else {
					l.Warnf("non-numeric type(%s) for algorithm SUM, ignored", reflect.TypeOf(val))
				}

			case AVG,
				COUNT,
				MIN,
				HISTOGRAM,
				EXPO_HISTOGRAM,
				STDEV,
				COUNT_DISTINCT,
				QUANTILES,
				LAST,
				FIRST: // TODO
			default: // pass
			}
		}
	}

	return res
}

// build used to delay build the tags.
func (mb *metricBase) build() {
	l.Debugf("before build aggr tags %+#v", mb.aggrTags)

	for _, kv := range mb.pt.Fields {
		if kv.IsTag {
			mb.aggrTags = append(mb.aggrTags, [2]string{kv.Key, kv.GetS()})
		}
	}

	l.Debugf("aggr tags %+#v", mb.aggrTags)
}

func prettyBatch(ab *AggregationBatch) string {
	return fmt.Sprintf("routingKey: %d\nconfigHash: %d\npoints: %d",
		ab.RoutingKey, ab.ConfigHash, len(ab.Points.Arr))
}

type (
	algoAvg struct {
		metricBase
		sum float64
		maxTime,
		count int64
	}
	algoCount struct {
		metricBase
		maxTime,
		count int64
	}
	algoMin struct {
		metricBase
		maxTime,
		count int64
		min float64
	}

	algoHistogram struct {
		metricBase
		min, max, sum float64
		count         int64
		bounds        []float64
		buckets       []uint64
	}

	explicitBounds struct {
		metricBase
		index  int64
		cnt    uint64
		lb, ub float64
		pos    bool
	}

	algoExpoHistogram struct {
		metricBase
		min, max, sum    float64
		zeroCount, count int64
		scale            int
		maxTime, minTime int64
		negBucketCounts,
		posBucketCounts []uint64
		bounds []*explicitBounds
	}

	algoStdev struct {
		metricBase
		// TODO
	}

	algoQuantiles struct {
		metricBase
		// TODO
	}

	algoCountDistinct struct {
		metricBase
		// TODO
	}
	algoCountLast struct {
		metricBase
		// TODO
	}
	algoCountFirst struct {
		metricBase
		// TODO
	}
)
