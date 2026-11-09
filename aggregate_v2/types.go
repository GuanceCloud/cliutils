// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package aggregate_v2 provides high-throughput, low-allocation stream aggregation
// and tail-sampling engines with zero-copy concatenation and bounded memory budgets.
package aggregate_v2

import (
	"errors"
	"time"
)

// Compression formats.
const (
	PayloadCompressionNone int32 = 0
	PayloadCompressionZstd int32 = 1
)

// Common errors.
var (
	ErrBudgetExceeded  = errors.New("tail sampling memory budget exceeded")
	ErrSamplerClosed   = errors.New("sampler is closed")
	ErrInvalidConfig   = errors.New("invalid sampler configuration")
	ErrSpillFailed     = errors.New("spill payload to disk failed")
	ErrHydrationFailed = errors.New("hydrate spilled payload failed")
	ErrPacketNil       = errors.New("incoming data packet is nil")
	ErrUnsupportedType = errors.New("unsupported data type")
	ErrPayloadTooLarge = errors.New("payload chunk exceeds single-write limit")
)

// PayloadSpiller is an abstraction for offloading large payloads to disk.
type PayloadSpiller interface {
	Put(data []byte) (string, error)
	Get(key string) ([]byte, error)
	Delete(key string) error
}

// SamplerConfig holds configuration for the aggregate_v2 Sampler.
type SamplerConfig struct {
	// ShardCount is the number of internal time wheel shards (defaults to 64).
	ShardCount int

	// DefaultTTL is the default sampling wait duration (e.g. 15s or 30s).
	DefaultTTL time.Duration

	// MaxHoldDuration is the absolute upper bound for any trace's residence time
	// regardless of how many new spans continue to arrive (defaults to 30s).
	MaxHoldDuration time.Duration

	// MaxResidentMemoryBytes is the total heap memory budget for payload retention
	// across all active traces before spilling or backpressure kicks in (defaults to 2GB).
	MaxResidentMemoryBytes int64

	// SpillThresholdBytes is the per-trace size threshold above which payload is spilled
	// to disk to save memory (defaults to 128KB). 0 disables size-triggered spilling.
	SpillThresholdBytes int64

	// Spiller handles disk spilling for large or backpressured traces.
	Spiller PayloadSpiller

	// WarningWatermarkRatio is the memory budget utilization ratio (0.0 - 1.0)
	// that triggers proactive disk spilling (defaults to 0.70).
	WarningWatermarkRatio float64

	// CriticalWatermarkRatio is the memory budget utilization ratio (0.0 - 1.0)
	// that triggers backpressure rejection (defaults to 0.85).
	CriticalWatermarkRatio float64

	// Hook provides optional telemetry and lifecycle interceptors.
	Hook SamplingHook
}

// DefaultSamplerConfig returns recommended production defaults.
func DefaultSamplerConfig() *SamplerConfig {
	return &SamplerConfig{
		ShardCount:             64,
		DefaultTTL:             15 * time.Second,
		MaxHoldDuration:        30 * time.Second,
		MaxResidentMemoryBytes: 2 << 30,   // 2 GB
		SpillThresholdBytes:    128 << 10, // 128 KB
		WarningWatermarkRatio:  0.70,
		CriticalWatermarkRatio: 0.85,
	}
}

// SamplerStats reports live metrics and runtime state.
type SamplerStats struct {
	ActiveTraces         int64
	ResidentPayloadBytes int64
	SpilledPayloadBytes  int64
	SpilledTraces        int64
	IngestedPackets      int64
	DecidedKept          int64
	DecidedDropped       int64
	BudgetRejections     int64
	UrgentEvictions      int64
}

// Sampler is the core entry point of aggregate_v2.
type Sampler interface {
	// Ingest accepts a trace or telemetry packet into the time wheel.
	Ingest(packet *DataPacket) error

	// IngestAt accepts a trace packet at a specific timestamp.
	IngestAt(packet *DataPacket, now time.Time) error

	// IngestBytes accepts a raw HTTP body (DataBatch or DataPacket protobuf bytes),
	// unmarshals using pooled easyproto with zero allocations, and ingests all packets.
	IngestBytes(body []byte) error

	// IngestBytesAt accepts a raw HTTP body at a specific timestamp.
	IngestBytesAt(body []byte, now time.Time) error

	// IngestBatch ingests all packets contained in a BatchPackets.
	IngestBatch(batch *BatchPackets) error

	// IngestBatchAt ingests all packets in a BatchPackets at a specific timestamp.
	IngestBatchAt(batch *BatchPackets, now time.Time) error

	// AdvanceTime moves time wheel forward, returning kept packets and count of dropped packets.
	AdvanceTime(now time.Time) (kept []*DataPacket, droppedCount int, err error)

	// UpdateConfig dynamically updates sampling rules for a given token.
	UpdateConfig(token string, cfg *TailSamplingConfigs) error

	// UpdateConfigFromTOML parses raw TOML configuration bytes and applies it to the token.
	UpdateConfigFromTOML(token string, tomlData []byte) error

	// UpdateConfigFromJSON parses raw JSON configuration bytes and applies it to the token.
	UpdateConfigFromJSON(token string, jsonData []byte) error

	// Stats returns current operational statistics.
	Stats() SamplerStats

	// Close shuts down the sampler and frees all active buffers.
	Close() error
}
