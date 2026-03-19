package aggregate

import "time"

type TailSamplingBuiltinMetric interface {
	Name() string
	OnIngest(packet *DataPacket) []DerivedMetricRecord
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

type TailSamplingProcessor struct {
	sampler   *GlobalSampler
	collector *DerivedMetricCollector
	metrics   TailSamplingBuiltinMetrics
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

func (r *TailSamplingProcessor) UpdateConfig(token string, cfg *TailSamplingConfigs) {
	if r == nil || r.sampler == nil || cfg == nil {
		return
	}

	r.sampler.UpdateConfig(token, cfg)
}

func (r *TailSamplingProcessor) IngestPacket(packet *DataPacket) {
	if r == nil || packet == nil {
		return
	}

	if r.collector != nil && len(r.metrics) > 0 {
		r.collector.Add(r.metrics.OnIngest(packet))
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

func (r *TailSamplingProcessor) TailSamplingData(dataGroups map[uint64]*DataGroup) map[uint64]*DataPacket {
	if r == nil || r.sampler == nil {
		return nil
	}

	outcomes := r.sampler.TailSamplingOutcomes(dataGroups)
	keptPackets := make(map[uint64]*DataPacket)

	if r.collector == nil || len(r.metrics) == 0 {
		for key, outcome := range outcomes {
			if outcome == nil || outcome.Packet == nil {
				continue
			}
			keptPackets[key] = outcome.Packet
		}
		return keptPackets
	}

	for key, outcome := range outcomes {
		if outcome == nil {
			continue
		}

		packetForMetrics := outcome.SourcePacket
		if packetForMetrics != nil {
			r.collector.Add(r.metrics.OnDecision(packetForMetrics, outcome.Decision))
		}

		if outcome.Packet != nil {
			keptPackets[key] = outcome.Packet
		}
	}

	return keptPackets
}

func (r *TailSamplingProcessor) RecordDecision(packet *DataPacket, decision DerivedMetricDecision) {
	if r == nil || packet == nil || r.collector == nil || len(r.metrics) == 0 {
		return
	}

	r.collector.Add(r.metrics.OnDecision(packet, decision))
}

func (r *TailSamplingProcessor) FlushDerivedMetrics(now time.Time) []*DerivedMetricPoints {
	if r == nil || r.collector == nil {
		return nil
	}

	return r.collector.Flush(now)
}
