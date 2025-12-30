package aggregate

import (
	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/cespare/xxhash/v2"
	"go.uber.org/zap/zapcore"
)

// CaculatorCache cache all Calculators.
type CaculatorCache struct {
	cache map[uint64]Calculator
}

func NewCaculatorCache() *CaculatorCache {
	return &CaculatorCache{
		cache: map[uint64]Calculator{},
	}
}

func (cc *CaculatorCache) addBatch(batch *AggregationBatch) {
	for _, c := range newCalculators(batch) {
		calcHash := c.hash()

		if calc, ok := cc.cache[calcHash]; ok {
			l.Debug("append new instance")
			calc.add(c)
		} else {
			l.Debug("create new instance")
			c.build()
			cc.cache[calcHash] = c
		}
	}
}

func newCalculators(batch *AggregationBatch) (res []Calculator) {
	var ptwrap *point.Point

	for key, algo := range batch.AggregationOpts {
		var extraTags [][2]string

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
				val     float64
				ok      bool
				ptwrap  = point.WrapPB(ptwrap, pt)
			)

			if algo.SourceField != "" {
				if val, ok = ptwrap.GetF(algo.SourceField); !ok {
					continue
				}

				keyName = key
			} else {
				// NOTE: all metric numeric values has been converted to float64
				if val, ok = ptwrap.GetF(key); !ok {
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

			// we get the kv for current algorithm.
			switch algo.Method {
			case MAX:
				calc := &algoMax{
					max:     val,
					maxTime: ptwrap.Time().UnixNano(),
					metricBase: metricBase{
						pt:   pt,
						key:  keyName,
						name: ptwrap.Name(),
					},
				}

				calc.doHash(batch.RoutingKey)
				calc.aggrTags = extraTags

				res = append(res, calc)

			case SUM:
				calc := &algoSum{
					delta:   val,
					maxTime: ptwrap.Time().UnixNano(),
					metricBase: metricBase{
						pt:   pt,
						key:  keyName,
						name: ptwrap.Name(),
					},
				}

				if len(algo.AddTags) > 0 {
					// NOTE: we first add these extra-tags from algorithm configure. If origin
					// data got same-name tag key, we override the algorithm configured tags by
					// using kv.SetTag().
					for k, v := range algo.AddTags {
						calc.aggrTags = append(calc.aggrTags, [2]string{k, v})
					}
				}

				calc.doHash(batch.RoutingKey)
				calc.aggrTags = extraTags

				res = append(res, calc)

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

type Calculator interface {
	add(any)
	aggr() ([]*point.Point, error)
	reset()
	hash() uint64
	build()
}

type metricBase struct {
	pt *point.PBPoint

	aggrTags [][2]string // hash tags
	key,
	name string
	hash uint64
}

// type assertions
var _ Calculator = &algoSum{}

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

func (c *algoSum) build() {
	c.metricBase.build()
}

func (c *algoSum) add(x any) {
	if inst, ok := x.(*algoSum); ok {
		c.count++
		c.delta += inst.delta

		if inst.maxTime > c.maxTime {
			c.maxTime = inst.maxTime
		}
	}
}

func (c *algoSum) aggr() ([]*point.Point, error) {
	var kvs point.KVs

	kvs = kvs.Add(c.key, c.delta).
		Add(c.key+"_count", c.count)

	for _, kv := range c.aggrTags {
		// NOTE: if same-name tag key exist, apply the last one.
		kvs = kvs.SetTag(kv[0], kv[1])
	}

	return []*point.Point{
		point.NewPoint(c.name, kvs, point.WithTimestamp(c.maxTime)),
	}, nil
}

func (c *algoSum) reset() {
	c.delta = 0
	c.maxTime = 0
	c.count = 0
}

func (c *algoSum) hash() uint64 {
	return c.metricBase.hash
}

func (c *algoSum) doHash(h1 uint64) {
	h := HashCombine(h1, xxhash.Sum64([]byte("sum")))
	h = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.key)))
	c.metricBase.hash = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.name)))
}

func (c *algoMax) add(x any) {
	if inst, ok := x.(*algoMax); ok {
		if c.max < inst.max {
			c.max = inst.max
		}
		if c.maxTime < inst.maxTime {
			c.maxTime = inst.maxTime
		}
		c.count++
	}
}

func (c *algoMax) aggr() ([]*point.Point, error) {
	var kvs point.KVs

	kvs = kvs.Add(c.key, c.max).
		Add(c.key+"_count", c.count)
	for _, kv := range c.aggrTags {
		kvs = kvs.SetTag(kv[0], kv[1])
	}

	return []*point.Point{
		point.NewPoint(c.name, kvs, point.WithTimestamp(c.maxTime)),
	}, nil
}

func (c *algoMax) reset() {
	c.max = 0.0
	c.count = 0
	c.maxTime = 0
}

func (c *algoMax) hash() uint64 {
	return c.metricBase.hash
}

func (c *algoMax) build() {
	c.metricBase.build()
}

func (c *algoMax) doHash(h1 uint64) {
	h := HashCombine(h1, xxhash.Sum64([]byte("max")))
	h = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.key)))
	c.metricBase.hash = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.name)))
}

type (
	algoSum struct {
		metricBase
		delta          float64
		maxTime, count int64
	}

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
	algoMax struct {
		metricBase
		maxTime,
		count int64
		max float64
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
