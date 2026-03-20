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
