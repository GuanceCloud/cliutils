// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import "unsafe"

func (p *Point) bufStr(src string) string {
	i := len(p.buf)
	j := i + len(src)

	p.buf = append(p.buf, src[0:]...)

	pointBufCap.Observe(float64(cap(p.buf)))

	offset := p.buf[i:j]
	return *(*string)(unsafe.Pointer(&offset))
}

// New create a new empty *Point.
func New(opts ...Option) (pt *Point) {
	if defaultPTPool != nil {
		pt = defaultPTPool.Get()
	} else {
		pt = emptyPoint()
	}

	pt.cfg = getCfg(opts...)
	return pt
}

func (p *Point) AddKV(k string, v any, force bool, opts ...KVOption) *Point {
	kStr := p.bufStr(k)

	switch x := v.(type) {
	case string:
		vStr := p.bufStr(x)
		old := KVs(p.pt.Fields)
		p.pt.Fields = old.AddV2(kStr, vStr, force, opts...)
		return p
	default:
		old := KVs(p.pt.Fields)
		p.pt.Fields = old.AddV2(kStr, v, force, opts...)
		return p
	}
}

func (p *Point) Check(opts ...Option) *Point {
	if p.cfg == nil {
		return p
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}

		opt(p.cfg)
	}

	return p.cfg.check(p)
}
