// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memorySpiller struct {
	mu    sync.Mutex
	files map[string][]byte
}

func newMemorySpiller() *memorySpiller {
	return &memorySpiller{files: make(map[string][]byte)}
}

func (s *memorySpiller) Put(data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := fmt.Sprintf("spill-%d", len(s.files)+1)
	s.files[key] = append([]byte(nil), data...)
	return key, nil
}

func (s *memorySpiller) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.files[key]
	if !ok {
		return nil, fmt.Errorf("file not found: %s", key)
	}
	return append([]byte(nil), d...), nil
}

func (s *memorySpiller) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, key)
	return nil
}

func TestTimeWheelSamplerIngestAndDecision(t *testing.T) {
	spiller := newMemorySpiller()
	cfg := &SamplerConfig{
		ShardCount:             8,
		DefaultTTL:             2 * time.Second,
		MaxHoldDuration:        5 * time.Second,
		MaxResidentMemoryBytes: 100 << 20, // 100MB
		SpillThresholdBytes:    64 << 10,  // 64KB
		Spiller:                spiller,
	}

	sampler, err := NewSampler(cfg)
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	// Configure rules: keep error traces, keep duration >= 500000000 ns, drop everything else
	rules := &TailSamplingConfigs{
		Tracing: &TraceTailSampling{
			DataTTL:         2 * time.Second,
			MaxHoldDuration: 5 * time.Second,
			Pipelines: []*SamplingPipeline{
				{
					Name:      "keep-errors",
					Type:      PipelineTypeCondition,
					Condition: "{ status = 'error' }",
					Action:    PipelineActionKeep,
				},
				{
					Name:      "keep-slow",
					Type:      PipelineTypeCondition,
					Condition: "{ duration >= 500000000 }",
					Action:    PipelineActionKeep,
				},
				{
					Name:      "drop-others",
					Type:      PipelineTypeCondition,
					Condition: "",
					Action:    PipelineActionDrop,
				},
			},
		},
	}
	require.NoError(t, sampler.UpdateConfig("tkn_test", rules))

	payloadNormal, _ := CompressPayload([]byte("span-normal-trace-data"))
	payloadError, _ := CompressPayload([]byte("span-error-trace-data"))
	payloadSlow, _ := CompressPayload([]byte("span-slow-trace-data"))

	// 1. Normal trace (should be dropped)
	err = sampler.Ingest(&DataPacket{
		GroupIdHash:        1001,
		RawGroupId:         "trace-normal",
		Token:              "tkn_test",
		DataType:           "tracing",
		PointsPayload:      payloadNormal,
		PayloadCompression: PayloadCompressionZstd,
	})
	require.NoError(t, err)

	// 2. Error trace (should be kept)
	err = sampler.Ingest(&DataPacket{
		GroupIdHash:        1002,
		RawGroupId:         "trace-error",
		Token:              "tkn_test",
		DataType:           "tracing",
		HasError:           true,
		PredError:          true,
		PointsPayload:      payloadError,
		PayloadCompression: PayloadCompressionZstd,
	})
	require.NoError(t, err)

	// 3. Slow trace (should be kept)
	err = sampler.Ingest(&DataPacket{
		GroupIdHash:        1003,
		RawGroupId:         "trace-slow",
		Token:              "tkn_test",
		DataType:           "tracing",
		MaxSpanDurationUs:  600000, // 600ms = 600,000us
		PointsPayload:      payloadSlow,
		PayloadCompression: PayloadCompressionZstd,
	})
	require.NoError(t, err)

	// Advance time wheel past TTL (3 seconds)
	var allKept []*DataPacket
	totalDropped := 0
	now := time.Now()

	for range 4 {
		now = now.Add(time.Second)
		kept, dropped, err := sampler.AdvanceTime(now)
		require.NoError(t, err)
		allKept = append(allKept, kept...)
		totalDropped += dropped
	}

	assert.Equal(t, 1, totalDropped, "normal trace must be dropped")
	require.Len(t, allKept, 2, "error trace and slow trace must be kept")

	keptIDs := make(map[string]bool)
	for _, p := range allKept {
		keptIDs[p.RawGroupId] = true
	}
	assert.True(t, keptIDs["trace-error"])
	assert.True(t, keptIDs["trace-slow"])
}

func TestBoundedLeasesPreventsLiveLeak(t *testing.T) {
	// Verify that repeatedly feeding spans to the same trace does NOT keep it in memory
	// past MaxHoldDuration.
	cfg := &SamplerConfig{
		ShardCount:             4,
		DefaultTTL:             2 * time.Second,
		MaxHoldDuration:        4 * time.Second, // Hard deadline at 4s
		MaxResidentMemoryBytes: 10 << 20,
	}

	sampler, err := NewSampler(cfg)
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	payload, _ := CompressPayload([]byte("span-continuous-arrival"))

	now := time.Now()
	// Ingest first span at T=0
	err = sampler.IngestAt(&DataPacket{
		GroupIdHash:        9999,
		RawGroupId:         "trace-infinite-stream",
		Token:              "tkn_test",
		DataType:           "tracing",
		PointsPayload:      payload,
		PayloadCompression: PayloadCompressionZstd,
	}, now)
	require.NoError(t, err)

	// Keep feeding spans at T=1s, 2s, 3s, 4s, 5s...
	evicted := false
	for range 7 {
		now = now.Add(time.Second)
		// Feed another span fragment attempting to push TTL further
		_ = sampler.IngestAt(&DataPacket{
			GroupIdHash:        9999,
			RawGroupId:         "trace-infinite-stream",
			Token:              "tkn_test",
			DataType:           "tracing",
			PointsPayload:      payload,
			PayloadCompression: PayloadCompressionZstd,
		}, now)

		kept, dropped, err := sampler.AdvanceTime(now)
		require.NoError(t, err)
		if len(kept) > 0 || dropped > 0 {
			evicted = true
			break
		}
	}

	assert.True(t, evicted, "trace must be forcibly evicted within MaxHoldDuration (<= 4s), preventing live memory leak")
}

func TestMemoryBudgetFastCheckAdmission(t *testing.T) {
	budget := NewMemoryBudget(1000, 0.70, 0.85) // Max 1000 bytes, critical at 850

	// Admitting 500 bytes: ok
	assert.NoError(t, budget.FastCheck(500))
	budget.AddResident(500)
	assert.Equal(t, WatermarkGreen, budget.Level())

	// Adding 250 bytes: reaches 750 (warning watermark)
	budget.AddResident(250)
	assert.Equal(t, WatermarkYellow, budget.Level())

	// Adding 150 bytes: reaches 900 (critical watermark)
	budget.AddResident(150)
	assert.Equal(t, WatermarkRed, budget.Level())

	// FastCheck now rejects new incoming packets!
	assert.ErrorIs(t, budget.FastCheck(50), ErrBudgetExceeded)
	assert.Equal(t, int64(1), budget.Rejections())
}

func TestUniqueTraceCounter(t *testing.T) {
	counter := NewUniqueTraceCounter(100)
	counter.Add("trace-001")
	counter.Add("trace-002")
	counter.Add("trace-001") // Duplicate
	counter.Add("trace-003")

	assert.Equal(t, 3, counter.Count())
	assert.Equal(t, 3, counter.FlushAndReset())
	assert.Equal(t, 0, counter.Count())
}
