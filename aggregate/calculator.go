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

func NewCaculatorCache() *CaculatorCache {
	return &CaculatorCache{
		cache: map[uint64]Calculator{},
	}
}

func (cc *CaculatorCache) AddBatches(batches ...*AggregationBatch) {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	for _, b := range batches {
		for _, c := range newCalculators(b) {
			calcHash := c.Base().hash

			if calc, ok := cc.cache[calcHash]; ok {
				calc.Add(c)
				l.Debugf("append to instance %s, heap size %d", c.Base(), len(cc.heap))
			} else {
				c.Base().build()

				cc.cache[calcHash] = c
				heap.Push(cc, c)

				l.Debugf("create new instance %s, heap size %d", c.Base(), len(cc.heap))
			}
		}
	}
}

func (cc *CaculatorCache) DelBatch(id uint64) bool {
	cc.mtx.Lock()
	defer cc.mtx.Unlock()

	if c, ok := cc.cache[id]; !ok {
		return false // not exist
	} else {
		heap.Remove(cc, c.Base().heapIdx)
		delete(cc.cache, id)
		return true
	}
}

func (cc *CaculatorCache) PeekNext() (Calculator, bool) {
	cc.mtx.RLock()
	defer cc.mtx.RUnlock()

	if len(cc.heap) == 0 {
		return nil, false
	}

	return cc.heap[0], true
}

/*
	func (cc *CaculatorCache) ScheduleJob(c Calculator) {
		cc.mtx.Lock()
		defer cc.mtx.Unlock()

		mb := c.Base()
		mb.nextWallTime = AlignNextWallTime(time.Now(), time.Duration(mb.window))
		mb.heapIdx = len(cc.heap)
		heap.Push(cc, c)
		cc.cache[mb.hash] = c
	}
*/
func (cc *CaculatorCache) Len() int {
	return len(cc.heap)
}

func (cc *CaculatorCache) Less(i, j int) bool {
	// smallest nextWallTime pop first.
	less := cc.heap[i].Base().nextWallTime < cc.heap[j].Base().nextWallTime

	l.Debugf("compare [%d]%s <-> [%d]%s => %v",
		i,
		time.Duration(cc.heap[i].Base().nextWallTime),
		j,
		time.Duration(cc.heap[j].Base().nextWallTime), less)

	return less
}

func (cc *CaculatorCache) Swap(i, j int) {
	if len(cc.heap) == 0 {
		return
	}

	l.Debugf("swap %s <-> %s, len: %d", cc.heap[i].Base(), cc.heap[j].Base(), len(cc.heap))

	cc.heap[i], cc.heap[j] = cc.heap[j], cc.heap[i]
	cc.heap[i].Base().heapIdx = i
	cc.heap[j].Base().heapIdx = j
}

func (cc *CaculatorCache) Push(x any) {
	c := x.(Calculator)
	c.Base().heapIdx = len(cc.heap)
	cc.heap = append(cc.heap, c)
}

func (cc *CaculatorCache) Pop() any {
	old := cc.heap
	n := len(old)
	if n == 0 {
		return nil
	}

	// pop out the last one
	c := old[n-1]
	c.Base().heapIdx = -1 // label removed
	cc.heap = old[0 : n-1]
	return c
}

type Calculator interface {
	Add(any)
	Aggr() ([]*point.Point, error)
	Reset()
	Base() *MetricBase
}

func AlignNextWallTime(t time.Time, align time.Duration) int64 {
	truncated := t.Truncate(align)

	if truncated.Equal(t) {
		return t.Unix()
	}

	return truncated.Add(align).Unix()
}

func newCalculators(batch *AggregationBatch) (res []Calculator) {
	var ptwrap *point.Point
	// now    = time.Now()

	for key, algo := range batch.AggregationOpts {
		var extraTags [][2]string
		// nextWallTime = AlignNextWallTime(now, time.Duration(algo.Window))

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

			mb := MetricBase{
				pt:       pt,
				key:      keyName,
				name:     ptwrap.Name(),
				aggrTags: extraTags,
				// align to next wall-time
				// XXX: what if the point reach too late?
				nextWallTime: AlignNextWallTime(ptwrap.Time(), time.Second*time.Duration(algo.Window)),
				window:       algo.Window,
			}

			// we get the kv for current algorithm.
			switch algo.Method {
			case MAX:
				if f64, ok := val.(float64); ok {
					calc := &algoMax{
						max:        f64,
						maxTime:    ptwrap.Time().UnixNano(),
						MetricBase: mb,
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
						MetricBase: mb,
					}

					calc.doHash(batch.RoutingKey)

					res = append(res, calc)
				} else {
					l.Warnf("non-numeric type(%s) for algorithm SUM, ignored", reflect.TypeOf(val))
				}

			case AVG:
				if f64, ok := val.(float64); ok {
					calc := &algoAvg{
						delta:      f64,
						maxTime:    ptwrap.Time().UnixNano(),
						MetricBase: mb,
					}

					calc.doHash(batch.RoutingKey)

					res = append(res, calc)
				} else {
					l.Warnf("non-numeric type(%s) for algorithm AVG, ignored", reflect.TypeOf(val))
				}
			case COUNT:
				calc := &algoCount{
					maxTime:    ptwrap.Time().UnixNano(),
					MetricBase: mb,
				}

				calc.doHash(batch.RoutingKey)

				res = append(res, calc)
			case MIN:
				if f64, ok := val.(float64); ok {
					calc := &algoMin{
						min:        f64,
						maxTime:    ptwrap.Time().UnixNano(),
						MetricBase: mb,
					}

					calc.doHash(batch.RoutingKey)

					res = append(res, calc)
				} else {
					l.Warnf("non-numeric type(%s) for algorithm MIN, ignored", reflect.TypeOf(val))
				}
			case HISTOGRAM:
				if f64, ok := val.(float64); ok {
					calc := &algoHistogram{
						val:        f64,
						maxTime:    ptwrap.Time().UnixNano(),
						MetricBase: mb,
					}

					calc.doHash(batch.RoutingKey)

					res = append(res, calc)
				} else {
					l.Warnf("non-numeric type(%s) for algorithm Histogram, ignored", reflect.TypeOf(val))
				}
			case EXPO_HISTOGRAM,
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

func prettyBatch(ab *AggregationBatch) string {
	return fmt.Sprintf("routingKey: %d\nconfigHash: %d\npoints: %d",
		ab.RoutingKey, ab.ConfigHash, len(ab.Points.Arr))
}
