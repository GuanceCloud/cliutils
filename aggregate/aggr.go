package aggregate

import (
	"fmt"
	"time"

	"github.com/GuanceCloud/cliutils"
	fp "github.com/GuanceCloud/cliutils/filter"
	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/point"
	"go.uber.org/zap/zapcore"
)

type (
	Action string
)

var l = logger.DefaultSLogger("aggregator")

const (
	// actions
	ActionPassThrough = "passthrough"
	ActionDrop        = "drop"
)

// AggregatorConfigure is the top-level aggregator configure on single workspace.
type AggregatorConfigure struct {
	DefaultWindow  time.Duration    `toml:"default_window" json:"default_window"`
	AggregateRules []*AggregateRule `toml:"aggregate_rules" json:"aggregate_rules"`
	DefaultAction  Action           `toml:"action" json:"action"`
	Version        int64            `toml:"version" json:"version"`
}

// AggregateRule configured a specific aggregate rule.
type AggregateRule struct {
	Name string `toml:"name" json:"name"`

	// override default window
	Window time.Duration `toml:"window,omitempty" json:"window,omitempty"`

	Selector   *ruleSelector                      `toml:"select" json:"select"`
	Groupby    []string                           `toml:"group_by" json:"group_by"`
	Aggregates map[string]*aggregateAlgoConfigure `toml:"aggregates" json:"aggregates"`
}

// ruleSelector is the selector to select measurements and fields among points.
type ruleSelector struct {
	Category string `toml:"category" json:"category"`

	Measurements          []string `toml:"measurements" json:"measurements"`
	measurementsWhitelist []*cliutils.WhiteListItem

	Fields          []string `toml:"fields" json:"fields"`
	fieldsWhitelist []*cliutils.WhiteListItem

	Condition string `toml:"conditon" json:"condition"`
	conds     fp.WhereConditions
}

func (a *AggregatorConfigure) Setup() error {
	for _, ar := range a.AggregateRules {
		if err := ar.Selector.Setup(); err != nil {
			return err
		}
	}

	return nil
}

func (a *AggregatorConfigure) SelectPoints(pts []*point.Point) (groups [][]*point.Point) {
	for _, ar := range a.AggregateRules {
		groups = append(groups, ar.SelectPoints(pts))
	}
	return
}

func (rs *ruleSelector) Setup() error {
	switch point.CatString(rs.Category) { // category required
	case point.Metric,
		point.Network,
		point.KeyEvent,
		point.Object,
		point.ObjectChange,
		point.CustomObject,
		point.Logging,
		point.Tracing,
		point.RUM,
		point.Security,
		point.Profiling,
		point.DialTesting:
	default:
		return fmt.Errorf("invalid category: %s", rs.Category)
	}

	if rs.Condition != "" {
		if ast, err := fp.GetConds(rs.Condition); err != nil {
			return err
		} else {
			rs.conds = ast
		}
	}

	if len(rs.Measurements) > 0 {
		for _, m := range rs.Measurements {
			rs.measurementsWhitelist = append(rs.measurementsWhitelist, cliutils.NewWhiteListItem(m))
		}
	}

	if len(rs.Fields) > 0 {
		for _, f := range rs.Fields {
			rs.fieldsWhitelist = append(rs.fieldsWhitelist, cliutils.NewWhiteListItem(f))
		}
	}

	return nil
}

func (ar *AggregateRule) SelectPoints(pts []*point.Point) []*point.Point {
	return ar.Selector.doSelect(ar.Groupby, pts)
}

func (ar *AggregateRule) GroupbyPoints(pts []*point.Point) map[uint64][]*point.Point {
	res := map[uint64][]*point.Point{}
	for _, pt := range pts {
		h := hash(pt, ar.Groupby)
		res[h] = append(res[h], pt)
	}

	return res
}

func (s *ruleSelector) doSelect(groupby []string, pts []*point.Point) (res []*point.Point) {
	ptwrapper := &ptWrap{}

	for _, pt := range pts {
		ptname := pt.Name()

		if len(s.measurementsWhitelist) > 0 {
			if !cliutils.WhiteListMatched(ptname, s.measurementsWhitelist) {
				l.Debugf("skip measurement %q", ptname)
				continue
			}
		}

		if len(s.conds) > 0 {
			ptwrapper.Point = pt
			if x := s.conds.Eval(ptwrapper); x < 0 {
				l.Debugf("condition skip measurement %q", ptname)
				continue // ignore the point
			}
		}

		// fork 1 or more points from pt, each forked points got only 1 non-tag field.
		var forkedPts []*point.Point

		// select specific aggregate fields
		if len(s.fieldsWhitelist) > 0 {
			for _, kv := range pt.KVs() {
				if !cliutils.WhiteListMatched(kv.Key, s.fieldsWhitelist) {
					l.Debugf("skip field %q", kv.Key)
					continue
				}

				if kv.IsTag {
					l.Debugf("skip tag %q", kv.Key)
					continue
				}

				var kvs point.KVs

				switch v := kv.Val.(type) {
				case *point.Field_F:
					kvs = kvs.Add(kv.Key, v.F)
				case *point.Field_I:
					kvs = kvs.Add(kv.Key, v.I)
				case *point.Field_U:
					kvs = kvs.Add(kv.Key, v.U)
				default:
					// pass: aggregate fields should only be int/float
					l.Debugf("skip non-numbermic field %q", kv.Key)
				}

				if len(kvs) > 0 {
					l.Debugf("fork kv %q as new point", kv.Key)
					forkedPts = append(forkedPts,
						point.NewPoint(ptname, kvs, point.WithTime(pt.Time())))
				}
			}
		}

		// NOTE: we may have selected multiple non-tag field from this point,
		// and we should build a new point on each of these non-tag field.
		if len(forkedPts) > 0 {
			l.Debugf("add %d tags to new points", len(groupby))

			for _, tagKey := range groupby {
				if v := pt.GetTag(tagKey); v != "" {
					for i := range forkedPts { // each point attach these tags
						forkedPts[i].SetTag(tagKey, v)

						if l.Level() == zapcore.DebugLevel {
							l.Debugf("tagged point: %s", forkedPts[i].Pretty())
						}
					}
				}
			}

			res = append(res, forkedPts...)
		}
	}

	return res
}
