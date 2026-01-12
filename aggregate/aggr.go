package aggregate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.uber.org/zap/zapcore"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/GuanceCloud/cliutils"
	fp "github.com/GuanceCloud/cliutils/filter"
	"github.com/GuanceCloud/cliutils/logger"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/cespare/xxhash/v2"
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
	DefaultWindow    time.Duration    `toml:"default_window" json:"default_window"`
	AggregateRules   []*AggregateRule `toml:"aggregate_rules" json:"aggregate_rules"`
	DefaultAction    Action           `toml:"action" json:"action"`
	DeleteRulesPoint bool             `toml:"delete_rules_point" json:"delete_rules_point"`

	hash  uint64
	raw   []byte
	calcs map[uint64]Calculator
}

func (ac *AggregatorConfigure) doHash() {
	if j, err := json.Marshal(ac); err != nil {
		return
	} else {
		ac.raw = j
		ac.hash = xxhash.Sum64(j)
	}
}

// AggregateRule configured a specific aggregate rule.
type AggregateRule struct {
	Name       string                      `toml:"name" json:"name"`
	Selector   *RuleSelector               `toml:"select" json:"select"`
	Groupby    []string                    `toml:"group_by" json:"group_by"`
	Algorithms map[string]*AggregationAlgo `toml:"algorithms" json:"algorithms"`
}

// ruleSelector is the selector to select measurements and fields among points.
type RuleSelector struct {
	Category     string   `toml:"category" json:"category"`
	Measurements []string `toml:"measurements" json:"measurements"`
	Fields       []string `toml:"fields" json:"fields"`
	Condition    string   `toml:"conditon" json:"condition"`

	measurementsWhitelist []*cliutils.WhiteListItem
	fieldsWhitelist       []*cliutils.WhiteListItem
	conds                 fp.WhereConditions
	delSelectKey          bool
}

func (ac *AggregatorConfigure) Setup() error {
	for _, ar := range ac.AggregateRules {
		if err := ar.Selector.Setup(); err != nil {
			return err
		}

		// set default window
		for _, algo := range ar.Algorithms {
			if algo.Window <= 10 {
				algo.Window = int64(ac.DefaultWindow)
			}
		}

		// make the group by tags sorted for point hash.
		sort.Strings(ar.Groupby)
	}

	ac.doHash()
	ac.calcs = map[uint64]Calculator{}

	return nil
}

func (ac *AggregatorConfigure) SelectPoints(pts []*point.Point) (groups [][]*point.Point) {
	for _, ar := range ac.AggregateRules {
		groups = append(groups, ar.SelectPoints(pts))
	}
	return
}

func (rs *RuleSelector) Setup() error {
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

// PickPoints pick points by datakit selector.
func (ar *AggregateRule) PickPoints(pts []*point.Point) map[uint64][]*point.Point {
	// name + groupBy
	res := map[uint64][]*point.Point{}
	for _, pt := range pts {
		h := pickHash(pt, ar.Groupby)
		res[h] = append(res[h], pt)
	}

	return res
}

func (ar *AggregateRule) GroupbyPoints(pts []*point.Point) map[uint64][]*point.Point {
	res := map[uint64][]*point.Point{}
	for _, pt := range pts {
		h := hash(pt, ar.Groupby)
		res[h] = append(res[h], pt)
	}

	return res
}

func (ar *AggregateRule) GroupbyBatch(ac *AggregatorConfigure, pts []*point.Point) (batches []*AggregationBatch) {
	hashedPts := map[uint64][]*point.Point{}
	for _, pt := range pts {
		h := hash(pt, ar.Groupby)
		hashedPts[h] = append(hashedPts[h], pt)
	}

	for h, pts := range hashedPts {
		b := &AggregationBatch{
			RoutingKey:      h,
			ConfigHash:      ac.hash,
			RawConfig:       ac.raw,
			AggregationOpts: ar.Algorithms,
			Points: func() *point.PBPoints {
				var pbpts point.PBPoints
				for _, pt := range pts {
					pbpts.Arr = append(pbpts.Arr, pt.PBPoint())
				}
				return &pbpts
			}(),
		}

		batches = append(batches, b)
	}

	return batches
}

const (
	GuanceRoutingKey = "Guance-Routing-Key"
)

func batchRequest(ab *AggregationBatch, url string) (*http.Request, error) {
	body, err := ab.Marshal()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// add routine header
	req.Header.Set(GuanceRoutingKey, strconv.FormatUint(ab.RoutingKey, 10))
	return req, nil
}

func (s *RuleSelector) doSelect(groupby []string, pts []*point.Point) (res []*point.Point) {
	ptwrapper := &ptWrap{}

	for _, pt := range pts {
		ptname := pt.Name()

		if len(s.measurementsWhitelist) > 0 {
			if !cliutils.WhiteListMatched(ptname, s.measurementsWhitelist) {
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
		var forkedPts = s.selectKVS(false, pt)

		// NOTE: we may have selected multiple non-tag field from this point,
		// and we should build a new point on each of these non-tag field.
		if len(forkedPts) > 0 {
			l.Debugf("add %d tags to new points", len(groupby))

			for _, tagKey := range groupby {
				if v := pt.GetTag(tagKey); v != "" {
					for i := range forkedPts { // each point attach these tags
						forkedPts[i].SetTag(tagKey, v)
					}
				}
				// histogram 'le' 和 'bucket' 关联，不能拆分，需要放进去
				// todo histogram 定制
				if v := pt.Get(tagKey); v != nil {
					for i := range forkedPts { // each point attach these tags
						forkedPts[i].Add(tagKey, v)
					}
				}
			}
			for i := range forkedPts {
				if l.Level() == zapcore.DebugLevel {
					l.Debugf("tagged point: %s", forkedPts[i].Pretty())
				}
			}

			res = append(res, forkedPts...)
		}
	}

	return res
}

func (s *RuleSelector) selectKVS(delKey bool, pt *point.Point) []*point.Point {
	var pts []*point.Point
	// select specific aggregate fields
	if len(s.fieldsWhitelist) > 0 {
		for _, kv := range pt.KVs() {
			if !cliutils.WhiteListMatched(kv.Key, s.fieldsWhitelist) {
				continue
			}

			if kv.IsTag {
				continue
			}
			var kvs point.KVs
			switch v := kv.Val.(type) {
			case *point.Field_F:
				kvs = kvs.Add(kv.Key, v.F)
			case *point.Field_I:
				kvs = kvs.Add(kv.Key, float64(v.I)) // convert all into float
			case *point.Field_U:
				kvs = kvs.Add(kv.Key, float64(v.U)) // convert all into float

			// for category logging-like, we may need to check if tag/field exist/first/last
			case *point.Field_S:
				kvs = kvs.Add(kv.Key, v.S)
			case *point.Field_D:
				kvs = kvs.Add(kv.Key, v.D)
			case *point.Field_B:
				kvs = kvs.Add(kv.Key, v.B)

			default:
				// pass: aggregate fields should only be int/float
				l.Debugf("skip non-numbermic field %q", kv.Key)
			}

			if len(kvs) > 0 {
				l.Debugf("fork kv %q as new point", kv.Key)
				if delKey { // 如果挑选结束需要删除，则删除
					pt.Del(kv.Key)
				}
				pts = append(pts, point.NewPoint(pt.Name(), kvs, point.WithTime(pt.Time())))
			}
		}
	}

	return pts
}
