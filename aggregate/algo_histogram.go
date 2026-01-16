package aggregate

import (
	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/cespare/xxhash/v2"
)

type algoHistogram struct {
	MetricBase
	count int64
	//bounds  []float64
	//buckets []uint64
	val float64
	// tag:"le" 固定tag
	leBucket map[string]float64
	maxTime  int64
}

// type assertions
var _ Calculator = &algoHistogram{}

func (c *algoHistogram) Add(x any) {
	if inst, ok := x.(*algoHistogram); ok {
		c.count++

		if inst.pt != nil {
			kvs := point.KVs(inst.pt.Fields)
			if le := kvs.GetTag("le"); le != "" {
				if _, ok := c.leBucket[le]; ok {
					c.leBucket[le] += c.val
				} else {
					c.leBucket[le] = inst.val
				}
			}
		}
	}
}

func (c *algoHistogram) Aggr() ([]*point.Point, error) {
	var kvs point.KVs
	for le, f := range c.leBucket {
		kvs = kvs.Add(le, f)
	}
	kvs = kvs.Add(c.key+"_count", c.count)

	for _, kv := range c.aggrTags {
		// NOTE: if same-name tag key exist, apply the last one.
		kvs = kvs.SetTag(kv[0], kv[1])
	}

	return []*point.Point{
		point.NewPoint(c.name, kvs, point.WithTimestamp(c.maxTime)),
	}, nil
}

func (c *algoHistogram) Reset() {
	c.leBucket = map[string]float64{}
	c.maxTime = 0
	c.count = 0
}

func (c *algoHistogram) doHash(h1 uint64) {
	h := HashCombine(h1, xxhash.Sum64([]byte("histogram")))
	h = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.key)))
	c.MetricBase.hash = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.name)))
}

func (c *algoHistogram) Base() *MetricBase {
	return &c.MetricBase
}
