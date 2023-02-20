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
	}

	return point.NewPointV2([]byte("diskcache"),
		append(point.NewTags(tags), point.NewKVs(fields)...))
}
