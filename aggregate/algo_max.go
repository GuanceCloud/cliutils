package aggregate

import (
	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/cespare/xxhash/v2"
)

type algoMax struct {
	metricBase
	maxTime,
	count int64
	max float64
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

func (c *algoMax) base() *metricBase {
	return &c.metricBase
}

func (c *algoMax) doHash(h1 uint64) {
	h := HashCombine(h1, xxhash.Sum64([]byte("max")))
	h = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.key)))
	c.hash = HashCombine(h, xxhash.Sum64(cliutils.ToUnsafeBytes(c.name)))
}
