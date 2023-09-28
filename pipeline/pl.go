// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package pipeline

import (
	"fmt"

	"github.com/GuanceCloud/cliutils/pipeline/offload"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/pipeline/relation"
	plscript "github.com/GuanceCloud/cliutils/pipeline/script"
	"github.com/GuanceCloud/cliutils/point"
)

type ScriptResult struct {
	pts        []*point.Point
	ptsOffload []*point.Point
	ptsCreated map[point.Category][]*point.Point
}

func (r *ScriptResult) Pts() []*point.Point {
	return r.pts
}

func (r *ScriptResult) PtsOffload() []*point.Point {
	return r.ptsOffload
}

func (r *ScriptResult) PtsCreated() map[point.Category][]*point.Point {
	return r.ptsCreated
}

func RunPl(category point.Category, pts []*point.Point,
	plOpt *plscript.Option, scriptMap map[string]string,
) (reslt *ScriptResult, retErr error) {
	defer func() {
		if err := recover(); err != nil {
			retErr = fmt.Errorf("run pl: %s", err)
		}
	}()

	if plscript.ScriptCount(category) < 1 {
		return &ScriptResult{
			pts: pts,
		}, nil
	}

	ret := []*point.Point{}
	offl := []*point.Point{}
	ptOpt := &point.PointOption{
		DisableGlobalTags: true,
		Category:          category.URL(),
	}

	if plOpt != nil {
		ptOpt.MaxFieldValueLen = plOpt.MaxFieldValLen
	}

	subPt := make(map[point.Category][]*point.Point)
	for _, pt := range pts {
		script, inputData, ok := searchScript(category, pt, scriptMap)

		if !ok || script == nil || inputData == nil {
			ret = append(ret, pt)
			continue
		}

		if offload.Enabled() &&
			script.NS() == plscript.RemoteScriptNS &&
			category == point.Logging {
			offl = append(offl, pt)
			continue
		}

		err := script.Run(inputData, nil, plOpt)
		if err != nil {
			l.Warn(err)
			ret = append(ret, pt)
			continue
		}

		if pts := inputData.GetSubPoint(); len(pts) > 0 {
			for _, pt := range pts {
				if !pt.Dropped() {
					if point, err := pt.DkPoint(); err == nil {
						subPt[pt.Category()] = append(subPt[pt.Category()], point)
					} else {
						l.Warn(err)
					}
				}
			}
		}

		if inputData.Dropped() { // drop
			continue
		}

		if point, err := inputData.DkPoint(); err != nil {
			ret = append(ret, pt)
		} else {
			ret = append(ret, point)
		}
	}

	return &ScriptResult{
		pts:        ret,
		ptsOffload: offl,
		ptsCreated: subPt,
	}, nil
}

func searchScript(cat point.Category, pt *point.Point, scriptMap map[string]string) (*plscript.PlScript, ptinput.PlInputPt, bool) {
	if pt == nil {
		return nil, nil, false
	}
	scriptName, plpt, ok := scriptName(cat, pt, scriptMap)
	if !ok {
		return nil, nil, false
	}

	sc, ok := plscript.QueryScript(cat, scriptName)
	if ok {
		if plpt == nil {
			var err error
			plpt, err = ptinput.WrapDeprecatedPoint(cat, pt)
			if err != nil {
				return nil, nil, false
			}
		}
		return sc, plpt, true
	} else {
		return nil, nil, false
	}
}

func scriptName(cat point.Category, pt *point.Point, scriptMap map[string]string) (string, ptinput.PlInputPt, bool) {
	if pt == nil {
		return "", nil, false
	}

	var scriptName string
	var plpt ptinput.PlInputPt
	var err error

	// built-in rules last
	switch cat { //nolint:exhaustive
	case point.RUM:
		plpt, err = ptinput.WrapDeprecatedPoint(cat, pt)
		if err != nil {
			return "", nil, false
		}
		scriptName = _rumSName(plpt)
	case point.Security:
		plpt, err = ptinput.WrapDeprecatedPoint(cat, pt)
		if err != nil {
			return "", nil, false
		}
		scriptName = _securitySName(plpt)
	case point.Tracing, point.Profiling:
		plpt, err = ptinput.WrapDeprecatedPoint(cat, pt)
		if err != nil {
			return "", nil, false
		}
		scriptName = _apmSName(plpt)
	default:
		scriptName = _defaultCatSName(pt)
	}

	if scriptName == "" {
		return "", plpt, false
	}

	// configuration first
	if sName, ok := scriptMap[scriptName]; ok {
		switch sName {
		case "-":
			return "", nil, false
		case "":
		default:
			return sName, plpt, true
		}
	}

	// remote relation sencond
	if sName, ok := relation.QueryRemoteRelation(cat, scriptName); ok {
		return sName, plpt, true
	}

	return scriptName + ".p", plpt, true
}
