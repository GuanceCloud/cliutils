// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateConfigFromTOML(t *testing.T) {
	sampler, err := NewSampler(nil)
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	tomlConfig := []byte(`
version = 101

[trace]
data_ttl = "20s"
max_hold_duration = "45s"
group_key = "trace_id"

[[trace.sampling_pipeline]]
name = "keep-error"
type = "condition"
condition = '{ status = "error" }'
action = "keep"

[[trace.sampling_pipeline]]
name = "sample-10pct"
type = "probabilistic"
condition = '{ 1 = 1 }'
rate = 0.1
`)

	err = sampler.UpdateConfigFromTOML("tkn_toml_test", tomlConfig)
	require.NoError(t, err)

	sampler.configMu.RLock()
	cfg := sampler.configMap["tkn_toml_test"]
	sampler.configMu.RUnlock()

	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Tracing)
	assert.Equal(t, int64(101), cfg.Version)
	assert.Equal(t, 20*time.Second, cfg.Tracing.DataTTL)
	assert.Equal(t, 45*time.Second, cfg.Tracing.MaxHoldDuration)
	require.Len(t, cfg.Tracing.Pipelines, 2)
	assert.Equal(t, "keep-error", cfg.Tracing.Pipelines[0].Name)
	assert.Equal(t, "sample-10pct", cfg.Tracing.Pipelines[1].Name)
}

func TestUpdateConfigFromJSON(t *testing.T) {
	sampler, err := NewSampler(nil)
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	jsonConfig := []byte(`{
		"version": 202,
		"trace": {
			"data_ttl": 15000000000,
			"max_hold_duration": 30000000000,
			"group_key": "trace_id",
			"pipelines": [
				{
					"name": "keep-http-5xx",
					"type": "condition",
					"condition": "{ http_status_code = \"500\" }",
					"action": "keep"
				}
			]
		}
	}`)

	err = sampler.UpdateConfigFromJSON("tkn_json_test", jsonConfig)
	require.NoError(t, err)

	sampler.configMu.RLock()
	cfg := sampler.configMap["tkn_json_test"]
	sampler.configMu.RUnlock()

	require.NotNil(t, cfg)
	require.NotNil(t, cfg.Tracing)
	assert.Equal(t, int64(202), cfg.Version)
	require.Len(t, cfg.Tracing.Pipelines, 1)
	assert.Equal(t, "keep-http-5xx", cfg.Tracing.Pipelines[0].Name)
}

func TestBuiltinMetricsHook(t *testing.T) {
	hook := NewBuiltinMetricsHook()
	cfg := &SamplerConfig{
		DefaultTTL:      1 * time.Second,
		MaxHoldDuration: 2 * time.Second,
		Hook:            hook,
	}

	sampler, err := NewSampler(cfg)
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	rules := &TailSamplingConfigs{
		Tracing: &TraceTailSampling{
			DataTTL:         1 * time.Second,
			MaxHoldDuration: 2 * time.Second,
			Pipelines: []*SamplingPipeline{
				{
					Name:      "keep-error",
					Type:      PipelineTypeCondition,
					Condition: "{ status = 'error' }",
					Action:    PipelineActionKeep,
				},
				{
					Name:   "drop-others",
					Type:   PipelineTypeCondition,
					Action: PipelineActionDrop,
				},
			},
		},
	}
	require.NoError(t, sampler.UpdateConfig("tkn_hook", rules))

	payload, _ := CompressPayload([]byte("span-data"))
	// Ingest error packet
	err = sampler.Ingest(&DataPacket{
		GroupIdHash:        3001,
		RawGroupId:         "trace-error",
		Token:              "tkn_hook",
		DataType:           "tracing",
		PointCount:         3,
		HasError:           true,
		PredError:          true,
		PointsPayload:      payload,
		PayloadCompression: PayloadCompressionZstd,
	})
	require.NoError(t, err)

	// Ingest normal packet
	err = sampler.Ingest(&DataPacket{
		GroupIdHash:        3002,
		RawGroupId:         "trace-normal",
		Token:              "tkn_hook",
		DataType:           "tracing",
		PointCount:         2,
		PointsPayload:      payload,
		PayloadCompression: PayloadCompressionZstd,
	})
	require.NoError(t, err)

	assert.Equal(t, int64(2), hook.TotalIngestedPackets.Load())
	assert.Equal(t, int64(5), hook.TotalIngestedSpans.Load())

	// Advance time wheel
	now := time.Now()
	for range 3 {
		now = now.Add(time.Second)
		_, _, _ = sampler.AdvanceTime(now)
	}

	assert.Equal(t, int64(1), hook.TotalKeptTraces.Load())
	assert.Equal(t, int64(1), hook.TotalErrorTraces.Load())
	assert.Equal(t, int64(1), hook.TotalDroppedTraces.Load())
}

func TestRunTimeWheelLoop(t *testing.T) {
	sampler, err := NewSampler(&SamplerConfig{
		DefaultTTL:      1 * time.Second,
		MaxHoldDuration: 2 * time.Second,
	})
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	var mu sync.Mutex
	totalKept := 0
	stop := RunTimeWheelLoop(sampler, 200*time.Millisecond, func(kept []*DataPacket, dropped int) {
		mu.Lock()
		defer mu.Unlock()
		totalKept += len(kept)
	})
	defer stop()

	payload, _ := CompressPayload([]byte("payload"))
	err = sampler.Ingest(&DataPacket{
		GroupIdHash:        5001,
		RawGroupId:         "trace-loop",
		Token:              "tkn_loop",
		DataType:           "tracing",
		PointsPayload:      payload,
		PayloadCompression: PayloadCompressionZstd,
	})
	require.NoError(t, err)

	// Wait for loop to tick past 1s TTL
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return totalKept > 0
	}, 3*time.Second, 100*time.Millisecond)
}
