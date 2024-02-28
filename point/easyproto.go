// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package point

import (
	"fmt"

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

			pt := &PBPoint{}
			pts.Arr = append(pts.Arr, pt)

			if err := pt.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("unmarshal point failed: %w", err)
			}
		}
	}

	return nil
}

func (pt *PBPoint) UnmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for PBPoint failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if name, ok := fc.String(); ok {
				pt.Name = name
			} else {
				return fmt.Errorf("cannot read PBPoint name")
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Fields for PBPoint")
			}

			f := &Field{}
			pt.Fields = append(pt.Fields, f)
			if err := f.UnmarshalProtobuf(data); err != nil {
				return fmt.Errorf("cannot unmarshal field: %w", err)
			}
		case 3:
			if ts, ok := fc.Int64(); ok {
				pt.Time = ts
			} else {
				return fmt.Errorf("cannot read PBPoint time")
			}

		case 4: // Warns
		case 5: // Debugs
		}
	}

	return err
}

func (f *Field) UnmarshalProtobuf(src []byte) (err error) {
	var fc easyproto.FieldContext

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for Field failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if key, ok := fc.String(); ok {
				f.Key = key
			} else {
				return fmt.Errorf("cannot read Field key")
			}

		case 8:
			if isTag, ok := fc.Bool(); ok {
				f.IsTag = isTag
			} else {
				return fmt.Errorf("cannot unmarshal is-tag for Field")
			}

		case 2:
			if x, ok := fc.Int64(); ok {
				f.Val = &Field_I{I: x}
			}
		case 3:
			if x, ok := fc.Uint64(); ok {
				f.Val = &Field_U{U: x}
			}
		case 4:
			if x, ok := fc.Double(); ok {
				f.Val = &Field_F{F: x}
			}
		case 5:
			if x, ok := fc.Bool(); ok {
				f.Val = &Field_B{B: x}
			}
		case 6:
			if x, ok := fc.Bytes(); ok {
				f.Val = &Field_D{D: x}
			}

		case 11:
			if x, ok := fc.String(); ok {
				f.Val = &Field_S{S: x}
			}

		case 9:
			if x, ok := fc.Int32(); ok {
				f.Type = MetricType(x)
			}
		case 10:
			if x, ok := fc.String(); ok {
				f.Unit = x
			}

		default: // pass
		}
	}

	return err
}
