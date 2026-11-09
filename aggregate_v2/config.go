// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"fmt"
	"strings"
	"time"

	fp "github.com/GuanceCloud/cliutils/filter"
)

// Pipeline types and actions.
const (
	PipelineTypeCondition = "condition"
	PipelineTypeSampling  = "probabilistic"

	PipelineActionKeep = "keep"
	PipelineActionDrop = "drop"

	defaultSampleRange = 10000
)

// PipelineType represents the type of a sampling pipeline.
type PipelineType = string

// PipelineAction represents the action of a sampling pipeline (keep or drop).
type PipelineAction = string

// SamplingPipeline defines an individual sampling rule.
type SamplingPipeline struct {
	Name      string         `toml:"name" json:"name"`
	Type      PipelineType   `toml:"type" json:"type"`
	Condition string         `toml:"condition,omitempty" json:"condition,omitempty"`
	Action    PipelineAction `toml:"action,omitempty" json:"action,omitempty"`
	Rate      float64        `toml:"rate,omitempty" json:"rate,omitempty"`
	HashKeys  []string       `toml:"hash_keys" json:"hash_keys"`

	conds fp.WhereConditions
}

// Apply compiles the pipeline's condition expression into an AST using cliutils/filter.
func (sp *SamplingPipeline) Apply() error {
	if sp == nil || sp.Condition == "" {
		if sp != nil {
			sp.conds = nil
		}
		return nil
	}

	ast, err := fp.GetConds(sp.Condition)
	if err != nil {
		return fmt.Errorf("compile condition %q failed: %w", sp.Condition, err)
	}
	sp.conds = ast
	return nil
}

// TraceTailSampling holds trace-level tail sampling configuration.
type TraceTailSampling struct {
	DataTTL         time.Duration       `toml:"data_ttl" json:"data_ttl"`
	MaxHoldDuration time.Duration       `toml:"max_hold_duration" json:"max_hold_duration"`
	Pipelines       []*SamplingPipeline `toml:"sampling_pipeline" json:"pipelines"`
	Version         int64               `toml:"version" json:"version"`
	GroupKey        string              `toml:"group_key" json:"group_key"` // Defaults to "trace_id"

	compiledPlan *compiledDecisionPlan
}

// LoggingGroupDimension configures tail sampling for a specific logging group dimension.
type LoggingGroupDimension struct {
	GroupKey  string              `toml:"group_key" json:"group_key"`
	Pipelines []*SamplingPipeline `toml:"pipelines" json:"pipelines"`

	compiledPlan *compiledDecisionPlan
}

// LoggingTailSampling holds logging tail sampling configuration.
type LoggingTailSampling struct {
	DataTTL         time.Duration            `toml:"data_ttl" json:"data_ttl"`
	MaxHoldDuration time.Duration            `toml:"max_hold_duration" json:"max_hold_duration"`
	Version         int64                    `toml:"version" json:"version"`
	GroupDimensions []*LoggingGroupDimension `toml:"group_dimensions" json:"group_dimensions"`
}

// RUMGroupDimension configures tail sampling for a specific RUM group dimension.
type RUMGroupDimension struct {
	GroupKey  string              `toml:"group_key" json:"group_key"`
	Pipelines []*SamplingPipeline `toml:"pipelines" json:"pipelines"`

	compiledPlan *compiledDecisionPlan
}

// RUMTailSampling holds RUM tail sampling configuration.
type RUMTailSampling struct {
	DataTTL         time.Duration        `toml:"data_ttl" json:"data_ttl"`
	MaxHoldDuration time.Duration        `toml:"max_hold_duration" json:"max_hold_duration"`
	Version         int64                `toml:"version" json:"version"`
	GroupDimensions []*RUMGroupDimension `toml:"group_dimensions" json:"group_dimensions"`
}

// TailSamplingConfigs is the root configuration structure for tail sampling.
type TailSamplingConfigs struct {
	Tracing *TraceTailSampling   `toml:"trace" json:"trace"`
	Logging *LoggingTailSampling `toml:"logging" json:"logging"`
	RUM     *RUMTailSampling     `toml:"rum" json:"rum"`
	Version int64                `toml:"version" json:"version"`
}

// Init validates and compiles all rule pipelines in the configuration.
func (t *TailSamplingConfigs) Init() error {
	if t == nil {
		return nil
	}

	if t.Tracing != nil {
		if t.Tracing.DataTTL <= 0 {
			t.Tracing.DataTTL = 15 * time.Second
		}
		if t.Tracing.MaxHoldDuration <= 0 {
			t.Tracing.MaxHoldDuration = 2 * t.Tracing.DataTTL
			if t.Tracing.MaxHoldDuration < 30*time.Second {
				t.Tracing.MaxHoldDuration = 30 * time.Second
			}
		}
		if t.Tracing.GroupKey == "" {
			t.Tracing.GroupKey = "trace_id"
		}
		for _, p := range t.Tracing.Pipelines {
			if err := p.Apply(); err != nil {
				return fmt.Errorf("trace pipeline %q init failed: %w", p.Name, err)
			}
		}
		t.Tracing.compiledPlan = compileDecisionPlan(t.Tracing.Pipelines)
	}

	if t.Logging != nil {
		if t.Logging.DataTTL <= 0 {
			t.Logging.DataTTL = 15 * time.Second
		}
		if t.Logging.MaxHoldDuration <= 0 {
			t.Logging.MaxHoldDuration = 30 * time.Second
		}
		for _, gd := range t.Logging.GroupDimensions {
			if gd == nil {
				continue
			}
			for _, p := range gd.Pipelines {
				if err := p.Apply(); err != nil {
					return fmt.Errorf("logging pipeline %q init failed: %w", p.Name, err)
				}
			}
			gd.compiledPlan = compileDecisionPlan(gd.Pipelines)
		}
	}

	if t.RUM != nil {
		if t.RUM.DataTTL <= 0 {
			t.RUM.DataTTL = 15 * time.Second
		}
		if t.RUM.MaxHoldDuration <= 0 {
			t.RUM.MaxHoldDuration = 30 * time.Second
		}
		for _, gd := range t.RUM.GroupDimensions {
			if gd == nil {
				continue
			}
			for _, p := range gd.Pipelines {
				if err := p.Apply(); err != nil {
					return fmt.Errorf("rum pipeline %q init failed: %w", p.Name, err)
				}
			}
			gd.compiledPlan = compileDecisionPlan(gd.Pipelines)
		}
	}

	return nil
}

// ToString returns a human-readable summary of the configuration.
func (t *TailSamplingConfigs) ToString() string {
	if t == nil {
		return "<nil>"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "TailSamplingConfigs{version=%d", t.Version)
	if t.Tracing != nil {
		fmt.Fprintf(&b, ", trace={ttl=%s, max_hold=%s, pipelines=%d}",
			t.Tracing.DataTTL, t.Tracing.MaxHoldDuration, len(t.Tracing.Pipelines))
	}
	if t.Logging != nil {
		fmt.Fprintf(&b, ", logging={ttl=%s, max_hold=%s, groups=%d}",
			t.Logging.DataTTL, t.Logging.MaxHoldDuration, len(t.Logging.GroupDimensions))
	}
	if t.RUM != nil {
		fmt.Fprintf(&b, ", rum={ttl=%s, max_hold=%s, groups=%d}",
			t.RUM.DataTTL, t.RUM.MaxHoldDuration, len(t.RUM.GroupDimensions))
	}
	b.WriteString("}")
	return b.String()
}
