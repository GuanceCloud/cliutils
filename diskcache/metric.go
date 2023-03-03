// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/point"
)

// Metrics get various tags/fields about the DiskCache.
func (c *DiskCache) Metrics() *point.Point {
	c.rlock.Lock()
	defer c.rlock.Unlock()

	tags := map[string]string{
		"path":   c.path,
		"nosync": fmt.Sprintf("%v", c.noSync),
	}

	gcnt, pcnt := c.getCount, c.putCount
	if gcnt == 0 {
		gcnt = 1
	}

	if pcnt == 0 {
		pcnt = 1
	}

	fields := map[string]any{
		"size":           c.size,
		"data_files":     len(c.dataFiles),
		"cur_batch_size": c.curBatchSize,
		"rotate_count":   c.rotateCount,
		"dropped_batch":  c.droppedBatch,
		"get":            c.getCount,
		"put":            c.putCount,
		"get_bytes":      c.getBytes,
		"put_bytes":      c.putBytes,
		"get_cost_avg":   c.getCost / int64(gcnt),
		"put_cost_avg":   c.putCost / int64(pcnt),
	}

	return point.NewPointV2([]byte("diskcache"),
		append(point.NewTags(tags), point.NewKVs(fields)...))
}
