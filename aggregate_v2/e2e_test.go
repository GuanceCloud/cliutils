// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEndToEndHTTPSamplingPipeline(t *testing.T) {
	spiller := newMemorySpiller()
	cfg := &SamplerConfig{
		ShardCount:             8,
		DefaultTTL:             2 * time.Second,
		MaxHoldDuration:        4 * time.Second,
		MaxResidentMemoryBytes: 50 << 20, // 50MB
		SpillThresholdBytes:    32 << 10, // 32KB spill threshold
		Spiller:                spiller,
	}

	sampler, err := NewSampler(cfg)
	require.NoError(t, err)
	defer func() { _ = sampler.Close() }()

	// Configure rules for token "tkn_e2e"
	rules := &TailSamplingConfigs{
		Tracing: &TraceTailSampling{
			DataTTL:         2 * time.Second,
			MaxHoldDuration: 4 * time.Second,
			Pipelines: []*SamplingPipeline{
				{
					Name:      "keep-error-traces",
					Type:      PipelineTypeCondition,
					Condition: "{ status = 'error' }",
					Action:    PipelineActionKeep,
				},
				{
					Name:      "keep-slow-traces",
					Type:      PipelineTypeCondition,
					Condition: "{ duration >= 500000000 }",
					Action:    PipelineActionKeep,
				},
				{
					Name:      "drop-normal-traces",
					Type:      PipelineTypeCondition,
					Condition: "",
					Action:    PipelineActionDrop,
				},
			},
		},
	}
	require.NoError(t, sampler.UpdateConfig("tkn_e2e", rules))

	// Start real HTTP server wrapping the aggregate_v2 Sampler
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := sampler.IngestBytes(body); err != nil {
			if errors.Is(err, ErrBudgetExceeded) {
				http.Error(w, "budget exceeded", http.StatusTooManyRequests)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := ts.Client()

	// 1. Send HTTP request 1: Trace-Error span 1 & Trace-Normal span 1
	pError1, _ := CompressPayload([]byte("span-err-01; "))
	pNormal1, _ := CompressPayload([]byte("span-norm-01; "))
	batch1 := &DataBatch{
		CollectorId: "agent-node-01",
		Packets: []*DataPacket{
			{
				GroupIdHash:        2001,
				RawGroupId:         "trace-error",
				Token:              "tkn_e2e",
				DataType:           "tracing",
				HasError:           true,
				PredError:          true,
				PointCount:         1,
				PointsPayload:      pError1,
				PayloadCompression: PayloadCompressionZstd,
			},
			{
				GroupIdHash:        2002,
				RawGroupId:         "trace-normal",
				Token:              "tkn_e2e",
				DataType:           "tracing",
				PointCount:         1,
				PointsPayload:      pNormal1,
				PayloadCompression: PayloadCompressionZstd,
			},
		},
	}
	sendBatchHTTP(t, client, ts.URL, batch1)

	// 2. Send HTTP request 2: Trace-Error span 2 & Trace-Slow span 1
	pError2, _ := CompressPayload([]byte("span-err-02; "))
	pSlow1, _ := CompressPayload([]byte("span-slow-01; "))
	batch2 := &DataBatch{
		CollectorId: "agent-node-02",
		Packets: []*DataPacket{
			{
				GroupIdHash:        2001,
				RawGroupId:         "trace-error",
				Token:              "tkn_e2e",
				DataType:           "tracing",
				PointCount:         1,
				PointsPayload:      pError2,
				PayloadCompression: PayloadCompressionZstd,
			},
			{
				GroupIdHash:        2003,
				RawGroupId:         "trace-slow",
				Token:              "tkn_e2e",
				DataType:           "tracing",
				MaxSpanDurationUs:  850000, // 850ms
				PointCount:         1,
				PointsPayload:      pSlow1,
				PayloadCompression: PayloadCompressionZstd,
			},
		},
	}
	sendBatchHTTP(t, client, ts.URL, batch2)

	// 3. Send HTTP request 3: Jumbo Trace (exceeds 32KB spill threshold)
	jumboRaw := strings.Repeat("span-jumbo-payload-chunk;", 2000) // ~50KB
	pJumbo, err := CompressPayload([]byte(jumboRaw))
	require.NoError(t, err)
	singlePacket := &DataPacket{
		GroupIdHash:        2004,
		RawGroupId:         "trace-jumbo",
		Token:              "tkn_e2e",
		DataType:           "tracing",
		HasError:           true,
		PredError:          true,
		PointCount:         20,
		PointsPayload:      pJumbo,
		PayloadCompression: PayloadCompressionZstd,
	}
	sendSinglePacketHTTP(t, client, ts.URL, singlePacket)

	// Verify stats show ingested packets
	stats := sampler.Stats()
	assert.Equal(t, int64(5), stats.IngestedPackets)
	assert.Equal(t, int64(4), stats.ActiveTraces)

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

	// Assert decisions:
	// trace-normal (2002) dropped -> totalDropped == 1
	// trace-error (2001), trace-slow (2003), trace-jumbo (2004) kept -> len(allKept) == 3
	assert.Equal(t, 1, totalDropped, "trace-normal must be dropped")
	require.Len(t, allKept, 3, "trace-error, trace-slow, and trace-jumbo must be kept")

	keptMap := make(map[string]*DataPacket)
	for _, p := range allKept {
		keptMap[p.RawGroupId] = p
	}

	// Verify Trace-Error: spans concatenated seamlessly
	errTrace, ok := keptMap["trace-error"]
	require.True(t, ok)
	decompErr, err := DecompressPayload(errTrace.PointsPayload)
	require.NoError(t, err)
	assert.Contains(t, string(decompErr), "span-err-01;")
	assert.Contains(t, string(decompErr), "span-err-02;")

	// Verify Trace-Slow
	slowTrace, ok := keptMap["trace-slow"]
	require.True(t, ok)
	decompSlow, err := DecompressPayload(slowTrace.PointsPayload)
	require.NoError(t, err)
	assert.Contains(t, string(decompSlow), "span-slow-01;")

	// Verify Jumbo Trace was properly spilled and then hydrated
	jumboTrace, ok := keptMap["trace-jumbo"]
	require.True(t, ok)
	decompJumbo, err := DecompressPayload(jumboTrace.PointsPayload)
	require.NoError(t, err)
	assert.Equal(t, len(jumboRaw), len(decompJumbo))

	// Verify all active traces cleaned up: zero memory leak
	postStats := sampler.Stats()
	assert.Equal(t, int64(0), postStats.ActiveTraces)
	assert.Equal(t, int64(0), postStats.ResidentPayloadBytes)
	assert.Equal(t, int64(0), postStats.SpilledPayloadBytes)
}

func sendBatchHTTP(t *testing.T, client *http.Client, url string, batch *DataBatch) {
	t.Helper()
	data, err := MarshalDataBatch(batch)
	require.NoError(t, err)

	resp, err := client.Post(url, "application/x-protobuf", bytes.NewReader(data))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func sendSinglePacketHTTP(t *testing.T, client *http.Client, url string, packet *DataPacket) {
	t.Helper()
	data, err := MarshalDataPacket(packet)
	require.NoError(t, err)

	resp, err := client.Post(url, "application/x-protobuf", bytes.NewReader(data))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}
