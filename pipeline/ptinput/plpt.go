// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package ptinput

import (
	"fmt"
	"time"

	"github.com/goccy/go-json"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ipdb"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/plcache"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/plmap"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ptwindow"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/refertable"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/utils"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/GuanceCloud/platypus/pkg/ast"
	"github.com/spf13/cast"
)

type PlPt struct {
	name     string
	kvs      point.KVs
	time     int64
	category point.Category

	aggBuckets *plmap.AggBuckets
	ipdb       ipdb.IPdb
	refTable   refertable.PlReferTables
	cache      *plcache.Cache

	subPlpt []PlInputPt

	ptWindowPool       *ptwindow.WindowPool
	winKeyVal          [2][]string
	ptWindowRegistered bool

	drop bool
}

func PtWrap(cat point.Category, pt *point.Point) *PlPt {
	var kvs point.KVs
	if pt.HasFlag(point.Ppooled) {
		ptKVs := pt.KVs()
		kvs = make(point.KVs, 0, len(ptKVs))
		for _, kv := range ptKVs {
			kvs = append(kvs, point.NewKV(kv.Key, kv.Raw(), point.WithKVTagSet(kv.IsTag)))
		}
	} else {
		ptKVs := pt.KVs()
		kvs = make(point.KVs, 0, len(ptKVs))
		kvs = append(kvs, pt.KVs()...)
	}
	return &PlPt{
		name:     pt.Name(),
		kvs:      kvs,
		time:     pt.PBPoint().Time,
		category: cat,
	}
}

func (pp *PlPt) GetPtName() string {
	return pp.name
}

func (pp *PlPt) SetPtName(name string) {
	pp.name = name
}

func (pp *PlPt) Get(k string) (any, ast.DType, error) {
	kv := pp.kvs.Get(k)
	if kv == nil {
		return nil, ast.Nil, ErrKeyNotExist
	}
	v1, v2 := getVal(kv, false)
	return v1, v2, nil
}

func (pp *PlPt) Set(k string, v any, dtype ast.DType) bool {
	pp._set(k, v, false, false)
	return true
}

func (pp *PlPt) SetTag(k string, v any, dtype ast.DType) bool {
	pp._set(k, v, true, false)
	return true
}

func (pp *PlPt) _set(k string, v any, asTag bool, asField bool) {
	// replace high level
	for i, kv := range pp.kvs {
		if kv.Key != k {
			continue
		}
		if !asTag && !asField && kv.IsTag {
			asTag = true
		}

		v, _ := normalVal(v, asTag, false)
		pp.kvs[i] = point.NewKV(k, v, point.WithKVTagSet(asTag))
		return
	}

	// append
	v, _ = normalVal(v, asTag, false)
	pp.kvs = append(pp.kvs, point.NewKV(k, v, point.WithKVTagSet(asTag)))
}

func (pp *PlPt) Delete(k string) {
	pp.delete(k)
}

func (pp *PlPt) delete(k string) (found bool, val any, isTag bool) {
	for i, kv := range pp.kvs {
		if kv.Key == k {
			isTag = kv.IsTag
			val, _ = getVal(kv, false)

			l := len(pp.kvs) - 1
			pp.kvs[i] = pp.kvs[l]
			pp.kvs = pp.kvs[:l]
			return true, val, isTag
		}
	}

	return
}

func (pp *PlPt) RenameKey(from, to string) error {
	if found, val, isTag := pp.delete(from); found {
		pp._set(to, val, isTag, false)
	}
	return nil
}

func (pp *PlPt) MarkDrop(drop bool) {
	pp.drop = drop
}

func (pp *PlPt) Dropped() bool {
	return pp.drop
}

func (pp *PlPt) PtTime() time.Time {
	return time.Unix(0, pp.time)

}

func (pp *PlPt) Category() point.Category {
	return pp.category
}

func (pp *PlPt) Tags() map[string]string {
	tags := map[string]string{}
	for _, kv := range pp.kvs {
		if kv.IsTag {
			if v, ok := kv.Raw().(string); ok {
				tags[kv.Key] = v
			}
		}
	}
	return tags
}

func (pp *PlPt) Fields() map[string]any {
	fields := map[string]any{}
	for _, kv := range pp.kvs {
		if !kv.IsTag {
			fields[kv.Key] = kv.Raw()
		}
	}
	return fields
}

func (pp *PlPt) Point() *point.Point {
	opt := utils.PtCatOption(pp.category)
	opt = append(opt, point.WithTime(pp.PtTime()))

	return point.NewPointV2(pp.name, pp.kvs, opt...)
}

func (pp *PlPt) KeyTime2Time() {
	if v, _, err := pp.Get("time"); err == nil {
		if nanots, ok := v.(int64); ok {
			if pp.time != 0 {
				pp.time = nanots
			}
		}
		pp.Delete("time")
	}
}

func getVal(kv *point.Field, allowComposite bool) (any, ast.DType) {
	if kv == nil {
		return nil, ast.Nil
	}

	var dt ast.DType
	var val any

	switch kv.Val.(type) {
	case *point.Field_I:
		return kv.GetI(), ast.Int
	case *point.Field_U:
		return int64(kv.GetU()), ast.Int
	case *point.Field_F:
		return kv.GetF(), ast.Float
	case *point.Field_B:
		return kv.GetB(), ast.Bool
	case *point.Field_D:
		return string(kv.GetD()), ast.String
	case *point.Field_S:
		return kv.GetS(), ast.String

	case *point.Field_A:
		v, err := point.AnyRaw(kv.GetA())
		if err != nil {
			return nil, ast.Nil
		}
		switch v.(type) {
		case []any:
			val, dt = v, ast.List
		case map[string]any:
			val, dt = v, ast.Map
		default:
			return nil, ast.Nil
		}
	default:
		return nil, ast.Nil
	}

	if !allowComposite {
		if dt == ast.Map || dt == ast.List {
			if v, err := Conv2String(val, dt); err == nil {
				return v, ast.String
			}
		}
	} else {
		return val, dt
	}

	return nil, ast.Nil
}

func normalVal(v any, conv2Str bool, allowComposite bool) (any, ast.DType) {
	var val any
	var dt ast.DType
	switch v := v.(type) {
	case string:
		val, dt = v, ast.String
	case int64:
		val, dt = v, ast.Int
	case int32, int8, int16, int,
		uint, uint16, uint32, uint64, uint8:
		val, dt = cast.ToInt64(v), ast.Int
	case float64:
		val, dt = v, ast.Float
	case float32:
		val, dt = cast.ToFloat64(v), ast.Float
	case bool:
		val, dt = v, ast.Bool
	case []any:
		val, dt = v, ast.List
	case map[string]any:
		val, dt = v, ast.Map
	case []byte:
		val, dt = string(v), ast.String
	default:
		val, dt = nil, ast.Nil
	}

	if conv2Str {
		if dt == ast.Nil {
			return "", ast.String
		}
		if v, err := Conv2String(val, dt); err == nil {
			return v, ast.String
		} else {
			return "", ast.String
		}
	} else {
		if !allowComposite {
			if dt == ast.Map || dt == ast.List {
				if v, err := Conv2String(val, dt); err == nil {
					return v, ast.String
				} else {
					return nil, ast.Nil
				}
			}
		}
		return val, dt
	}
}

func Conv2String(v any, dtype ast.DType) (string, error) {
	switch dtype { //nolint:exhaustive
	case ast.Int, ast.Float, ast.Bool, ast.String:
		return cast.ToString(v), nil
	case ast.List, ast.Map:
		res, err := json.Marshal(v)
		return string(res), err
	case ast.Nil:
		return "", nil
	default:
		return "", fmt.Errorf("unsupported data type %d", dtype)
	}
}
