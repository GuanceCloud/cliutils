// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"
)

// DefaultTimeWheelSampler is the production implementation of the aggregate_v2 Sampler interface.
type DefaultTimeWheelSampler struct {
	shards     []*Shard
	shardCount int
	budget     *MemoryBudget
	config     *SamplerConfig

	configMu  sync.RWMutex
	configMap map[string]*TailSamplingConfigs

	// Stats counters
	ingestedPackets  atomic.Int64
	decidedKept      atomic.Int64
	decidedDropped   atomic.Int64
	urgentEvictions  atomic.Int64
	budgetRejections atomic.Int64

	closed atomic.Bool
}

// NewSampler creates a new initialized aggregate_v2 Sampler.
func NewSampler(cfg *SamplerConfig) (*DefaultTimeWheelSampler, error) {
	if cfg == nil {
		cfg = DefaultSamplerConfig()
	}
	if cfg.ShardCount <= 0 {
		cfg.ShardCount = 64
	}
	if cfg.DefaultTTL <= 0 {
		cfg.DefaultTTL = 15 * time.Second
	}
	if cfg.MaxHoldDuration <= 0 {
		cfg.MaxHoldDuration = 30 * time.Second
	}

	budget := NewMemoryBudget(
		cfg.MaxResidentMemoryBytes,
		cfg.WarningWatermarkRatio,
		cfg.CriticalWatermarkRatio,
	)

	s := &DefaultTimeWheelSampler{
		shards:     make([]*Shard, cfg.ShardCount),
		shardCount: cfg.ShardCount,
		budget:     budget,
		config:     cfg,
		configMap:  make(map[string]*TailSamplingConfigs),
	}

	for i := range s.shards {
		s.shards[i] = NewShard(budget, cfg.Spiller, cfg.SpillThresholdBytes)
	}

	return s, nil
}

// Ingest accepts an incoming telemetry packet and routes it into the appropriate time wheel shard.
func (s *DefaultTimeWheelSampler) Ingest(packet *DataPacket) error {
	return s.IngestAt(packet, time.Now())
}

// IngestAt accepts an incoming packet at a specific time (useful for deterministic tests).
func (s *DefaultTimeWheelSampler) IngestAt(packet *DataPacket, now time.Time) error {
	if s.closed.Load() {
		return ErrSamplerClosed
	}
	if packet == nil {
		return ErrPacketNil
	}

	// 1. Admission Gate: check memory budget before processing
	payloadSize := int64(len(packet.PointsPayload))
	if err := s.budget.FastCheck(payloadSize); err != nil {
		s.budgetRejections.Add(1)
		return err
	}

	// 2. Ensure payload is compressed using zstd
	if len(packet.PointsPayload) > 0 && packet.PayloadCompression != PayloadCompressionZstd {
		compressed, compType, err := CompressIfNeeded(packet.PointsPayload, packet.PayloadCompression)
		if err != nil {
			return fmt.Errorf("compress points payload failed: %w", err)
		}
		packet.PointsPayload = compressed
		packet.PayloadCompression = compType
	}

	// 3. Resolve TTL and MaxHold configuration
	ttlSec, maxHoldSec := s.resolveTTL(packet)

	// 4. Hash route to shard
	shardIdx := packet.GroupIdHash % uint64(s.shardCount)
	shard := s.shards[shardIdx]

	if err := shard.Ingest(packet, ttlSec, maxHoldSec, now); err != nil {
		return err
	}

	if s.config.Hook != nil {
		s.config.Hook.OnIngest(packet)
	}

	s.ingestedPackets.Add(1)
	return nil
}

// IngestBytes accepts a raw HTTP body (DataBatch or DataPacket protobuf bytes),
// unmarshals using pooled easyproto with zero allocations, and ingests all packets.
func (s *DefaultTimeWheelSampler) IngestBytes(body []byte) error {
	return s.IngestBytesAt(body, time.Now())
}

// IngestBytesAt accepts a raw HTTP body at a specific timestamp.
func (s *DefaultTimeWheelSampler) IngestBytesAt(body []byte, now time.Time) error {
	if s.closed.Load() {
		return ErrSamplerClosed
	}
	if len(body) == 0 {
		return nil
	}

	bp := NewBatchPackets()
	defer bp.Release()

	if err := bp.Unmarshal(body); err != nil {
		return fmt.Errorf("easyproto unmarshal failed: %w", err)
	}

	return s.IngestBatchAt(bp, now)
}

// IngestBatch ingests all packets contained in a BatchPackets.
func (s *DefaultTimeWheelSampler) IngestBatch(batch *BatchPackets) error {
	return s.IngestBatchAt(batch, time.Now())
}

// IngestBatchAt ingests all packets in a BatchPackets at a specific timestamp.
func (s *DefaultTimeWheelSampler) IngestBatchAt(batch *BatchPackets, now time.Time) error {
	if s.closed.Load() {
		return ErrSamplerClosed
	}
	if batch == nil || len(batch.Packets) == 0 {
		return nil
	}

	for _, packet := range batch.Packets {
		if err := s.IngestAt(packet, now); err != nil {
			return err
		}
	}
	return nil
}

// AdvanceTime ticks the time wheel forward, evaluates sampling rules for expired traces,
// and returns kept packets while freeing dropped traces.
func (s *DefaultTimeWheelSampler) AdvanceTime(now time.Time) ([]*DataPacket, int, error) {
	if s.closed.Load() {
		return nil, 0, ErrSamplerClosed
	}

	var keptPackets []*DataPacket
	droppedCount := 0

	for _, shard := range s.shards {
		expiredMetas := shard.AdvanceSlot()
		if len(expiredMetas) == 0 {
			continue
		}

		for _, meta := range expiredMetas {
			plan := s.planFor(meta)
			decision := plan.Evaluate(meta)

			if s.config.Hook != nil {
				s.config.Hook.OnDecision(meta, decision.Action, decision.RuleName)
			}

			if decision.Action == PipelineActionKeep {
				// Keep path: materialize payload and build outgoing DataPacket
				packet, err := s.materializePacket(meta)
				if err != nil {
					// On hydration error: log and drop to avoid crash
					FreeTraceMeta(meta)
					droppedCount++
					s.decidedDropped.Add(1)
					continue
				}
				keptPackets = append(keptPackets, packet)
				s.decidedKept.Add(1)
				// Do not call FreeTraceMeta yet; payload ownership transferred to packet
				meta.Payload = nil
				FreeTraceMeta(meta)
			} else {
				// Drop path: 0 allocations, 0 decompression, immediate release
				FreeTraceMeta(meta)
				droppedCount++
				s.decidedDropped.Add(1)
			}
		}
	}

	return keptPackets, droppedCount, nil
}

func (s *DefaultTimeWheelSampler) resolveTTL(packet *DataPacket) (int, int) {
	s.configMu.RLock()
	cfg := s.configMap[packet.Token]
	s.configMu.RUnlock()

	defaultTTL := int(s.config.DefaultTTL.Seconds())
	defaultMaxHold := int(s.config.MaxHoldDuration.Seconds())

	if cfg == nil {
		return defaultTTL, defaultMaxHold
	}

	switch packet.DataType {
	case "tracing":
		if cfg.Tracing != nil {
			ttl := int(cfg.Tracing.DataTTL.Seconds())
			maxHold := int(cfg.Tracing.MaxHoldDuration.Seconds())
			if ttl <= 0 {
				ttl = defaultTTL
			}
			if maxHold <= 0 {
				maxHold = defaultMaxHold
			}
			return ttl, maxHold
		}
	case "logging":
		if cfg.Logging != nil {
			ttl := int(cfg.Logging.DataTTL.Seconds())
			maxHold := int(cfg.Logging.MaxHoldDuration.Seconds())
			if ttl <= 0 {
				ttl = defaultTTL
			}
			if maxHold <= 0 {
				maxHold = defaultMaxHold
			}
			return ttl, maxHold
		}
	case "rum":
		if cfg.RUM != nil {
			ttl := int(cfg.RUM.DataTTL.Seconds())
			maxHold := int(cfg.RUM.MaxHoldDuration.Seconds())
			if ttl <= 0 {
				ttl = defaultTTL
			}
			if maxHold <= 0 {
				maxHold = defaultMaxHold
			}
			return ttl, maxHold
		}
	}

	return defaultTTL, defaultMaxHold
}

func (s *DefaultTimeWheelSampler) planFor(meta *TraceMeta) *compiledDecisionPlan {
	s.configMu.RLock()
	cfg := s.configMap[meta.Token]
	s.configMu.RUnlock()

	if cfg == nil {
		return nil
	}

	switch meta.DataType {
	case "tracing":
		if cfg.Tracing != nil {
			return cfg.Tracing.compiledPlan
		}
	case "logging":
		if cfg.Logging != nil {
			for _, gd := range cfg.Logging.GroupDimensions {
				if gd != nil && gd.GroupKey == meta.GroupKey {
					return gd.compiledPlan
				}
			}
		}
	case "rum":
		if cfg.RUM != nil {
			for _, gd := range cfg.RUM.GroupDimensions {
				if gd != nil && gd.GroupKey == meta.GroupKey {
					return gd.compiledPlan
				}
			}
		}
	}
	return nil
}

func (s *DefaultTimeWheelSampler) materializePacket(m *TraceMeta) (*DataPacket, error) {
	var payload []byte
	if m.Payload != nil {
		b, err := m.Payload.Bytes()
		if err != nil {
			return nil, err
		}
		payload = b
		m.Payload.Release()
	}

	return &DataPacket{
		GroupIdHash:             m.GroupIDHash,
		RawGroupId:              m.RawGroupID,
		Token:                   m.Token,
		Source:                  m.Source,
		DataType:                m.DataType,
		ConfigVersion:           m.ConfigVersion,
		HasError:                m.HasError(),
		GroupKey:                m.GroupKey,
		PointCount:              m.PointCount,
		TraceStartTimeUnixNano:  m.TraceStartTimeUnixNano,
		TraceEndTimeUnixNano:    m.TraceEndTimeUnixNano,
		PointsPayload:           payload,
		MaxPointTimeUnixNano:    m.MaxPointTimeUnixNano,
		PayloadCompression:      PayloadCompressionZstd,
		PredError:               (m.PredMask & PredFlagPredError) != 0,
		PredHttpError:           m.HasHttpError(),
		PredBizError:            m.HasBizError(),
		PredTraceKeep:           m.HasTraceKeep(),
		MaxSpanDurationUs:       m.MaxSpanDurationUs,
		RootDurationUs:          m.RootDurationUs,
		MaxNonrootDurationUs:    m.MaxNonrootDurationUs,
		PredicateSummaryVersion: m.PredicateSummaryVersion,
	}, nil
}

// UpdateConfig updates sampling configuration for a specific token.
func (s *DefaultTimeWheelSampler) UpdateConfig(token string, cfg *TailSamplingConfigs) error {
	if s.closed.Load() {
		return ErrSamplerClosed
	}
	if cfg != nil {
		if err := cfg.Init(); err != nil {
			return err
		}
	}

	s.configMu.Lock()
	if cfg == nil {
		delete(s.configMap, token)
	} else {
		s.configMap[token] = cfg
	}
	s.configMu.Unlock()
	return nil
}

// Stats returns live statistics of the sampler.
func (s *DefaultTimeWheelSampler) Stats() SamplerStats {
	var activeTraces int64
	for _, shard := range s.shards {
		activeTraces += int64(shard.ActiveCount())
	}

	return SamplerStats{
		ActiveTraces:         activeTraces,
		ResidentPayloadBytes: s.budget.ResidentBytes(),
		SpilledPayloadBytes:  s.budget.SpilledBytes(),
		IngestedPackets:      s.ingestedPackets.Load(),
		DecidedKept:          s.decidedKept.Load(),
		DecidedDropped:       s.decidedDropped.Load(),
		BudgetRejections:     s.budgetRejections.Load(),
		UrgentEvictions:      s.urgentEvictions.Load(),
	}
}

// Close closes all shards and releases all allocated memory.
func (s *DefaultTimeWheelSampler) Close() error {
	if s.closed.Swap(true) {
		return nil
	}

	for _, shard := range s.shards {
		shard.Close()
	}
	return nil
}

// HashToken combines a token and groupIDHash into a 64-bit key.
func HashToken(token string, groupIDHash uint64) uint64 {
	h := xxhash.Sum64String(token)
	return h ^ groupIDHash
}
