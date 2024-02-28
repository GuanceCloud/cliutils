// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"
	"time"

	"github.com/VictoriaMetrics/easyproto"
)

var mp easyproto.MarshalerPool

// marshal
func (pts *PBPoints) MarshalProtobuf(dst []byte) []byte {
	m := mp.Get()
	mm := m.MessageMarshaler()

	for _, pt := range pts.Arr {
		pt.marshalProtobuf(mm.AppendMessage(1))
	}

	dst = m.Marshal(dst)
	mp.Put(m)
	return dst
}

func (pt *PBPoint) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, pt.Name)
	for _, f := range pt.Fields {
		f.marshalProtobuf(mm.AppendMessage(2))
	}

	mm.AppendInt64(3, pt.Time)

	for _, w := range pt.Warns {
		w.marshalProtobuf(mm.AppendMessage(4))
	}

	for _, d := range pt.Debugs {
		d.marshalProtobuf(mm.AppendMessage(5))
	}
}

func (f *Field) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, f.Key)

	switch x := f.Val.(type) {
	case *Field_I:
		mm.AppendInt64(2, x.I)
	case *Field_U:
		mm.AppendUint64(3, x.U)
	case *Field_F:
		mm.AppendDouble(4, x.F)
	case *Field_B:
		mm.AppendBool(5, x.B)
	case *Field_D:
		mm.AppendBytes(6, x.D)
	case *Field_S:
		mm.AppendString(11, x.S)
	case *Field_A:
		// TODO
	}

	mm.AppendBool(8, f.IsTag)
	mm.AppendInt32(9, int32(f.Type))
	mm.AppendString(10, f.Unit)
}

func (w *Warn) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, w.Type)
	mm.AppendString(2, w.Msg)
}

func (d *Debug) marshalProtobuf(mm *easyproto.MessageMarshaler) {
	mm.AppendString(1, d.Info)
}

// unmarshal

func (pts *PBPoints) UnmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for PBPoints failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read read Arr for PBPoints")
			}

			if pt, err := unmarshalPoint(data); err != nil {
				return fmt.Errorf("unmarshal point failed: %w", err)
			} else {
				pts.Arr = append(pts.Arr, pt)
			}
		}
	}

	return nil
}

func unmarshalPoint(src []byte) (*PBPoint, error) {
	var (
		fc     easyproto.FieldContext
		kvs    KVs
		warns  []*Warn
		debugs []*Debug
		name   string
		ts     int64
		err    error
	)

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("read next field for PBPoint failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if x, ok := fc.String(); ok {
				name = x
			} else {
				return nil, fmt.Errorf("cannot read PBPoint name")
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read Fields for PBPoint")
			}

			if kv, err := unmarshalField(data); err == nil {
				kvs = append(kvs, kv)
			} else {
				return nil, fmt.Errorf("cannot unmarshal field: %w", err)
			}
		case 3:
			if x, ok := fc.Int64(); ok {
				ts = x
			} else {
				return nil, fmt.Errorf("cannot read PBPoint time")
			}

		case 4: // Warns
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read Warn for PBPoint")
			}

			if x, err := unmarshalWarn(data); err == nil {
				warns = append(warns, x)
			}

		case 5: // Debugs
			data, ok := fc.MessageData()
			if !ok {
				return nil, fmt.Errorf("cannot read Debug for PBPoint")
			}

			if x, err := unmarshalDebug(data); err == nil {
				debugs = append(debugs, x)
			}
		}
	}

	pt := NewPointV2(name, kvs, WithTime(time.Unix(0, ts)))
	return pt.pt, err
}

func unmarshalWarn(src []byte) (*Warn, error) {
	var (
		wtype, wmsg string
		fc          easyproto.FieldContext
		err         error
	)

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("read next field for Warn failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if x, ok := fc.String(); ok {
				wtype = x
			}
		case 2:
			if x, ok := fc.String(); ok {
				wmsg = x
			}
		}
	}

	return &Warn{Type: wtype, Msg: wmsg}, nil
}

func unmarshalDebug(src []byte) (*Debug, error) {
	var (
		info string
		fc   easyproto.FieldContext
		err  error
	)

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("read next field for Debug failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if x, ok := fc.String(); ok {
				info = x
			}
		}
	}

	return &Debug{Info: info}, nil
}

func unmarshalField(src []byte) (*Field, error) {
	var (
		fc         easyproto.FieldContext
		key, unit  string
		isTag      bool
		f          *Field
		metricType MetricType
		err        error
	)

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return nil, fmt.Errorf("read next field for Field failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if x, ok := fc.String(); ok {
				key = x
			} else {
				return nil, fmt.Errorf("cannot read Field key")
			}

		case 8:
			if x, ok := fc.Bool(); ok {
				isTag = x
			} else {
				return nil, fmt.Errorf("cannot unmarshal is-tag for Field")
			}

		case 2:
			if x, ok := fc.Int64(); ok {
				f = NewKV(key, x)
			}

		case 3:
			if x, ok := fc.Uint64(); ok {
				f = NewKV(key, x)
			}
		case 4:
			if x, ok := fc.Double(); ok {
				f = NewKV(key, x)
			}
		case 5:
			if x, ok := fc.Bool(); ok {
				f = NewKV(key, x)
			}
		case 6:
			if x, ok := fc.Bytes(); ok {
				f = NewKV(key, x)
			}

		case 11:
			if x, ok := fc.String(); ok {
				f = NewKV(key, x)
			}

		case 9:
			if x, ok := fc.Int32(); ok {
				metricType = MetricType(x)
			}
		case 10:
			if x, ok := fc.String(); ok {
				unit = x
			}
		default: // pass
		}
	}

	if f != nil {
		f.Unit = unit
		f.Type = metricType
		f.IsTag = isTag
	}

	return f, err
}
