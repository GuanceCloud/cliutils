// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

// Package plpt implement pipeline point.
package plpt

import (
	"github.com/GuanceCloud/cliutils/point"
	"github.com/GuanceCloud/platypus/pkg/ast"
)

type PlPt struct {
	kvs      []*point.Field
	kvsDelPp []int
	originPp *point.Point
	pooled   bool
}

func PtWrap(pp *point.Point) *PlPt {
	return &PlPt{
		originPp: pp,
		pooled:   pp.HasFlag(point.Ppooled),
	}
}

func (pt *PlPt) Get(k string) (any, ast.DType) {
	oKvs := pt.originPp.KVs()
	for i := range pt.kvsDelPp {
		if oKvs[i].Key == k {
			return nil, ast.Nil
		}
	}

	for _, kv := range pt.kvs {
		if kv.Key != k {
			continue
		}
		return getVal(kv)
	}

	for _, kv := range oKvs {
		if kv.Key != k {
			continue
		}
		return getVal(kv)
	}
	return nil, ast.Nil
}

func (pt *PlPt) Set(k string, v any, dtype ast.DType, asTag bool) {
	oKvs := pt.originPp.KVs()
	for i := range pt.kvsDelPp {
		if oKvs[i].Key == k {
			pt.kvsDelPp = append(pt.kvsDelPp[:i], pt.kvsDelPp[i+1:]...)
			pt.kvs = append(pt.kvs, point.NewKV(k, v, point.WithKVTagSet(asTag)))
			return
		}
	}

	// replace
	for i, kv := range pt.kvs {
		if kv.Key != k {
			continue
		}
		if pt.pooled {
			if pool := point.GetPointPool(); pool != nil {
				pool.PutKV(kv)
				pt.kvs[i] = point.NewKV(k, v, point.WithKVTagSet(asTag))
			}
			return
		} else {
			pt.kvs[i] = point.NewKV(k, v, point.WithKVTagSet(asTag))
			return
		}
	}

	// append
	pt.kvs = append(pt.kvs, point.NewKV(k, v, point.WithKVTagSet(asTag)))
}

func (pt *PlPt) Delete(k string) {
	oKvs := pt.originPp.KVs()
	for i := range pt.kvsDelPp {
		if oKvs[i].Key == k {
			return
		}
	}

	for i, kv := range pt.kvs {
		if kv.Key == k {
			if pt.pooled {
				if pool := point.GetPointPool(); pool != nil {
					pool.PutKV(kv)
				}
			}
			pt.kvs = append(pt.kvs[:i], pt.kvs[i+1:]...)
			break
		}
	}

	// append to kvsDel, if k in origin pt
	for i, kv := range oKvs {
		if kv.Key == k {
			pt.kvsDelPp = append(pt.kvsDelPp, i)
			break
		}
	}
}

func getVal(kv *point.Field) (any, ast.DType) {
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
		raw := kv.GetD()
		r := make([]any, 0, len(raw))
		for _, v := range raw {
			r = append(r, v)
		}
		return r, ast.List
	case *point.Field_S:
		return kv.GetS(), ast.String

	case *point.Field_A:
		v, err := point.AnyRaw(kv.GetA())
		if err != nil {
			return nil, ast.Nil
		}
		switch v.(type) {
		case []any:
			return v, ast.List
		case map[string]any:
			return v, ast.Map
		default:
			return nil, ast.Nil
		}
	default:
		return nil, ast.Nil
	}
}
