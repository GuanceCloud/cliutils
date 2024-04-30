// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"sort"
)

func NewPointV2(name string, kvs KVs, opts ...Option) *Point {
	return doNewPoint(name, kvs, opts...)
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

	return doNewPoint(name, kvs, opts...), nil
}

func doNewPoint(name string, kvs KVs, opts ...Option) *Point {
	var pt *Point

	if defaultPTPool != nil {
		pt = defaultPTPool.Get()

		pt.SetName(name)
		pt.pt.Fields = kvs
		pt.SetFlag(Ppooled)
	} else {
		pt = emptyPoint()
		pt.pt.Name = name
		pt.pt.Fields = kvs
	}

	applyCfgOptions(pt.cfg, opts...)

	// add extra tags
	if len(pt.cfg.extraTags) > 0 {
		for _, kv := range pt.cfg.extraTags {
			pt.AddTag(kv.Key, kv.GetS()) // NOTE: do-not-override exist keys
		}
	}

	if pt.cfg.enc == Protobuf {
		pt.SetFlag(Ppb)
	}

	if pt.cfg.keySorted {
		kvs := KVs(pt.pt.Fields)
		sort.Sort(kvs)
		pt.pt.Fields = kvs
	}

	if pt.cfg.precheck {
		pt = pt.cfg.check(pt)
	}

	// sort again: during check, kv maybe update
	if pt.cfg.keySorted {
		sort.Sort(KVs(pt.pt.Fields))
	}

	return pt
}
