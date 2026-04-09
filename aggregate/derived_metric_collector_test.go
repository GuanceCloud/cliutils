package aggregate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDerivedMetricCollector_Flush(t *testing.T) {
	collector := NewDerivedMetricCollector(15 * time.Second)
	baseTime := time.Unix(1700000001, 0)
	expectedTS := AlignNextWallTime(baseTime, 15*time.Second) * int64(time.Second)

	collector.Add([]DerivedMetricRecord{
		{
			Token:      "token-a",
			DataType:   "tracing",
			MetricName: "trace_total_count",
			Stage:      DerivedMetricStageIngest,
			Tags: map[string]string{
				"service": "checkout",
			},
			Value: 1,
			Time:  baseTime,
		},
		{
			Token:      "token-a",
			DataType:   "tracing",
			MetricName: "trace_total_count",
			Stage:      DerivedMetricStageIngest,
			Tags: map[string]string{
				"service": "checkout",
			},
			Value: 2,
			Time:  baseTime.Add(2 * time.Second),
		},
		{
			Token:      "token-b",
			DataType:   "logging",
			MetricName: "logging_total_count",
			Stage:      DerivedMetricStageIngest,
			Value:      1,
			Time:       baseTime.Add(4 * time.Second),
		},
	})

	res := collector.Flush(baseTime.Add(15 * time.Second))
	require.Len(t, res, 2)

	grouped := map[string]*DerivedMetricPoints{}
	for _, item := range res {
		grouped[item.Token] = item
	}

	require.Contains(t, grouped, "token-a")
	require.Contains(t, grouped, "token-b")

	require.Len(t, grouped["token-a"].PTS, 1)
	ptA := grouped["token-a"].PTS[0]
	valueA, ok := ptA.GetF("trace_total_count")
	require.True(t, ok)
	assert.Equal(t, 3.0, valueA)
	assert.Equal(t, TailSamplingDerivedMeasurement, ptA.Name())
	assert.Equal(t, "ingest", ptA.GetTag("stage"))
	assert.Equal(t, "checkout", ptA.GetTag("service"))
	assert.Equal(t, expectedTS, ptA.Time().UnixNano())

	require.Len(t, grouped["token-b"].PTS, 1)
	ptB := grouped["token-b"].PTS[0]
	valueB, ok := ptB.GetF("logging_total_count")
	require.True(t, ok)
	assert.Equal(t, 1.0, valueB)
	assert.Equal(t, TailSamplingDerivedMeasurement, ptB.Name())
	assert.Equal(t, "ingest", ptB.GetTag("stage"))
	assert.Equal(t, expectedTS, ptB.Time().UnixNano())
}

func TestDerivedMetricCollector_FlushHistogram(t *testing.T) {
	collector := NewDerivedMetricCollector(30 * time.Second)
	baseTime := time.Unix(1700000001, 0)
	expectedTS := AlignNextWallTime(baseTime, 30*time.Second) * int64(time.Second)

	collector.Add([]DerivedMetricRecord{
		{
			Token:      "token-a",
			MetricName: "trace_duration",
			Kind:       DerivedMetricKindHistogram,
			Stage:      DerivedMetricStagePreDecision,
			Tags: map[string]string{
				"data_type": "tracing",
			},
			Value:   12,
			Buckets: []float64{1, 10, 20},
			Time:    baseTime,
		},
		{
			Token:      "token-a",
			MetricName: "trace_duration",
			Kind:       DerivedMetricKindHistogram,
			Stage:      DerivedMetricStagePreDecision,
			Tags: map[string]string{
				"data_type": "tracing",
			},
			Value:   25,
			Buckets: []float64{1, 10, 20},
			Time:    baseTime.Add(2 * time.Second),
		},
	})

	res := collector.Flush(baseTime.Add(30 * time.Second))
	require.Len(t, res, 1)
	require.Len(t, res[0].PTS, 6) // 4 buckets + sum + count

	var (
		foundSum   bool
		foundCount bool
		foundLE20  bool
	)
	for _, pt := range res[0].PTS {
		assert.Equal(t, expectedTS, pt.Time().UnixNano())
		if v, ok := pt.GetF("trace_duration_sum"); ok {
			foundSum = true
			assert.Equal(t, 37.0, v)
		}
		if v, ok := pt.GetF("trace_duration_count"); ok {
			foundCount = true
			assert.Equal(t, 2.0, v)
		}
		if v, ok := pt.GetF("trace_duration_bucket"); ok && pt.GetTag("le") == "20" {
			foundLE20 = true
			assert.Equal(t, 1.0, v)
		}
	}

	assert.True(t, foundSum)
	assert.True(t, foundCount)
	assert.True(t, foundLE20)
}
