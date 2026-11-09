// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package aggregate_v2

import (
	"strconv"
	"strings"
	"time"

	fp "github.com/GuanceCloud/cliutils/filter"
)

type spanPredicateFunc func(m *TraceMeta) bool

type compiledDecisionPlan struct {
	pipelines  []*SamplingPipeline
	predicates []spanPredicateFunc
	fast       bool
}

// compileDecisionPlan pre-compiles pipelines for zero-allocation fast evaluation.
func compileDecisionPlan(pipelines []*SamplingPipeline) *compiledDecisionPlan {
	if len(pipelines) == 0 {
		return &compiledDecisionPlan{}
	}

	plan := &compiledDecisionPlan{
		pipelines:  pipelines,
		predicates: make([]spanPredicateFunc, len(pipelines)),
		fast:       true,
	}

	for i, p := range pipelines {
		pred, ok := compileSinglePipeline(p)
		if !ok {
			plan.fast = false
			break
		}
		plan.predicates[i] = pred
	}

	return plan
}

func compileSinglePipeline(p *SamplingPipeline) (spanPredicateFunc, bool) {
	if p == nil {
		return nil, false
	}

	// Probabilistic pipeline with match-all condition
	if p.Type == PipelineTypeSampling {
		rateThreshold := uint64(p.Rate * float64(defaultSampleRange))
		probFn := func(m *TraceMeta) bool {
			return (m.GroupIDHash % defaultSampleRange) < rateThreshold
		}

		if p.Condition == "" || isMatchAllCondition(p.Condition) {
			return probFn, true
		}
		// If condition is specified with sampling, both must match
		condFn, ok := compileConditionPredicate(p.Condition)
		if !ok {
			return nil, false
		}
		return func(m *TraceMeta) bool {
			return condFn(m) && probFn(m)
		}, true
	}

	// Condition pipeline
	if p.Condition == "" || isMatchAllCondition(p.Condition) {
		return func(_ *TraceMeta) bool { return true }, true
	}

	return compileConditionPredicate(p.Condition)
}

func isMatchAllCondition(cond string) bool {
	clean := strings.TrimSpace(cond)
	clean = strings.TrimPrefix(clean, "{")
	clean = strings.TrimSuffix(clean, "}")
	clean = strings.TrimSpace(clean)
	return clean == "" || clean == "1 = 1" || clean == "1=1" || clean == "true"
}

func compileConditionPredicate(cond string) (spanPredicateFunc, bool) {
	clean := strings.TrimSpace(cond)
	clean = strings.TrimPrefix(clean, "{")
	clean = strings.TrimSuffix(clean, "}")
	clean = strings.TrimSpace(clean)

	// status = 'error'
	if strings.Contains(clean, "status") && strings.Contains(clean, "error") {
		return func(m *TraceMeta) bool {
			return m.HasError()
		}, true
	}

	// trace_keep = true
	if strings.Contains(clean, "trace_keep") {
		return func(m *TraceMeta) bool {
			return m.HasTraceKeep()
		}, true
	}

	// http_status_code
	if strings.Contains(clean, "http_status_code") {
		return func(m *TraceMeta) bool {
			return m.HasHttpError()
		}, true
	}

	// body_code != '0' / biz error
	if strings.Contains(clean, "body_code") {
		return func(m *TraceMeta) bool {
			return m.HasBizError()
		}, true
	}

	// duration > X (support ns, us, ms, s)
	if strings.Contains(clean, "duration") {
		return compileDurationPredicate(clean)
	}

	return nil, false
}

func compileDurationPredicate(cond string) (spanPredicateFunc, bool) {
	parts := strings.Fields(cond)
	for i, part := range parts {
		if (part == ">" || part == ">=") && i+1 < len(parts) {
			valStr := strings.Trim(parts[i+1], `"'`)
			thresholdUs := parseDurationToUs(valStr)
			if thresholdUs > 0 {
				return func(m *TraceMeta) bool {
					return m.MaxSpanDurationUs >= thresholdUs
				}, true
			}
		}
	}
	return nil, false
}

func parseDurationToUs(valStr string) int64 {
	// Try parsing as duration string like "500ms", "1s"
	if d, err := time.ParseDuration(valStr); err == nil {
		return d.Microseconds()
	}
	// Try parsing as integer microseconds or nanoseconds
	if num, err := strconv.ParseInt(valStr, 10, 64); err == nil {
		if num >= 1e6 { // Nanoseconds (>= 1ms)
			return num / 1000
		}
		return num // Assume microseconds
	}
	return 0
}

// DecisionResult represents the outcome of evaluating a trace against sampling rules.
type DecisionResult struct {
	Matched     bool
	Action      PipelineAction
	RuleName    string
	RuleIndex   int
	SpilledDrop bool
}

// Evaluate evaluates a TraceMeta against the compiled decision plan in zero allocations.
func (p *compiledDecisionPlan) Evaluate(m *TraceMeta) DecisionResult {
	if p == nil || len(p.pipelines) == 0 || m == nil {
		// Default when no pipelines configured: keep
		return DecisionResult{Matched: true, Action: PipelineActionKeep, RuleIndex: -1}
	}

	// Fast-path: all rules coverable by TraceMeta predicates
	if p.fast {
		for i, pred := range p.predicates {
			if pred != nil && pred(m) {
				pipeline := p.pipelines[i]
				action := pipeline.Action
				if pipeline.Type == PipelineTypeSampling {
					// Probabilistic pipeline with match-all or matching condition keeps on match
					action = PipelineActionKeep
				}
				return DecisionResult{
					Matched:   true,
					Action:    action,
					RuleName:  pipeline.Name,
					RuleIndex: i,
				}
			}
		}
		// No rule matched: default drop
		return DecisionResult{Matched: false, Action: PipelineActionDrop, RuleIndex: -1}
	}

	// Fallback path: evaluate AST conditions if slow-path required
	kvs := metaKVs{m: m}
	for i, pipeline := range p.pipelines {
		if pipeline == nil {
			continue
		}
		if pipeline.conds == nil || pipeline.conds.Eval(kvs) >= 0 {
			action := pipeline.Action
			if pipeline.Type == PipelineTypeSampling {
				rateThreshold := uint64(pipeline.Rate * float64(defaultSampleRange))
				if (m.GroupIDHash % defaultSampleRange) < rateThreshold {
					action = PipelineActionKeep
				} else {
					action = PipelineActionDrop
				}
			}
			return DecisionResult{
				Matched:   true,
				Action:    action,
				RuleName:  pipeline.Name,
				RuleIndex: i,
			}
		}
	}

	return DecisionResult{Matched: false, Action: PipelineActionDrop, RuleIndex: -1}
}

// metaKVs implements fp.KVs for AST condition fallback evaluation.
type metaKVs struct {
	m *TraceMeta
}

var _ fp.KVs = metaKVs{}

func (k metaKVs) Get(key string) (any, bool) {
	if k.m == nil {
		return nil, false
	}
	switch key {
	case "status":
		if k.m.HasError() {
			return "error", true
		}
		return "ok", true
	case "trace_keep":
		return k.m.HasTraceKeep(), true
	case "duration":
		return k.m.MaxSpanDurationUs * 1000, true // Nanoseconds for filter
	case "trace_id":
		return k.m.RawGroupID, true
	default:
		return nil, false
	}
}
