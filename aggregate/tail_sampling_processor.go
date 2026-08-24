package aggregate

import (
	"sync"
	"time"

	"github.com/GuanceCloud/cliutils/point"
)

type TailSamplingBuiltinMetric interface {
	Name() string
	OnIngest(packet *DataPacket) []DerivedMetricRecord
	OnPreDecision(packet *DataPacket) []DerivedMetricRecord
	OnDecision(packet *DataPacket, decision DerivedMetricDecision) []DerivedMetricRecord
}

type TailSamplingBuiltinMetrics []TailSamplingBuiltinMetric

func (ms TailSamplingBuiltinMetrics) OnIngest(packet *DataPacket) []DerivedMetricRecord {
	var records []DerivedMetricRecord
	for _, metric := range ms {
		records = append(records, metric.OnIngest(packet)...)
	}
	return records
}

func (ms TailSamplingBuiltinMetrics) OnDecision(packet *DataPacket, decision DerivedMetricDecision) []DerivedMetricRecord {
	var records []DerivedMetricRecord
	for _, metric := range ms {
		records = append(records, metric.OnDecision(packet, decision)...)
	}
	return records
}

func (ms TailSamplingBuiltinMetrics) OnPreDecision(packet *DataPacket) []DerivedMetricRecord {
	var records []DerivedMetricRecord
	for _, metric := range ms {
		records = append(records, metric.OnPreDecision(packet)...)
	}
	return records
}

type TailSamplingProcessor struct {
	sampler   *GlobalSampler
	collector *DerivedMetricCollector
	metrics   TailSamplingBuiltinMetrics
}

// TailSamplingTickResult contains the decisions and critical-path timings for
// one time-wheel tick. Outcomes intentionally omit internal shard/map keys:
// callers only need the decision and packet, while shard ownership remains an
// implementation detail of TailSamplingProcessor.
type TailSamplingTickResult struct {
	Outcomes                 []*TailSamplingOutcome
	ExpiredGroups            int
	ShardCount               int
	ActiveShards             int
	AdvanceDuration          time.Duration
	DecisionDuration         time.Duration
	MaxShardDecisionDuration time.Duration
	TotalDuration            time.Duration
}

func NewDefaultTailSamplingProcessor(shardCount int, waitTime time.Duration) *TailSamplingProcessor {
	return &TailSamplingProcessor{
		sampler:   NewGlobalSampler(shardCount, waitTime),
		collector: NewDerivedMetricCollector(DefaultDerivedMetricFlushWindow),
		metrics:   DefaultTailSamplingBuiltinMetrics(),
	}
}

func NewTailSamplingProcessor(
	sampler *GlobalSampler,
	collector *DerivedMetricCollector,
	metrics TailSamplingBuiltinMetrics,
) *TailSamplingProcessor {
	return &TailSamplingProcessor{
		sampler:   sampler,
		collector: collector,
		metrics:   metrics,
	}
}

func (r *TailSamplingProcessor) Sampler() *GlobalSampler {
	if r == nil {
		return nil
	}
	return r.sampler
}

func (r *TailSamplingProcessor) Collector() *DerivedMetricCollector {
	if r == nil {
		return nil
	}
	return r.collector
}

func (r *TailSamplingProcessor) BuiltinMetrics() TailSamplingBuiltinMetrics {
	if r == nil {
		return nil
	}
	return r.metrics
}

func (r *TailSamplingProcessor) UpdateConfig(token string, cfg *TailSamplingConfigs) error {
	if r == nil || r.sampler == nil || cfg == nil {
		return nil
	}

	return r.sampler.UpdateConfig(token, cfg)
}

func (r *TailSamplingProcessor) IngestPacket(packet *DataPacket) {
	if r == nil || packet == nil {
		return
	}

	if r.collector != nil && len(r.metrics) > 0 {
		r.collector.Add(r.filterBuiltinRecords(packet, r.metrics.OnIngest(packet)))
	}

	if r.sampler != nil {
		r.sampler.Ingest(packet)
	}
}

func (r *TailSamplingProcessor) AdvanceTime() map[uint64]*DataGroup {
	if r == nil || r.sampler == nil {
		return nil
	}

	return r.sampler.AdvanceTime()
}

// ProcessTick advances every time-wheel shard and evaluates expired groups in
// parallel. It preserves the previous two-phase ordering: all shards advance
// before any decision starts, so callers get the same tick semantics as
// AdvanceTime followed by TailSamplingOutcomes.
func (r *TailSamplingProcessor) ProcessTick() TailSamplingTickResult {
	var result TailSamplingTickResult
	if r == nil || r.sampler == nil || len(r.sampler.shards) == 0 {
		return result
	}

	started := time.Now()
	result.ShardCount = len(r.sampler.shards)
	expiredByShard := make([]map[uint64]*DataGroup, len(r.sampler.shards))

	advanceStarted := time.Now()
	var wg sync.WaitGroup
	wg.Add(len(r.sampler.shards))
	for index, shard := range r.sampler.shards {
		go func() {
			defer wg.Done()
			expiredByShard[index] = advanceTailSamplingShard(shard)
		}()
	}
	wg.Wait()
	result.AdvanceDuration = time.Since(advanceStarted)

	for _, expired := range expiredByShard {
		if len(expired) == 0 {
			continue
		}
		result.ActiveShards++
		result.ExpiredGroups += len(expired)
	}
	if result.ExpiredGroups == 0 {
		result.TotalDuration = time.Since(started)
		return result
	}

	outcomesByShard := make([]map[uint64]*TailSamplingOutcome, len(expiredByShard))
	decisionDurations := make([]time.Duration, len(expiredByShard))
	decisionStarted := time.Now()
	for index, expired := range expiredByShard {
		if len(expired) == 0 {
			continue
		}

		wg.Go(func() {
			shardStarted := time.Now()
			outcomesByShard[index] = r.TailSamplingOutcomes(expired)
			decisionDurations[index] = time.Since(shardStarted)
		})
	}
	wg.Wait()
	result.DecisionDuration = time.Since(decisionStarted)

	result.Outcomes = make([]*TailSamplingOutcome, 0, result.ExpiredGroups)
	for index, outcomes := range outcomesByShard {
		if decisionDurations[index] > result.MaxShardDecisionDuration {
			result.MaxShardDecisionDuration = decisionDurations[index]
		}
		for _, outcome := range outcomes {
			result.Outcomes = append(result.Outcomes, outcome)
		}
	}
	result.TotalDuration = time.Since(started)

	return result
}

func (r *TailSamplingProcessor) TailSamplingData(dataGroups map[uint64]*DataGroup) map[uint64]*DataPacket {
	outcomes := r.TailSamplingOutcomes(dataGroups)
	if outcomes == nil {
		return nil
	}

	keptPackets := make(map[uint64]*DataPacket)

	for key, outcome := range outcomes {
		if outcome == nil || outcome.Packet == nil {
			continue
		}
		keptPackets[key] = outcome.Packet
	}

	return keptPackets
}

func (r *TailSamplingProcessor) TailSamplingOutcomes(dataGroups map[uint64]*DataGroup) map[uint64]*TailSamplingOutcome {
	if r == nil || r.sampler == nil {
		return nil
	}

	if r.collector != nil && len(r.metrics) > 0 {
		for _, dg := range dataGroups {
			if dg == nil || dg.packet == nil {
				continue
			}
			r.collector.Add(r.filterBuiltinRecords(dg.packet, r.metrics.OnPreDecision(dg.packet)))
		}
	}

	outcomes := r.sampler.TailSamplingOutcomes(dataGroups)

	if r.collector == nil || len(r.metrics) == 0 {
		return outcomes
	}

	for _, outcome := range outcomes {
		if outcome == nil {
			continue
		}

		packetForMetrics := outcome.SourcePacket
		if packetForMetrics != nil {
			r.collector.Add(r.filterBuiltinRecords(packetForMetrics, r.metrics.OnDecision(packetForMetrics, outcome.Decision)))
		}
	}

	return outcomes
}

func (r *TailSamplingProcessor) RecordDecision(packet *DataPacket, decision DerivedMetricDecision) {
	if r == nil || packet == nil || r.collector == nil || len(r.metrics) == 0 {
		return
	}

	r.collector.Add(r.filterBuiltinRecords(packet, r.metrics.OnDecision(packet, decision)))
}

func (r *TailSamplingProcessor) FlushDerivedMetrics(now time.Time) []*DerivedMetricPoints {
	if r == nil || r.collector == nil {
		return nil
	}

	return r.collector.Flush(now)
}

func (r *TailSamplingProcessor) filterBuiltinRecords(packet *DataPacket, records []DerivedMetricRecord) []DerivedMetricRecord {
	if len(records) == 0 || packet == nil {
		return records
	}

	filtered := make([]DerivedMetricRecord, 0, len(records))
	for _, record := range records {
		if r.isBuiltinMetricEnabled(packet.Token, packet.DataType, record.MetricName) {
			filtered = append(filtered, record)
		}
	}

	return filtered
}

func (r *TailSamplingProcessor) isBuiltinMetricEnabled(token, dataType, metricName string) bool {
	if r == nil || r.sampler == nil {
		return true
	}

	var cfgs []*BuiltinMetricCfg

	switch dataType {
	case point.STracing:
		traceCfg := r.sampler.GetTraceConfig(token)
		if traceCfg == nil {
			return true
		}
		cfgs = traceCfg.BuiltinMetrics
	case point.SLogging:
		loggingCfg := r.sampler.GetLoggingConfig(token)
		if loggingCfg == nil {
			return true
		}
		cfgs = loggingCfg.BuiltinMetrics
	case point.SRUM:
		rumCfg := r.sampler.GetRUMConfig(token)
		if rumCfg == nil {
			return true
		}
		cfgs = rumCfg.BuiltinMetrics
	default:
		return true
	}

	if len(cfgs) == 0 {
		return true
	}

	for _, cfg := range cfgs {
		if cfg == nil {
			continue
		}
		if cfg.Name == metricName {
			return cfg.Enabled
		}
	}

	return true
}
