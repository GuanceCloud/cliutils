package point

import (
	"fmt"
	sync "sync"

	"github.com/VictoriaMetrics/easyproto"
)

func NewBatchPoints() *BatchPoints {
	return bpPool.Get().(*BatchPoints)
}

type BatchPoints struct {
	Points []*Point

	fieldsPool  []*Field
	fieldIsPool []Field_I
	fieldUsPool []Field_U
	fieldFsPool []Field_F
	fieldBsPool []Field_B
	fieldDsPool []Field_D
	fieldSsPool []Field_S
	fieldAsPool []Field_A
}

func (bp *BatchPoints) Reset() {
	for _, pt := range bp.Points {
		pt.pt.Fields = nil
	}
	bp.Points = bp.Points[:0]

	bp.fieldsPool = bp.fieldsPool[:0]
	bp.fieldIsPool = bp.fieldIsPool[:0]
	bp.fieldUsPool = bp.fieldUsPool[:0]
	bp.fieldFsPool = bp.fieldFsPool[:0]
	bp.fieldBsPool = bp.fieldBsPool[:0]
	bp.fieldDsPool = bp.fieldDsPool[:0]
	bp.fieldSsPool = bp.fieldSsPool[:0]
	bp.fieldAsPool = bp.fieldAsPool[:0]
}

func (bp *BatchPoints) Release() {
	bp.Reset()
	bpPool.Put(bp)
}

func (bp *BatchPoints) Unmarshal(src []byte) (err error) {
	fc := fcPool.Get().(*easyproto.FieldContext)
	defer fcPool.Put(fc)

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for batch point failed: %w", err)
		}
		if fc.FieldNum != 1 {
			return fmt.Errorf("unknown field number: %d", fc.FieldNum)
		}

		data, ok := fc.MessageData()
		if !ok {
			return fmt.Errorf("cannot read read data for batch point")
		}

		if cap(bp.Points) > len(bp.Points) {
			bp.Points = bp.Points[:len(bp.Points)+1]
			if bp.Points[len(bp.Points)-1] == nil {
				pt := &Point{pt: &PBPoint{}}
				bp.Points[len(bp.Points)-1] = pt
			}
		} else {
			bp.Points = append(bp.Points, &Point{pt: &PBPoint{}})
		}
		pt := bp.Points[len(bp.Points)-1]

		if err := bp.unmarshalPoint(fc, pt, data); err != nil {
			return fmt.Errorf("unmarshal point failed: %w", err)
		}
	}

	if len(src) > 0 {
		return fmt.Errorf("unmarshal tail bytes: %v", src)
	}
	return nil
}

func (bp *BatchPoints) unmarshalPoint(fc *easyproto.FieldContext, pt *Point, src []byte) (err error) {
	fieldsLen := len(bp.fieldsPool)

	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for PBPoint failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if x, ok := fc.String(); ok {
				pt.pt.Name = x
			} else {
				return fmt.Errorf("cannot read PBPoint name")
			}
		case 2:
			data, ok := fc.MessageData()
			if !ok {
				return fmt.Errorf("cannot read Fields for PBPoint")
			}

			if cap(bp.fieldsPool) > len(bp.fieldsPool) {
				bp.fieldsPool = bp.fieldsPool[:len(bp.fieldsPool)+1]
				if bp.fieldsPool[len(bp.fieldsPool)-1] == nil {
					bp.fieldsPool[len(bp.fieldsPool)-1] = &Field{}
				}
			} else {
				bp.fieldsPool = append(bp.fieldsPool, &Field{})
			}
			field := bp.fieldsPool[len(bp.fieldsPool)-1]

			if err := bp.unmarshalField(fc, field, data); err != nil {
				return fmt.Errorf("cannot unmarshal field: %w", err)
			}
		case 3:
			if x, ok := fc.Int64(); ok {
				pt.pt.Time = x
			} else {
				return fmt.Errorf("cannot read PBPoint time")
			}
		}
	}

	pt.pt.Fields = bp.fieldsPool[fieldsLen:]
	return nil
}

func (bp *BatchPoints) unmarshalField(fc *easyproto.FieldContext, field *Field, src []byte) (err error) {
	for len(src) > 0 {
		src, err = fc.NextField(src)
		if err != nil {
			return fmt.Errorf("read next field for Field failed: %w", err)
		}

		switch fc.FieldNum {
		case 1:
			if x, ok := fc.String(); ok {
				field.Key = x
			} else {
				return fmt.Errorf("cannot read Field key")
			}
		case 8:
			if x, ok := fc.Bool(); ok {
				field.IsTag = x
			} else {
				return fmt.Errorf("cannot unmarshal is-tag for Field")
			}
		case 2:
			if x, ok := fc.Int64(); ok {
				if cap(bp.fieldIsPool) > len(bp.fieldIsPool) {
					bp.fieldIsPool = bp.fieldIsPool[:len(bp.fieldIsPool)+1]
				} else {
					bp.fieldIsPool = append(bp.fieldIsPool, Field_I{})
				}
				iVal := &bp.fieldIsPool[len(bp.fieldIsPool)-1]
				iVal.I = x
				field.Val = iVal
			}
		case 3:
			if x, ok := fc.Uint64(); ok {
				if cap(bp.fieldUsPool) > len(bp.fieldUsPool) {
					bp.fieldUsPool = bp.fieldUsPool[:len(bp.fieldUsPool)+1]
				} else {
					bp.fieldUsPool = append(bp.fieldUsPool, Field_U{})
				}
				uVal := &bp.fieldUsPool[len(bp.fieldUsPool)-1]
				uVal.U = x
				field.Val = uVal
			}
		case 4:
			if x, ok := fc.Double(); ok {
				if cap(bp.fieldFsPool) > len(bp.fieldFsPool) {
					bp.fieldFsPool = bp.fieldFsPool[:len(bp.fieldFsPool)+1]
				} else {
					bp.fieldFsPool = append(bp.fieldFsPool, Field_F{})
				}
				fVal := &bp.fieldFsPool[len(bp.fieldFsPool)-1]
				fVal.F = x
				field.Val = fVal
			}
		case 5:
			if x, ok := fc.Bool(); ok {
				if cap(bp.fieldBsPool) > len(bp.fieldBsPool) {
					bp.fieldBsPool = bp.fieldBsPool[:len(bp.fieldBsPool)+1]
				} else {
					bp.fieldBsPool = append(bp.fieldBsPool, Field_B{})
				}
				bVal := &bp.fieldBsPool[len(bp.fieldBsPool)-1]
				bVal.B = x
				field.Val = bVal
			}
		case 6:
			if x, ok := fc.Bytes(); ok {
				if cap(bp.fieldDsPool) > len(bp.fieldDsPool) {
					bp.fieldDsPool = bp.fieldDsPool[:len(bp.fieldDsPool)+1]
				} else {
					bp.fieldDsPool = append(bp.fieldDsPool, Field_D{})
				}
				dVal := &bp.fieldDsPool[len(bp.fieldDsPool)-1]
				dVal.D = x
				field.Val = dVal
			}
		case 11:
			if x, ok := fc.String(); ok {
				if cap(bp.fieldSsPool) > len(bp.fieldSsPool) {
					bp.fieldSsPool = bp.fieldSsPool[:len(bp.fieldSsPool)+1]
				} else {
					bp.fieldSsPool = append(bp.fieldSsPool, Field_S{})
				}
				sVal := &bp.fieldSsPool[len(bp.fieldSsPool)-1]
				sVal.S = x
				field.Val = sVal
			}
		case 9:
			if x, ok := fc.Int32(); ok {
				field.Type = MetricType(x)
			}
		case 10:
			if x, ok := fc.String(); ok {
				field.Unit = x
			}
		default: // pass
		}
	}
	return nil
}

var (
	bpPool = sync.Pool{
		New: func() any {
			return &BatchPoints{
				Points: make([]*Point, 0, 1000),
			}
		},
	}
	fcPool = sync.Pool{
		New: func() any {
			return &easyproto.FieldContext{}
		},
	}
)
