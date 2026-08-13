package aggregate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSpiller struct {
	mu          sync.Mutex
	data        map[string][]byte
	putCalls    int
	getCalls    int
	deleteCalls int
	failGet     bool
}

func newMockSpiller() *mockSpiller {
	return &mockSpiller{data: map[string][]byte{}}
}

func (m *mockSpiller) Put(payload []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("k%d", m.putCalls)
	m.data[key] = payload
	m.putCalls++
	return key, nil
}

func (m *mockSpiller) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getCalls++
	if m.failGet {
		return nil, fmt.Errorf("key %s read failed", key)
	}
	payload, ok := m.data[key]
	if !ok {
		return nil, fmt.Errorf("key %s not found", key)
	}
	return payload, nil
}

func (m *mockSpiller) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deleteCalls++
	delete(m.data, key)
	return nil
}

func (m *mockSpiller) Close() error { return nil }

func (m *mockSpiller) getCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getCalls
}

func (m *mockSpiller) deleteCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.deleteCalls
}

func (m *mockSpiller) putCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.putCalls
}

func TestFilePayloadSpiller(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	spiller, err := NewFilePayloadSpiller(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = spiller.Close() })

	payload := []byte(strings.Repeat("spill-payload-", 1024))
	key, err := spiller.Put(payload)
	require.NoError(t, err)

	got, err := spiller.Get(key)
	require.NoError(t, err)
	assert.Equal(t, payload, got)

	require.NoError(t, spiller.Delete(key))
	require.NoError(t, spiller.Delete(key)) // idempotent
	_, err = spiller.Get(key)
	require.Error(t, err)

	_, err = spiller.Get("../../etc/passwd")
	require.Error(t, err)
	require.NoError(t, spiller.Delete("../../etc/passwd"))
}

func TestFilePayloadSpillerClearsOnOpen(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "leftover"), []byte("stale"), 0o600))

	spiller, err := NewFilePayloadSpiller(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = spiller.Close() })

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "打开时应清空上次运行遗留的 spill 文件")
}

func TestSamplerSpillLargePacketKept(t *testing.T) {
	const token = "tkn_spill_kept"

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1024)

	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
	}))

	packet := &DataPacket{
		GroupIdHash:          1,
		RawGroupId:           "trace-spill",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           100,
		PointsPayload:        []byte(strings.Repeat("x", 4096)),
		MaxSpanDurationUs:    1000,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(packet)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	for _, dg := range expired {
		require.NotEmpty(t, dg.spillKey, "大包应落盘")
		require.Empty(t, dg.packet.PointsPayload, "落盘后内存不保留 payload 字节")
	}

	assert.Equal(t, 1, spiller.putCount())

	outcomes := sampler.TailSamplingOutcomes(expired)
	require.Len(t, outcomes, 1)
	for _, outcome := range outcomes {
		require.Equal(t, DerivedMetricDecisionKept, outcome.Decision)
		require.NotNil(t, outcome.Packet)
		// Kept packets must have the full payload read back (needed for sending).
		assert.Equal(t, 4096, len(outcome.Packet.PointsPayload))
	}

	assert.Equal(t, 1, spiller.getCount())
	assert.Equal(t, 1, spiller.deleteCount(), "决策完成后磁盘数据应释放")
}

func TestSamplerSpillLargePacketDroppedNoHydrate(t *testing.T) {
	const token = "tkn_spill_dropped"

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1024)

	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "drop-all", Type: PipelineTypeSampling, Rate: 0.000001},
			},
		},
	}))

	packet := &DataPacket{
		GroupIdHash:          1,
		RawGroupId:           "trace-spill-drop",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           100,
		PointsPayload:        []byte(strings.Repeat("y", 4096)),
		MaxSpanDurationUs:    1000,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(packet)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	outcomes := sampler.TailSamplingOutcomes(expired)
	for _, outcome := range outcomes {
		require.Equal(t, DerivedMetricDecisionDropped, outcome.Decision)
		assert.Nil(t, outcome.Packet)
	}

	// Probabilistic (match-all) decisions need no payload: dropped packets
	// should incur zero disk reads.
	assert.Equal(t, 0, spiller.getCount())
	assert.Equal(t, 1, spiller.deleteCount())
}

func TestSamplerSpillNeedsWalkHydrates(t *testing.T) {
	const token = "tkn_spill_walk"

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1024)

	// Uncompilable condition (custom field): the decision must read back the
	// payload and decompress to walk.
	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{
					Name:      "keep-resource",
					Type:      PipelineTypeCondition,
					Condition: `{ resource = "/api/spill" }`,
					Action:    PipelineActionKeep,
				},
			},
		},
	}))

	kvs := point.NewKVs(map[string]any{
		"resource":  "/api/spill",
		"pad_field": strings.Repeat("p", 2048),
		"trace_id":  "trace-walk-spill",
		"span_id":   "s1",
	})
	payload := point.AppendPBPointToPBPointsPayload(nil, point.NewPoint("span", kvs).PBPoint())
	require.Greater(t, len(payload), 1024, "大字段值应使 payload 超过落盘阈值")

	packet := &DataPacket{
		GroupIdHash:          1,
		RawGroupId:           "trace-walk-spill",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           1,
		PointsPayload:        payload,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(packet)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	outcomes := sampler.TailSamplingOutcomes(expired)
	for _, outcome := range outcomes {
		require.Equal(t, DerivedMetricDecisionKept, outcome.Decision, "walk 应命中 resource 条件")
	}

	assert.Equal(t, 1, spiller.getCount(), "不可编译 pipeline 决策必须读盘 hydrate")
	assert.Equal(t, 1, spiller.deleteCount())
}

func TestSamplerSpillMergeAcrossDataKits(t *testing.T) {
	const token = "tkn_spill_merge"

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1024)

	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
	}))

	first := &DataPacket{
		GroupIdHash:          7,
		RawGroupId:           "trace-merge",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           50,
		PointsPayload:        []byte(strings.Repeat("a", 4096)),
		MaxSpanDurationUs:    1000,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(first)

	second := &DataPacket{
		GroupIdHash:          7,
		RawGroupId:           "trace-merge",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           50,
		PointsPayload:        []byte(strings.Repeat("b", 4096)),
		MaxSpanDurationUs:    1000,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(second)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	for _, dg := range expired {
		require.NotEmpty(t, dg.spillKey, "合并后大包应重新落盘")
		require.Empty(t, dg.packet.PointsPayload)
		assert.Equal(t, int32(100), dg.packet.PointCount)
	}

	outcomes := sampler.TailSamplingOutcomes(expired)
	for _, outcome := range outcomes {
		require.Equal(t, DerivedMetricDecisionKept, outcome.Decision)
		require.NotNil(t, outcome.Packet)
		// Merge semantics: full payload (sum of both parts).
		assert.Equal(t, 8192, len(outcome.Packet.PointsPayload))
	}
}

func TestSamplerSmallPacketStaysInMemory(t *testing.T) {
	const token = "tkn_spill_small"

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1024)

	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
	}))

	packet := &DataPacket{
		GroupIdHash:          2,
		RawGroupId:           "trace-small",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           1,
		PointsPayload:        []byte("small"),
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(packet)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)
	for _, dg := range expired {
		assert.Empty(t, dg.spillKey, "小包应留在内存")
		assert.Equal(t, []byte("small"), dg.packet.PointsPayload)
	}

	assert.Equal(t, 0, spiller.putCount())
}

func TestSamplerSpillMergeHydrateFailureSkipsMerge(t *testing.T) {
	const token = "tkn_spill_hydrate_fail"

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(1, time.Second)
	sampler.SetPayloadSpiller(spiller, 1024)

	require.NoError(t, sampler.UpdateConfig(token, &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL: time.Second,
			Pipelines: []*SamplingPipeline{
				{Name: "keep-all", Type: PipelineTypeSampling, Rate: 1},
			},
		},
	}))

	first := &DataPacket{
		GroupIdHash:          9,
		RawGroupId:           "trace-hydrate-fail",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           50,
		PointsPayload:        []byte(strings.Repeat("a", 4096)),
		MaxSpanDurationUs:    1000,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(first)

	// After a disk read failure, another batch of the same trace must be
	// skipped at merge time; otherwise PointCount accumulates while the payload
	// holds only the new packet (data misalignment).
	spiller.failGet = true
	second := &DataPacket{
		GroupIdHash:          9,
		RawGroupId:           "trace-hydrate-fail",
		Token:                token,
		DataType:             point.STracing,
		GroupKey:             "trace_id",
		PointCount:           50,
		PointsPayload:        []byte(strings.Repeat("b", 4096)),
		MaxSpanDurationUs:    1000,
		MaxPointTimeUnixNano: time.Now().UnixNano(),
	}
	sampler.Ingest(second)

	expired := sampler.AdvanceTime()
	require.Len(t, expired, 1)

	for _, dg := range expired {
		require.NotEmpty(t, dg.spillKey)
		require.Empty(t, dg.packet.PointsPayload)
		// Merge was skipped: PointCount keeps the first packet value, not
		// accumulated with the second.
		assert.Equal(t, int32(50), dg.packet.PointCount)
	}

	// Decision: trusted predicates → fast-path kept; the payload hydrate still
	// fails → stays empty.
	outcomes := sampler.TailSamplingOutcomes(expired)
	for _, outcome := range outcomes {
		require.Equal(t, DerivedMetricDecisionKept, outcome.Decision)
		require.NotNil(t, outcome.Packet)
		assert.Empty(t, outcome.Packet.PointsPayload, "hydrate 失败时 kept 包 payload 应为空")
	}
}
