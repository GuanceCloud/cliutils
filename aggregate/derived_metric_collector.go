package aggregate

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/cespare/xxhash/v2"
)

type derivedMetricKey struct {
	token       string
	measurement string
	metricName  string
	stage       DerivedMetricStage
	decision    DerivedMetricDecision
	tagsHash    uint64
}

type derivedMetricValue struct {
	token       string
	measurement string
	metricName  string
	stage       DerivedMetricStage
	decision    DerivedMetricDecision
	tags        map[string]string
	sum         float64
	maxTS       int64
}

type DerivedMetricCollector struct {
	mu      sync.Mutex
	window  time.Duration
	buckets map[int64]map[derivedMetricKey]*derivedMetricValue
}

const DefaultDerivedMetricFlushWindow = 30 * time.Second

func NewDerivedMetricCollector(window time.Duration) *DerivedMetricCollector {
	if window <= 0 {
		window = DefaultDerivedMetricFlushWindow
	}

	return &DerivedMetricCollector{
		window:  window,
		buckets: make(map[int64]map[derivedMetricKey]*derivedMetricValue),
	}
}

func (c *DerivedMetricCollector) Add(records []DerivedMetricRecord) {
	if len(records) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, record := range records {
		timestamp := record.Time
		if timestamp.IsZero() {
			timestamp = time.Now()
		}

		exp := AlignNextWallTime(timestamp, c.window)
		key := newDerivedMetricKey(record)
		bucket := c.buckets[exp]
		if bucket == nil {
			bucket = make(map[derivedMetricKey]*derivedMetricValue)
			c.buckets[exp] = bucket
		}

		if current, ok := bucket[key]; ok {
			current.sum += record.Value
			if timestamp.UnixNano() > current.maxTS {
				current.maxTS = timestamp.UnixNano()
			}
			continue
		}

		bucket[key] = &derivedMetricValue{
			token:       record.Token,
			measurement: record.measurement(),
			metricName:  record.MetricName,
			stage:       record.Stage,
			decision:    record.Decision,
			tags:        cloneTags(record.Tags),
			sum:         record.Value,
			maxTS:       timestamp.UnixNano(),
		}
	}
}

func (c *DerivedMetricCollector) Flush(now time.Time) []*DerivedMetricPoints {
	c.mu.Lock()
	defer c.mu.Unlock()

	nowUnix := now.Unix()
	grouped := make(map[string][]*point.Point)

	for exp, bucket := range c.buckets {
		if exp > nowUnix {
			continue
		}

		for _, val := range bucket {
			grouped[val.token] = append(grouped[val.token], val.toPoint())
		}

		delete(c.buckets, exp)
	}

	res := make([]*DerivedMetricPoints, 0, len(grouped))
	for token, pts := range grouped {
		res = append(res, &DerivedMetricPoints{
			Token: token,
			PTS:   pts,
		})
	}

	return res
}

func newDerivedMetricKey(record DerivedMetricRecord) derivedMetricKey {
	return derivedMetricKey{
		token:       record.Token,
		measurement: record.measurement(),
		metricName:  record.MetricName,
		stage:       record.Stage,
		decision:    record.Decision,
		tagsHash:    hashDerivedMetricTags(record.Tags),
	}
}

func (v *derivedMetricValue) toPoint() *point.Point {
	kvs := point.KVs{}
	kvs = kvs.Add(v.metricName, v.sum)

	if v.stage != "" {
		kvs = kvs.AddTag("stage", string(v.stage))
	}

	if v.decision != "" {
		kvs = kvs.AddTag("decision", string(v.decision))
	}

	keys := sortedTagKeys(v.tags)
	for _, key := range keys {
		kvs = kvs.AddTag(key, v.tags[key])
	}

	return point.NewPoint(v.measurement, kvs, point.WithTimestamp(v.maxTS))
}

func hashDerivedMetricTags(tags map[string]string) uint64 {
	if len(tags) == 0 {
		return 0
	}

	keys := sortedTagKeys(tags)
	hash := Seed1
	for _, key := range keys {
		hash = HashCombine(hash, xxhash.Sum64(cliutils.ToUnsafeBytes(key)))
		hash = HashCombine(hash, xxhash.Sum64(cliutils.ToUnsafeBytes(tags[key])))
	}

	return hash
}

func sortedTagKeys(tags map[string]string) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneTags(tags map[string]string) map[string]string {
	if len(tags) == 0 {
		return nil
	}

	res := make(map[string]string, len(tags))
	for key, value := range tags {
		res[key] = value
	}
	return res
}

func (c *DerivedMetricCollector) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return "derived-metric-collector buckets=" + strconv.Itoa(len(c.buckets))
}
