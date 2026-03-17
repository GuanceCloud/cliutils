// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package aggregate provides data aggregation and tail sampling functionality.
// This file contains derived metric generator for tail sampling.
package aggregate

import (
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	fp "github.com/GuanceCloud/cliutils/filter"
	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/point"
)

const (
	DerivedMetricFieldTraceID       = "$trace_id"
	DerivedMetricFieldSpanCount     = "$span_count"
	DerivedMetricFieldTraceDuration = "$trace_duration"
	DerivedMetricFieldErrorFlag     = "$error_flag"
)

var mgl = logger.DefaultSLogger("aggregate.metricgen")

// MetricGenerator 派生指标生成器
// 负责从trace/logging/RUM数据生成指标点
type MetricGenerator struct {
	// 缓存条件解析结果
	conditionCache map[string]fp.WhereConditions
	lock           sync.RWMutex
}

// NewMetricGenerator 创建新的指标生成器
func NewMetricGenerator() *MetricGenerator {
	return &MetricGenerator{
		conditionCache: make(map[string]fp.WhereConditions),
	}
}

// GenerateFromDataPacket 从DataPacket生成派生指标
// 返回生成的指标点列表
func (mg *MetricGenerator) GenerateFromDataPacket(packet *DataPacket, metric *DerivedMetric) []*point.Point {
	if packet == nil || metric == nil || metric.Algorithm == nil {
		return nil
	}

	// 1. 检查条件过滤
	if !mg.evaluateCondition(packet, metric.Condition) {
		return nil
	}

	// 2. 提取分组标签
	tags := mg.extractGroupTags(packet, metric.Groupby)

	// 3. 根据算法生成指标
	return mg.generateMetricsByAlgorithm(packet, metric, tags)
}

// evaluateCondition 评估条件是否满足
func (mg *MetricGenerator) evaluateCondition(packet *DataPacket, condition string) bool {
	if condition == "" {
		return true // 无条件，总是满足
	}

	// 从缓存获取或解析条件
	mg.lock.RLock()
	conds, ok := mg.conditionCache[condition]
	mg.lock.RUnlock()

	if !ok {
		// 解析条件
		if ast, err := fp.GetConds(condition); err != nil {
			mgl.Errorf("failed to parse condition %s: %v", condition, err)
			return false
		} else {
			mg.lock.Lock()
			mg.conditionCache[condition] = ast
			mg.lock.Unlock()
			conds = ast
		}
	}

	// 评估条件
	ptw := &ptWrap{}
	for _, span := range packet.Points {
		ptw.Point = point.FromPB(span)
		if result := conds.Eval(ptw); result >= 0 {
			return true // 条件满足
		}
	}

	return false
}

// extractGroupTags 提取分组标签
func (mg *MetricGenerator) extractGroupTags(packet *DataPacket, groupBy []string) map[string]string {
	tags := make(map[string]string)

	if len(groupBy) == 0 {
		return tags
	}

	// 从第一个span提取标签（假设同一个trace的span有相同的标签）
	if len(packet.Points) == 0 {
		return tags
	}

	pt := point.FromPB(packet.Points[0])
	ptw := &ptWrap{Point: pt}

	for _, key := range groupBy {
		if value, exists := ptw.Get(key); exists {
			tags[key] = mg.valueToString(value)
		}
	}

	return tags
}

// generateMetricsByAlgorithm 根据算法生成指标
func (mg *MetricGenerator) generateMetricsByAlgorithm(packet *DataPacket,
	metric *DerivedMetric, tags map[string]string) []*point.Point {
	algo := metric.Algorithm

	switch algo.Method {
	case COUNT:
		return mg.generateCountMetric(packet, algo, metric.Name, tags)
	case HISTOGRAM:
		return mg.generateHistogramMetric(packet, algo, metric.Name, tags)
	case QUANTILES:
		return mg.generateQuantileMetric(packet, algo, metric.Name, tags)
	case AVG, SUM, MIN, MAX:
		return mg.generateAggregationMetric(packet, algo, metric.Name, tags)
	case COUNT_DISTINCT:
		return mg.generateCountDistinctMetric(packet, algo, metric.Name, tags)
	default:
		mgl.Warnf("unsupported algorithm method: %s for metric: %s", algo.Method, metric.Name)
		return nil
	}
}

// generateCountMetric 生成计数指标
func (mg *MetricGenerator) generateCountMetric(packet *DataPacket, algo *AggregationAlgo,
	name string, tags map[string]string) []*point.Point {
	// 计算符合条件的span数量
	count := 0
	sourceField := algo.SourceField

	// 特殊处理：使用$trace_id表示统计trace数量
	if sourceField == DerivedMetricFieldTraceID {
		count = 1 // 每个trace计数1
	} else if sourceField == DerivedMetricFieldSpanCount {
		count = len(packet.Points) // span数量
	} else if sourceField == DerivedMetricFieldErrorFlag {
		if packet.HasError {
			count = 1
		}
	} else {
		// 统计具有特定字段的span数量
		for _, pbPoint := range packet.Points {
			pt := point.FromPB(pbPoint)
			ptw := &ptWrap{Point: pt}
			if value, exists := ptw.Get(sourceField); exists && value != nil {
				count++
			}
		}
	}

	if count == 0 {
		return nil
	}
	// 创建指标点
	fields := map[string]interface{}{
		name: float64(count),
	}

	return mg.createMetricPoint(TailSamplingDerivedMetricName, fields, tags)
}

// generateHistogramMetric 生成直方图指标
func (mg *MetricGenerator) generateHistogramMetric(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string) []*point.Point {
	// 获取直方图配置
	histogramOpts, ok := algo.Options.(*AggregationAlgo_HistogramOpts)
	if !ok || histogramOpts.HistogramOpts == nil {
		mgl.Warnf("missing histogram options for metric: %s", name)
		return nil
	}

	buckets := histogramOpts.HistogramOpts.Buckets
	if len(buckets) == 0 {
		return nil
	}

	// 提取源字段值
	values := mg.extractSourceValues(packet, algo.SourceField)
	if len(values) == 0 {
		return nil
	}

	// 计算直方图
	histogram := make(map[float64]int)
	infCount := 0

	for _, value := range values {
		placed := false
		for _, bucket := range buckets {
			if value <= bucket {
				histogram[bucket]++
				placed = true
				break
			}
		}
		if !placed {
			infCount++
		}
	}

	// 创建指标点
	points := make([]*point.Point, 0, len(histogram)+1)

	// 每个桶一个点
	for bucket, count := range histogram {
		fields := map[string]interface{}{
			name + "_bucket": float64(count),
		}
		bucketTags := make(map[string]string)
		for k, v := range tags {
			bucketTags[k] = v
		}
		bucketTags["le"] = strconv.FormatFloat(bucket, 'f', -1, 64)

		points = append(points, mg.createMetricPoint(TailSamplingDerivedMetricName, fields, bucketTags)...)
	}

	// +Inf 桶
	if infCount > 0 {
		fields := map[string]interface{}{
			name + "_bucket": float64(infCount),
		}
		infTags := make(map[string]string)
		for k, v := range tags {
			infTags[k] = v
		}
		infTags["le"] = "+Inf"

		points = append(points, mg.createMetricPoint(TailSamplingDerivedMetricName, fields, infTags)...)
	}

	// 总数
	totalFields := map[string]interface{}{
		name + "_count": float64(len(values)),
	}
	points = append(points, mg.createMetricPoint(TailSamplingDerivedMetricName, totalFields, tags)...)

	// 总和
	sum := 0.0
	for _, value := range values {
		sum += value
	}
	sumFields := map[string]interface{}{
		name + "_sum": sum,
	}
	points = append(points, mg.createMetricPoint(TailSamplingDerivedMetricName, sumFields, tags)...)

	return points
}

// generateQuantileMetric 生成百分位数指标
func (mg *MetricGenerator) generateQuantileMetric(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string) []*point.Point {
	// 获取百分位数配置
	quantileOpts, ok := algo.Options.(*AggregationAlgo_QuantileOpts)
	if !ok || quantileOpts.QuantileOpts == nil {
		mgl.Warnf("missing quantile options for metric: %s", name)
		return nil
	}

	percentiles := quantileOpts.QuantileOpts.Percentiles
	if len(percentiles) == 0 {
		return nil
	}

	// 提取源字段值
	values := mg.extractSourceValues(packet, algo.SourceField)
	if len(values) == 0 {
		return nil
	}

	// 排序并计算百分位数
	sortedValues := mg.sortFloat64Slice(values)
	points := make([]*point.Point, 0, len(percentiles))

	for _, percentile := range percentiles {
		if percentile < 0 || percentile > 1 {
			continue
		}

		index := int(math.Ceil(percentile*float64(len(sortedValues)))) - 1
		if index < 0 {
			index = 0
		}
		if index >= len(sortedValues) {
			index = len(sortedValues) - 1
		}

		value := sortedValues[index]

		fields := map[string]interface{}{
			name: value,
		}
		quantileTags := make(map[string]string)
		for k, v := range tags {
			quantileTags[k] = v
		}
		quantileTags["quantile"] = strconv.FormatFloat(percentile, 'f', -1, 64)

		points = append(points, mg.createMetricPoint(TailSamplingDerivedMetricName, fields, quantileTags)...)
	}

	return points
}

// generateAggregationMetric 生成聚合指标（AVG/SUM/MIN/MAX）
func (mg *MetricGenerator) generateAggregationMetric(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string) []*point.Point {
	values := mg.extractSourceValues(packet, algo.SourceField)
	if len(values) == 0 {
		return nil
	}

	var result float64
	switch algo.Method {
	case AVG:
		sum := 0.0
		for _, v := range values {
			sum += v
		}
		result = sum / float64(len(values))
	case SUM:
		result = 0.0
		for _, v := range values {
			result += v
		}
	case MIN:
		result = math.Inf(1)
		for _, v := range values {
			if v < result {
				result = v
			}
		}
	case MAX:
		result = math.Inf(-1)
		for _, v := range values {
			if v > result {
				result = v
			}
		}
	default:
		return nil
	}

	fields := map[string]interface{}{
		name: result,
	}

	return mg.createMetricPoint(TailSamplingDerivedMetricName, fields, tags)
}

// generateCountDistinctMetric 生成去重计数指标
func (mg *MetricGenerator) generateCountDistinctMetric(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string) []*point.Point {
	distinctValues := make(map[string]bool)

	for _, pbPoint := range packet.Points {
		pt := point.FromPB(pbPoint)
		ptw := &ptWrap{Point: pt}
		if value, exists := ptw.Get(algo.SourceField); exists && value != nil {
			strValue := mg.valueToString(value)
			if strValue != "" {
				distinctValues[strValue] = true
			}
		}
	}

	count := len(distinctValues)
	if count == 0 {
		return nil
	}

	fields := map[string]interface{}{
		name: float64(count),
	}

	return mg.createMetricPoint(TailSamplingDerivedMetricName, fields, tags)
}

// extractSourceValues 提取源字段值
func (mg *MetricGenerator) extractSourceValues(packet *DataPacket, sourceField string) []float64 {
	// 特殊字段处理
	if sourceField == DerivedMetricFieldTraceDuration {
		// 计算trace持续时间（纳秒）
		if packet.TraceStartTimeUnixNano > 0 &&
			packet.TraceEndTimeUnixNano > packet.TraceStartTimeUnixNano {

			// 纳秒转毫秒
			durationMs := float64(packet.TraceEndTimeUnixNano-packet.TraceStartTimeUnixNano) / 1e6
			return []float64{durationMs}
		}
		return nil
	}

	if sourceField == DerivedMetricFieldErrorFlag {
		// 错误标志：有错误为1，无错误为0
		if packet.HasError {
			return []float64{1.0}
		}
		return []float64{0.0}
	}

	// 普通字段提取
	values := make([]float64, 0)
	for _, pbPoint := range packet.Points {
		pt := point.FromPB(pbPoint)
		ptw := &ptWrap{Point: pt}
		if value, exists := ptw.Get(sourceField); exists {
			if f64, ok := mg.toFloat64(value); ok {
				values = append(values, f64)
			}
		}
	}

	return values
}

// createMetricPoint 创建指标点
func (mg *MetricGenerator) createMetricPoint(name string, fields map[string]interface{},
	tags map[string]string) []*point.Point {
	// 添加额外标签
	if len(tags) == 0 {
		tags = make(map[string]string)
	}

	// 创建点
	pt := point.NewPoint(name,
		append(point.NewTags(tags), point.NewKVs(fields)...),
		point.WithTime(time.Now()),
	)

	return []*point.Point{pt}
}

// Helper functions
func (mg *MetricGenerator) valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	case []byte:
		return string(v)
	default:
		return ""
	}
}

func (mg *MetricGenerator) toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case int:
		return float64(v), true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

func (mg *MetricGenerator) sortFloat64Slice(values []float64) []float64 {
	sorted := make([]float64, len(values))
	copy(sorted, values)

	// 简单冒泡排序（对于小数据集足够）
	for i := 0; i < len(sorted)-1; i++ {
		for j := 0; j < len(sorted)-i-1; j++ {
			if sorted[j] > sorted[j+1] {
				sorted[j], sorted[j+1] = sorted[j+1], sorted[j]
			}
		}
	}

	return sorted
}

// Global metric generator instance
var globalMetricGenerator = NewMetricGenerator()

func cloneAggregationAlgo(algo *AggregationAlgo) *AggregationAlgo {
	if algo == nil {
		return nil
	}

	cloned := *algo
	return &cloned
}

func metricFieldKey(pt *point.Point) string {
	if pt == nil {
		return ""
	}

	for _, kv := range pt.KVs() {
		if !kv.IsTag {
			return kv.Key
		}
	}

	return ""
}

func metricPointToAggregationBatch(pt *point.Point, algo *AggregationAlgo, window int64) *AggregationBatch {
	if pt == nil || algo == nil {
		return nil
	}

	if window <= 0 {
		window = DefaultDerivedMetricWindowSeconds
	}

	tagKeys := make([]string, 0)
	for _, kv := range pt.KVs() {
		if kv.IsTag {
			tagKeys = append(tagKeys, kv.Key)
		}
	}
	sort.Strings(tagKeys)

	fieldKey := metricFieldKey(pt)
	if fieldKey == "" {
		return nil
	}

	aggrAlgo := cloneAggregationAlgo(algo)
	aggrAlgo.SourceField = fieldKey
	aggrAlgo.Window = window

	return &AggregationBatch{
		RoutingKey: hash(pt, tagKeys),
		PickKey:    pickHash(pt, tagKeys),
		AggregationOpts: map[string]*AggregationAlgo{
			fieldKey: aggrAlgo,
		},
		Points: &point.PBPoints{
			Arr: []*point.PBPoint{pt.PBPoint()},
		},
	}
}

func (mg *MetricGenerator) buildCountMetricBatches(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string, window int64) []*AggregationBatch {
	countPoints := mg.generateCountMetric(packet, algo, name, tags)
	if len(countPoints) == 0 {
		return nil
	}

	batches := make([]*AggregationBatch, 0, len(countPoints))
	sumAlgo := &AggregationAlgo{
		Method:      SUM,
		SourceField: name,
	}

	for _, pt := range countPoints {
		if batch := metricPointToAggregationBatch(pt, sumAlgo, window); batch != nil {
			batches = append(batches, batch)
		}
	}

	return batches
}

func (mg *MetricGenerator) buildAggregationMetricBatches(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string, window int64) []*AggregationBatch {
	values := mg.extractSourceValues(packet, algo.SourceField)
	if len(values) == 0 {
		return nil
	}

	batches := make([]*AggregationBatch, 0, len(values))
	rawAlgo := cloneAggregationAlgo(algo)
	for _, value := range values {
		fields := map[string]interface{}{
			name: value,
		}
		pts := mg.createMetricPoint(TailSamplingDerivedMetricName, fields, tags)
		if len(pts) == 0 {
			continue
		}
		if batch := metricPointToAggregationBatch(pts[0], rawAlgo, window); batch != nil {
			batches = append(batches, batch)
		}
	}

	return batches
}

func (mg *MetricGenerator) buildHistogramMetricBatches(packet *DataPacket, algo *AggregationAlgo, name string, tags map[string]string, window int64) []*AggregationBatch {
	points := mg.generateHistogramMetric(packet, algo, name, tags)
	if len(points) == 0 {
		return nil
	}

	batches := make([]*AggregationBatch, 0, len(points))
	sumAlgo := &AggregationAlgo{
		Method:      SUM,
		SourceField: name,
	}

	for _, pt := range points {
		if batch := metricPointToAggregationBatch(pt, sumAlgo, window); batch != nil {
			batches = append(batches, batch)
		}
	}

	return batches
}

func (mg *MetricGenerator) buildMetricBatchesFromDataPacket(packet *DataPacket, metric *DerivedMetric, window int64) []*AggregationBatch {
	if packet == nil || metric == nil || metric.Algorithm == nil {
		return nil
	}

	if !mg.evaluateCondition(packet, metric.Condition) {
		return nil
	}

	tags := mg.extractGroupTags(packet, metric.Groupby)
	algo := metric.Algorithm

	switch algo.Method {
	case COUNT:
		return mg.buildCountMetricBatches(packet, algo, metric.Name, tags, window)
	case HISTOGRAM:
		return mg.buildHistogramMetricBatches(packet, algo, metric.Name, tags, window)
	case QUANTILES, AVG, SUM, MIN, MAX, STDEV:
		return mg.buildAggregationMetricBatches(packet, algo, metric.Name, tags, window)
	default:
		mgl.Warnf("unsupported batch build algorithm method: %s for metric: %s", algo.Method, metric.Name)
		return nil
	}
}

// GenerateDerivedMetrics 生成派生指标的全局函数
func GenerateDerivedMetrics(packet *DataPacket, metrics []*DerivedMetric) []*point.Point {
	if packet == nil || len(metrics) == 0 {
		return nil
	}

	allPoints := make([]*point.Point, 0)
	for _, metric := range metrics {
		points := globalMetricGenerator.GenerateFromDataPacket(packet, metric)
		if len(points) > 0 {
			allPoints = append(allPoints, points...)
		}
	}

	return allPoints
}

// BuildDerivedMetricBatches converts derived metrics into aggregation batches directly.
func BuildDerivedMetricBatches(packet *DataPacket, metrics []*DerivedMetric, window int64) []*AggregationBatch {
	if packet == nil || len(metrics) == 0 {
		return nil
	}

	if window <= 0 {
		window = DefaultDerivedMetricWindowSeconds
	}

	allBatches := make([]*AggregationBatch, 0)
	for _, metric := range metrics {
		batches := globalMetricGenerator.buildMetricBatchesFromDataPacket(packet, metric, window)
		if len(batches) > 0 {
			allBatches = append(allBatches, batches...)
		}
	}

	return allBatches
}
