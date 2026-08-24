package aggregate

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTailSamplingProcessorProcessTickMatchesSerialDecision(t *testing.T) {
	const (
		shardCount = 8
		groupCount = 64
	)

	serial := newTickTestProcessor(t, shardCount)
	parallel := newTickTestProcessor(t, shardCount)
	for groupID := 1; groupID <= groupCount; groupID++ {
		serial.IngestPacket(newTickTestPacket(uint64(groupID)))
		parallel.IngestPacket(newTickTestPacket(uint64(groupID)))
	}

	serialOutcomes := serial.TailSamplingOutcomes(serial.AdvanceTime())
	tick := parallel.ProcessTick()

	assert.Equal(t, shardCount, tick.ShardCount)
	assert.Equal(t, shardCount, tick.ActiveShards)
	assert.Equal(t, groupCount, tick.ExpiredGroups)
	require.Len(t, tick.Outcomes, groupCount)
	assert.Positive(t, tick.AdvanceDuration)
	assert.Positive(t, tick.DecisionDuration)
	assert.Positive(t, tick.MaxShardDecisionDuration)
	assert.GreaterOrEqual(t, tick.TotalDuration, tick.DecisionDuration)

	want := indexTickOutcomes(serialOutcomes)
	got := indexTickOutcomeSlice(tick.Outcomes)
	require.Equal(t, len(want), len(got))
	for traceID, wantOutcome := range want {
		gotOutcome := got[traceID]
		require.NotNil(t, gotOutcome, traceID)
		assert.Equal(t, wantOutcome.Decision, gotOutcome.Decision, traceID)
		assert.Equal(t, wantOutcome.Packet != nil, gotOutcome.Packet != nil, traceID)
		assert.Equal(t, wantOutcome.MatchedRule, gotOutcome.MatchedRule, traceID)
	}
}

func TestTailSamplingProcessorProcessTickRunsShardDecisionsConcurrently(t *testing.T) {
	const shardCount = 2

	sampler := NewGlobalSampler(shardCount, time.Second)
	require.NoError(t, sampler.UpdateConfig(benchmarkTailSamplingToken, tickTestConfig()))
	metric := &blockingTickMetric{
		entered: make(chan struct{}, shardCount),
		release: make(chan struct{}),
	}
	processor := NewTailSamplingProcessor(
		sampler,
		NewDerivedMetricCollector(time.Second),
		TailSamplingBuiltinMetrics{metric},
	)
	processor.IngestPacket(newTickTestPacket(2))
	processor.IngestPacket(newTickTestPacket(3))

	result := make(chan TailSamplingTickResult, 1)
	go func() {
		result <- processor.ProcessTick()
	}()

	for range shardCount {
		select {
		case <-metric.entered:
		case <-time.After(time.Second):
			close(metric.release)
			t.Fatal("shard decisions did not run concurrently")
		}
	}
	close(metric.release)

	select {
	case tick := <-result:
		assert.Equal(t, shardCount, tick.ActiveShards)
		assert.Equal(t, shardCount, tick.ExpiredGroups)
		assert.Equal(t, int64(shardCount), metric.calls.Load())
	case <-time.After(time.Second):
		t.Fatal("parallel tick did not finish")
	}
}

func TestTailSamplingProcessorProcessTickHydratesSpilledPayloadsAcrossShards(t *testing.T) {
	const shardCount = 2

	spiller := newMockSpiller()
	sampler := NewGlobalSampler(shardCount, time.Second)
	sampler.SetPayloadSpiller(spiller, 1)
	require.NoError(t, sampler.UpdateConfig(benchmarkTailSamplingToken, tickTestConfig()))
	processor := NewTailSamplingProcessor(sampler, nil, nil)

	for _, groupID := range []uint64{3, 6} {
		processor.IngestPacket(newTickTestPacket(groupID))
	}
	require.Equal(t, 2, spiller.putCount())

	tick := processor.ProcessTick()
	require.Len(t, tick.Outcomes, 2)
	assert.Equal(t, shardCount, tick.ActiveShards)
	for _, outcome := range tick.Outcomes {
		require.NotNil(t, outcome)
		require.Equal(t, DerivedMetricDecisionKept, outcome.Decision)
		require.NotNil(t, outcome.Packet)
		assert.NotEmpty(t, outcome.Packet.PointsPayload)
	}
	assert.Equal(t, 2, spiller.getCount())
	assert.Equal(t, 2, spiller.deleteCount())
}

func TestTailSamplingProcessorProcessTickNilReceiver(t *testing.T) {
	assert.Zero(t, (*TailSamplingProcessor)(nil).ProcessTick())
	assert.Zero(t, (&TailSamplingProcessor{}).ProcessTick())
}

type blockingTickMetric struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int64
}

func (m *blockingTickMetric) Name() string { return "blocking_tick" }

func (m *blockingTickMetric) OnIngest(*DataPacket) []DerivedMetricRecord { return nil }

func (m *blockingTickMetric) OnPreDecision(*DataPacket) []DerivedMetricRecord {
	m.calls.Add(1)
	m.entered <- struct{}{}
	<-m.release
	return nil
}

func (m *blockingTickMetric) OnDecision(*DataPacket, DerivedMetricDecision) []DerivedMetricRecord {
	return nil
}

func newTickTestProcessor(t *testing.T, shardCount int) *TailSamplingProcessor {
	t.Helper()
	processor := NewDefaultTailSamplingProcessor(shardCount, time.Second)
	require.NoError(t, processor.UpdateConfig(benchmarkTailSamplingToken, tickTestConfig()))
	return processor
}

func tickTestConfig() *TailSamplingConfigs {
	return &TailSamplingConfigs{
		Version: 1,
		Tracing: &TraceTailSampling{
			DataTTL:  time.Second,
			GroupKey: benchmarkTraceGroupKey,
			Pipelines: []*SamplingPipeline{
				{
					Name:      "keep-custom",
					Type:      PipelineTypeCondition,
					Condition: `{ custom_flag = "keep" }`,
					Action:    PipelineActionKeep,
				},
				{
					Name: "sample-rest",
					Type: PipelineTypeSampling,
					Rate: 0.5,
				},
			},
		},
	}
}

func newTickTestPacket(groupID uint64) *DataPacket {
	customFlag := "skip"
	if groupID%3 == 0 {
		customFlag = "keep"
	}
	payload := point.AppendPBPointToPBPointsPayload(nil, &point.PBPoint{
		Name: "span",
		Fields: []*point.Field{
			{Key: "custom_flag", Val: &point.Field_S{S: customFlag}},
		},
	})

	return &DataPacket{
		GroupIdHash:   groupID,
		RawGroupId:    fmt.Sprintf("trace-%d", groupID),
		Token:         benchmarkTailSamplingToken,
		DataType:      point.STracing,
		GroupKey:      benchmarkTraceGroupKey,
		ConfigVersion: 1,
		PointCount:    1,
		PointsPayload: payload,
	}
}

func indexTickOutcomes(outcomes map[uint64]*TailSamplingOutcome) map[string]*TailSamplingOutcome {
	indexed := make(map[string]*TailSamplingOutcome, len(outcomes))
	for _, outcome := range outcomes {
		if outcome != nil && outcome.SourcePacket != nil {
			indexed[outcome.SourcePacket.RawGroupId] = outcome
		}
	}
	return indexed
}

func indexTickOutcomeSlice(outcomes []*TailSamplingOutcome) map[string]*TailSamplingOutcome {
	indexed := make(map[string]*TailSamplingOutcome, len(outcomes))
	for _, outcome := range outcomes {
		if outcome != nil && outcome.SourcePacket != nil {
			indexed[outcome.SourcePacket.RawGroupId] = outcome
		}
	}
	return indexed
}
