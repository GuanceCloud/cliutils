// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package runner

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSyntheticsRetryCorrelation(t *testing.T) {
	for _, testID := range []string{"", "platform-task"} {
		t.Run("test_id="+testID, func(t *testing.T) {
			var engines []*fakeEngine
			result := RunScript(context.Background(), scriptForTest(), Options{
				SyntheticsTestID: testID, RetryCount: 1, TimeoutMS: 1000,
				EngineFactory: func(context.Context, EngineOptions) (Engine, error) {
					engine := &fakeEngine{}
					if len(engines) == 0 {
						engine.title = "Wrong"
					}
					engines = append(engines, engine)
					return engine, nil
				},
			})
			require.True(t, result.Success)
			require.Len(t, engines, 2)
			require.Len(t, result.RetryRecords, 2)
			for i, engine := range engines {
				correlation := engine.config.Synthetics
				require.Equal(t, testID, correlation.TestID)
				require.NotEmpty(t, correlation.ResultID)
				require.Equal(t, correlation.ResultID, result.RetryRecords[i].BrowserRunID)
				initScript, err := correlation.InitScript()
				require.NoError(t, err)
				require.Equal(t, testID == "", initScript == "")
			}
			require.Equal(t, engines[1].config.Synthetics.ResultID, result.RunID)
			if testID != "" {
				require.NotEqual(t, engines[0].config.Synthetics.ResultID, engines[1].config.Synthetics.ResultID)
			}
		})
	}
}
