// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.
package ptinput

import (
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/plcache"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/plmap"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ptwindow"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/refertable"
	"github.com/GuanceCloud/cliutils/point"
)

var _ PlInputPt = (*PlPt)(nil)

func (pt *PlPt) GetAggBuckets() *plmap.AggBuckets {
	return pt.aggBuckets
}

func (pt *PlPt) SetAggBuckets(buks *plmap.AggBuckets) {
	pt.aggBuckets = buks
}

func (pt *PlPt) SetPlReferTables(refTable refertable.PlReferTables) {
	pt.refTable = refTable
}

func (pt *PlPt) GetPlReferTables() refertable.PlReferTables {
	return pt.refTable
}

func (pt *PlPt) SetPtWinPool(w *ptwindow.WindowPool) {
	pt.ptWindowPool = w
}

func (pt *PlPt) PtWinRegister(before, after int, k, v []string) {
	if len(k) != len(v) || len(k) == 0 {
		return
	}
	if pt.ptWindowPool != nil && !pt.ptWindowRegistered {
		pt.ptWindowRegistered = true
		pt.ptWindowPool.Register(before, after, k, v)
		pt.winKeyVal = [2][]string{k, v}
	}
}

func (pt *PlPt) PtWinHit() {
	if pt.ptWindowPool != nil && pt.ptWindowRegistered {
		if len(pt.winKeyVal[0]) != len(pt.winKeyVal[1]) || len(pt.winKeyVal[0]) == 0 {
			return
		}

		// 不校验 pipeline 中 point_window 函数执行后的 tag 的值的变化
		//
		if v, ok := pt.ptWindowPool.Get(pt.winKeyVal[0], pt.winKeyVal[1]); ok {
			v.Hit()
		}
	}
}

func (pt *PlPt) CallbackPtWinMove() (result []*point.Point) {
	if pt.ptWindowPool != nil && pt.ptWindowRegistered {
		if v, ok := pt.ptWindowPool.Get(pt.winKeyVal[0], pt.winKeyVal[1]); ok {
			if pt.Dropped() {
				result = v.Move(pt.Point())
			} else {
				result = v.Move(nil)
			}
		}
	}
	return
}

func (pt *PlPt) SetIPDB(db ipdb.IPdb) {
	pt.ipdb = db
}

func (pt *PlPt) GetIPDB() ipdb.IPdb {
	return pt.ipdb
}

func (pt *PlPt) GetCache() *plcache.Cache {
	return pt.cache
}

func (pt *PlPt) SetCache(c *plcache.Cache) {
	pt.cache = c
}

func (pt *PlPt) AppendSubPoint(plpt PlInputPt) {
	pt.subPlpt = append(pt.subPlpt, plpt)
}

func (pt *PlPt) GetSubPoint() []PlInputPt {
	return pt.subPlpt
}
