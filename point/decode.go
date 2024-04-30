// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"encoding/json"
	sync "sync"
)

var decPool sync.Pool

// DecodeFn used to iterate on []*Point payload, if error returned, the iterate terminated.
type DecodeFn func([]*Point) error

type DecoderOption func(e *Decoder)

func WithDecEncoding(enc Encoding) DecoderOption {
	return func(d *Decoder) { d.enc = enc }
}

func WithDecFn(fn DecodeFn) DecoderOption {
	return func(d *Decoder) { d.fn = fn }
}

func WithDecEasyproto(on bool) DecoderOption {
	return func(d *Decoder) { d.easyproto = on }
}

type Decoder struct {
	enc Encoding
	fn  DecodeFn

	easyproto bool

	// For line-protocol parsing, keep original error.
	detailedError error
}

func GetDecoder(opts ...DecoderOption) *Decoder {
	v := decPool.Get()
	if v == nil {
		v = newDecoder()
	}

	x := v.(*Decoder)

	for _, opt := range opts {
		if opt != nil {
			opt(x)
		}
	}

	return x
}

func PutDecoder(d *Decoder) {
	d.reset()
	decPool.Put(d)
}

func newDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) reset() {
	d.enc = 0
	d.fn = nil
	d.detailedError = nil
	d.easyproto = false
}

func (d *Decoder) Decode(data []byte, opts ...Option) ([]*Point, error) {
	var (
		pts []*Point
		err error
	)

	switch d.enc {
	case JSON:
		var arr []JSONPoint
		if err := json.Unmarshal(data, &arr); err != nil {
			return nil, err
		}

		for _, x := range arr {
			if pt, err := x.Point(opts...); err != nil {
				return nil, err
			} else {
				pts = append(pts, pt)
			}
		}

	case Protobuf:
		if d.easyproto || defaultPTPool != nil { // force use easyproto when point pool enabled
			pts, err = unmarshalPoints(data)
			if err != nil {
				return nil, err
			}
		} else {
			var pbpts PBPoints
			if err = pbpts.Unmarshal(data); err != nil {
				return nil, err
			}

			for _, pbpt := range pbpts.Arr {
				pts = append(pts, FromPB(pbpt))
			}
		}

		// the opts not applied to pts, apply again.
		if len(opts) > 0 {
			for i, pt := range pts {
				pt.cfg = applyCfgOptions(pt.cfg, opts...)
				pts[i] = pt.cfg.check(pt)
			}
		}

	case LineProtocol:
		pts, err = parseLPPoints(data, opts...)
		if err != nil {
			d.detailedError = err
			return nil, simplifyLPError(err)
		}
	}

	if d.fn != nil {
		return pts, d.fn(pts)
	}

	return pts, nil
}

func (d *Decoder) DetailedError() error {
	return d.detailedError
}
