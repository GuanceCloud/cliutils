// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"
	"sort"
	"time"
)

func NewPointV3(name string, kvs [][]any, opts ...Option) *Point {
	var pt *Point

	c := GetCfg(opts...)
	defer PutCfg(c)

	if c.pointPool != nil {
		pt = c.pointPool.Get()
	} else {
		pt = emptyPoint()
	}

	pt.SetName(name)

	for _, kv := range kvs {
		var (
			field *Field
			value any
			isTag bool
		)

		switch len(kv) {
		case 0:
			pt.pt.Warns = append(pt.pt.Warns, &Warn{Type: "no key and value", Msg: "key or value are required"})
		case 1:
			value = nil // value not set, default to nil
		case 3:
			if x, ok := kv[2].(bool); ok { // check if boolean
				isTag = x
			} else {
				pt.pt.Warns = append(pt.pt.Warns, &Warn{Type: "invalid is-tag", Msg: "2nd element should be boolean to set the kv is tag or not"})
			}

		case 4: // for Type
			pt.pt.Warns = append(pt.pt.Warns, &Warn{Type: "type not implemented", Msg: "set Type not implemented"})
		case 5: // for Unit
			pt.pt.Warns = append(pt.pt.Warns, &Warn{Type: "unit not implemented", Msg: "set Unit not implemented"})
		default:
		}

		if len(kv) == 0 {
			continue
		}

		if len(kv) >= 2 {
			value = kv[1]
		}

		if key, ok := kv[0].(string); !ok {
			pt.pt.Warns = append(pt.pt.Warns, &Warn{Type: "invalid key", Msg: fmt.Sprintf("key %q's type must string type", key)})
		} else {
			if c.pointPool != nil {
				field = c.pointPool.GetKV(key, value)
			} else {
				field = NewKV(key, value)
			}

			if field == nil {
				pt.pt.Warns = append(pt.pt.Warns, &Warn{Type: "invalid field", Msg: fmt.Sprintf("got nil or invalid field value on key %q", key)})
			} else {
				field.IsTag = isTag
				pt.AddKVs(field)
			}
		}
	}

	// add extra tags
	if len(c.extraTags) > 0 {
		for _, kv := range c.extraTags {
			if c.pointPool != nil {
				kv := c.pointPool.GetKV(kv.Key, kv.GetS())
				kv.IsTag = true
			}
			pt.AddKVs(kv) // NOTE: do-not-override exist keys
		}
	}

	if c.enc == Protobuf {
		pt.SetFlag(Ppb)
	}

	if c.precheck {
		chk := checker{cfg: c}
		pt = chk.check(pt)
		pt.SetFlag(Pcheck)
	}

	if c.keySorted {
		kvs := KVs(pt.pt.Fields)
		sort.Sort(kvs)
		pt.pt.Fields = kvs
	}

	if !c.t.IsZero() {
		pt.pt.Time = c.t.Round(0).UnixNano() // trim monotonic clock
	} else {
		pt.pt.Time = time.Now().Round(0).UnixNano() // trim monotonic clock
	}

	return pt
}

func NewPointV2(name string, kvs KVs, opts ...Option) *Point {
	c := GetCfg(opts...)
	defer PutCfg(c)

	return doNewPoint(name, kvs, c)
}

// NewPoint returns a new Point given name(measurement), tags, fields and optional options.
//
// If fields empty(or nil), error ErrNoField will returned.
//
// Values in fields only allowed for int/uint(8-bit/16-bit/32-bit/64-bit), string, bool,
// float(32-bit/64-bit) and []byte, other types are ignored.
//
// Deprecated: use NewPointV2.
func NewPoint(name string, tags map[string]string, fields map[string]any, opts ...Option) (*Point, error) {
	if len(fields) == 0 {
		return nil, ErrNoFields
	}

	kvs := NewKVs(fields)
	for k, v := range tags {
		kvs = kvs.MustAddTag(k, v) // force add these tags
	}

	c := GetCfg(opts...)
	defer PutCfg(c)

	return doNewPoint(name, kvs, c), nil
}

func doNewPoint(name string, kvs KVs, c *cfg) *Point {
	var pt *Point

	if c.pointPool != nil {
		pt = c.pointPool.Get()
	} else {
		pt = &Point{
			pt: &PBPoint{
				Name:   name,
				Fields: kvs,
			},
		}
	}

	// add extra tags
	if len(c.extraTags) > 0 {
		for _, kv := range c.extraTags {
			pt.AddTag(kv.Key, kv.GetS()) // NOTE: do-not-override exist keys
		}
	}

	if c.enc == Protobuf {
		pt.SetFlag(Ppb)
	}

	if c.keySorted {
		kvs := KVs(pt.pt.Fields)
		sort.Sort(kvs)
		pt.pt.Fields = kvs
	}

	if c.precheck {
		chk := checker{cfg: c}
		pt = chk.check(pt)
		pt.SetFlag(Pcheck)
	}

	// sort again: during check, kv maybe update
	if c.keySorted {
		sort.Sort(KVs(pt.pt.Fields))
	}

	if !c.t.IsZero() {
		pt.pt.Time = c.t.Round(0).UnixNano() // trim monotonic clock
	}

	if pt.pt.Time == 0 {
		pt.pt.Time = time.Now().Round(0).UnixNano() // trim monotonic clock
	}

	return pt
}
