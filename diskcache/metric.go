// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"github.com/GuanceCloud/cliutils/point"
)

// Metrics get various tags/fields about the DiskCache.
func (c *DiskCache) Metrics() *point.Point {
	c.rlock.Lock()
	defer c.rlock.Unlock()

	tags := map[string]string{
		"path": c.path,
	}

	gcnt, pcnt := c.getCount, c.putCount
	if gcnt == 0 {
		gcnt = 1
	}

	if pcnt == 0 {
		pcnt = 1
	}

	fields := map[string]any{
		"batch_size":     c.batchSize,
		"cur_batch_size": c.curBatchSize,
		"data_files":     len(c.dataFiles),
		"dropped_batch":  c.droppedBatch,
		"get":            c.getCount,
		"get_bytes":      c.getBytes,
		"get_cost_avg":   c.getCost / int64(gcnt),
		"nolock":         c.noLock,
		"nopos":          c.noPos,
		"nosync":         c.noSync,
		"put":            c.putCount,
		"put_bytes":      c.putBytes,
		"put_cost_avg":   c.putCost / int64(pcnt),
		"rotate_count":   c.rotateCount,
		"size":           c.size,
	}

	return point.NewPointV2([]byte("diskcache"),
		append(point.NewTags(tags), point.NewKVs(fields)...))
}
