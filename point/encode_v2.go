// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

// EncodeV2 set points to be encoded.
func (e *Encoder) EncodeV2(pts []*Point) {
	e.pts = pts
}

// Next is a iterator that return encoded data in prepared buf.
// If all points encoded done, return false.
// If any error occurred, we can get the error by call e.LastErr().
func (e *Encoder) Next(buf []byte) ([]byte, bool) {
	switch e.enc {
	case Protobuf:
		return e.doEncodeProtobuf(buf)
	case LineProtocol:
		return e.doEncodeLineProtocol(buf)
	case JSON:
		return e.doEncodeJSON(buf)
	case PBJSON:
		return e.doEncodePBJSON(buf)
	default: // TODO: json
		return nil, false
	}
}

func (e *Encoder) doEncodeProtobuf(buf []byte) ([]byte, bool) {
	var curSize int

	// clear points before current package
	if len(e.pbpts.Arr) > 0 {
		e.pbpts.Arr = e.pbpts.Arr[:0]
	}

	for _, pt := range e.pts[e.lastPtsIdx:] {
		if pt == nil {
			continue
		}

		curSize += pt.PBArraySize()

		if curSize >= len(buf) {
			// no point added, means current point(not added) is a
			// huge point that can't fit into buf.
			if len(e.pbpts.Arr) == 0 {
				e.lastErr = errTooSmallBuffer
				return nil, false
			}

			break // break current package, the remaining points will encoded in next package
		} else {
			e.pbpts.Arr = append(e.pbpts.Arr, pt.pt)
			e.lastPtsIdx++
		}
	}

	if len(e.pbpts.Arr) == 0 { // no points available, it's done
		return nil, false
	}

	if n, err := e.pbpts.MarshalTo(buf); err != nil {
		e.lastErr = err
		return nil, false
	} else {
		if e.fn != nil {
			if err := e.fn(len(e.pbpts.Arr), buf[:n]); err != nil {
				e.lastErr = err
				return nil, false
			}
		}

		e.parts++
		return buf[:n], true
	}
}

func (e *Encoder) doEncodeLineProtocol(buf []byte) ([]byte, bool) {
	curSize := 0
	npts := 0

	for _, pt := range e.pts[e.lastPtsIdx:] {
		if pt == nil {
			continue
		}

		lppt, err := pt.LPPoint()
		if err != nil {
			e.lastErr = err
			continue
		}

		ptsize := lppt.StringSize()

		if curSize+ptsize+1 > len(buf) { // extra +1 used to store the last '\n'
			if curSize == 0 { // nothing added
				e.lastErr = errTooSmallBuffer
				return nil, false
			}

			if e.fn != nil {
				if err := e.fn(npts, buf[:curSize]); err != nil {
					e.lastErr = err
					return nil, false
				}
			}
			e.parts++
			return buf[:curSize], true
		} else {
			e.lpPointBuf = lppt.AppendString(e.lpPointBuf)

			copy(buf[curSize:], e.lpPointBuf[:ptsize])

			// Always add '\n' to the end of current point, this may
			// cause a _unneeded_ '\n' to the end of buf, it's ok for
			// line-protocol parsing.
			buf[curSize+ptsize] = '\n'
			curSize += (ptsize + 1)

			// clean buffer, next time AppendString() append from byte 0
			e.lpPointBuf = e.lpPointBuf[:0]
			e.lastPtsIdx++
			npts++
		}
	}

	if curSize > 0 {
		e.parts++
		if e.fn != nil { // NOTE: encode callback error will terminate encode
			if err := e.fn(npts, buf[:curSize]); err != nil {
				e.lastErr = err
				return nil, false
			}
		}
		return buf[:curSize], true
	} else {
		return nil, false
	}
}

func (e *Encoder) doEncodeJSON(buf []byte) ([]byte, bool) {
	return nil, false
}

func (e *Encoder) doEncodePBJSON(buf []byte) ([]byte, bool) {
	return nil, false
}
