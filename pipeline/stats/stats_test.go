// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package stats

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestStats(t *testing.T) {
	stats := Stats{}
	event := &ChangeEvent{
		Name:         "abc.p",
		NS:           "default",
		Category:     point.Metric,
		CompileError: "",
		Time:         time.Now(),
	}
	stats.WriteEvent(event)
	if e := stats.ReadEvent(); len(e) == 1 {
		assert.Equal(t, *event, e[0])
	} else {
		t.Fatal("len(events): ", len(e))
	}

	statsR := ScriptStatsROnly{
		Name:     "abc.p",
		NS:       "default",
		Category: point.Metric,
	}
	stats.WriteScriptStats(statsR.Category, statsR.NS, statsR.Name, 0, 0, 0, 0, nil)
	statsRL := stats.ReadStats()
	if len(statsRL) != 0 {
		t.Fatal("len(stats)", len(statsRL))
	}

	stats.UpdateScriptStatsMeta(statsR.Category, statsR.NS, statsR.Name, "", true, false, "")
	statsRL = stats.ReadStats()
	if len(statsRL) != 1 {
		t.Fatal("len(stats)", len(statsRL))
	}
	stats.UpdateScriptStatsMeta(statsR.Category, statsR.NS, statsR.Name, "", true, false, "")
	statsRL = stats.ReadStats()
	if len(statsRL) != 1 {
		t.Fatal("len(stats)", len(statsRL))
	}
}
