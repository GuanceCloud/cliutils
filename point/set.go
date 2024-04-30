// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import "time"

func (p *Point) SetName(name string) *Point {
	bufName := p.bufStr(name)
	p.pt.Name = bufName
	return p
}

func (p *Point) SetTime(t time.Time) *Point {
	p.pt.Time = t.UnixNano()
	return p
}

func (p *Point) SetTimestamp(nanoTimestamp int64) *Point {
	p.pt.Time = nanoTimestamp
	return p
}
