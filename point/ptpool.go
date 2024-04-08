package point

import (
	"fmt"
	sync "sync"
	"sync/atomic"

	types "github.com/gogo/protobuf/types"

	p8s "github.com/prometheus/client_golang/prometheus"
)

type PointPool interface {
	Get() *Point
	Put(*Point)

	GetKV(k string, v any) *Field
	PutKV(f *Field)

	String() string

	// For promethues metrics.
	p8s.Collector
}

var defaultPTPool PointPool

func SetPointPool(pp PointPool) {
	defaultPTPool = pp
}

func ClearPointPool() {
	defaultPTPool = nil
}

func (p *Point) clear() {
	if p.pt != nil {
		p.pt.Name = ""
		p.pt.Fields = p.pt.Fields[:0]
		p.pt.Time = 0
		p.pt.Warns = p.pt.Warns[:0]
		p.pt.Debugs = p.pt.Debugs[:0]
	}
}

func (p *Point) Reset() {
	p.flags = 0
	p.clear()
}

func emptyPoint() *Point {
	return &Point{
		pt: &PBPoint{},
	}
}

func isEmptyPoint(pt *Point) bool {
	if pt.pt != nil {
		return pt.flags == 0 &&
			pt.pt.Name == "" &&
			len(pt.pt.Fields) == 0 &&
			len(pt.pt.Warns) == 0 &&
			len(pt.pt.Debugs) == 0 &&
			pt.pt.Time == 0
	} else {
		return pt.flags == 0
	}
}

// NewPointPoolLevel1 get point pool that only cache point but it's key-valus.
func NewPointPoolLevel1() PointPool {
	return &ppv1{}
}

type ppv1 struct {
	sync.Pool
}

func (pp *ppv1) Describe(ch chan<- *p8s.Desc) { p8s.DescribeByCollect(pp, ch) }
func (pp *ppv1) Collect(ch chan<- p8s.Metric) { ch <- nil }

func (pp *ppv1) String() string {
	return ""
}

func (pp *ppv1) Get() *Point {
	if x := pp.Pool.Get(); x == nil {
		return emptyPoint()
	} else {
		return x.(*Point)
	}
}

func (pp *ppv1) Put(pt *Point) {
	pt.Reset()
	pp.Pool.Put(pt)
}

func (pp *ppv1) PutKV(f *Field) {
	// do nothing: all kvs are not cached.
}

func (pp *ppv1) GetKV(k string, v any) *Field {
	return doNewKV(k, v) // ppv1 always return new Field
}

type partialPointPool struct {
	ptpool,
	kvspool sync.Pool
}

// NewPointPoolLevel2 get point cache that cache all but drop Field's Val
func NewPointPoolLevel2() PointPool {
	return &partialPointPool{}
}

func (ppp *partialPointPool) Describe(ch chan<- *p8s.Desc) { p8s.DescribeByCollect(ppp, ch) }
func (ppp *partialPointPool) Collect(ch chan<- p8s.Metric) { ch <- nil }

func (pp *partialPointPool) String() string {
	return ""
}

func (ppp *partialPointPool) Get() *Point {
	if x := ppp.ptpool.Get(); x == nil {
		return emptyPoint()
	} else {
		return x.(*Point)
	}
}

func (ppp *partialPointPool) Put(pt *Point) {

	for _, kv := range pt.KVs() {
		ppp.PutKV(kv)
	}

	pt.Reset()
	ppp.ptpool.Put(pt)
}

func (ppp *partialPointPool) GetKV(k string, v any) *Field {
	if x := ppp.kvspool.Get(); x == nil {
		return doNewKV(k, v)
	} else {
		kv := x.(*Field)
		kv.Key = k
		kv.Val = newVal(v)
		return kv
	}
}

func (ppp *partialPointPool) PutKV(f *Field) {
	clearKV(f)
	ppp.kvspool.Put(f)
}

// NewPointPoolLevel3 cache everything within point.
func NewPointPoolLevel3() PointPool {
	return &fullPointPool{}
}

func (fpp *fullPointPool) Describe(ch chan<- *p8s.Desc) { p8s.DescribeByCollect(fpp, ch) }
func (fpp *fullPointPool) Collect(ch chan<- p8s.Metric) {
	ch <- p8s.MustNewConstMetric(kvCreatedDesc, p8s.CounterValue, float64(fpp.kvCreated.Load()))
	ch <- p8s.MustNewConstMetric(kvReusedDesc, p8s.CounterValue, float64(fpp.kvReused.Load()))
	ch <- p8s.MustNewConstMetric(pointCreatedDesc, p8s.CounterValue, float64(fpp.ptCreated.Load()))
	ch <- p8s.MustNewConstMetric(pointReusedDesc, p8s.CounterValue, float64(fpp.ptReused.Load()))

	ch <- p8s.MustNewConstMetric(kvGetDesc, p8s.CounterValue, float64(fpp.kvGetCount.Load()))
	ch <- p8s.MustNewConstMetric(kvPutDesc, p8s.CounterValue, float64(fpp.kvPutCount.Load()))
	ch <- p8s.MustNewConstMetric(pointGetDesc, p8s.CounterValue, float64(fpp.ptGetCount.Load()))
	ch <- p8s.MustNewConstMetric(pointPutDesc, p8s.CounterValue, float64(fpp.ptPutCount.Load()))
}

func (fpp *fullPointPool) String() string {
	return fmt.Sprintf("kvCreated: % 8d, kvReused: % 8d, ptCreated: % 8d, ptReused: % 8d",
		fpp.kvCreated.Load(),
		fpp.kvReused.Load(),
		fpp.ptCreated.Load(),
		fpp.ptReused.Load(),
	)
}

type fullPointPool struct {
	kvCreated, kvReused,
	ptCreated, ptReused,
	kvGetCount, kvPutCount,
	ptGetCount, ptPutCount atomic.Int64

	ptpool, // pool for *Point
	// other pools for various *Fields
	fpool, // float
	ipool, // int
	upool, // uint
	spool, // string
	bpool, // bool
	dpool, // []byte
	apool sync.Pool // any
}

func (fpp *fullPointPool) PutKV(f *Field) {
	f = resetKV(clearKV(f))

	fpp.kvPutCount.Add(1)

	switch f.Val.(type) {
	case *Field_A:
		fpp.apool.Put(f)
	case *Field_B:
		fpp.bpool.Put(f)
	case *Field_D:
		fpp.dpool.Put(f)
	case *Field_F:
		fpp.fpool.Put(f)
	case *Field_I:
		fpp.ipool.Put(f)
	case *Field_S:
		fpp.spool.Put(f)
	case *Field_U:
		fpp.upool.Put(f)
	}
}

func (fpp *fullPointPool) Put(p *Point) {
	for _, f := range p.KVs() {
		fpp.PutKV(f)
	}

	p.Reset()
	fpp.ptpool.Put(p)

	fpp.ptPutCount.Add(1)
}

func (fpp *fullPointPool) Get() *Point {
	fpp.ptGetCount.Add(1)

	if x := fpp.ptpool.Get(); x == nil {
		fpp.ptCreated.Add(1)
		return emptyPoint()
	} else {
		fpp.ptReused.Add(1)
		return x.(*Point)
	}
}

func (fpp *fullPointPool) getI() *Field {
	if x := fpp.ipool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_I{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getF() *Field {
	if x := fpp.fpool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_F{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getU() *Field {
	if x := fpp.upool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_U{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getD() *Field {
	if x := fpp.dpool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_D{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getS() *Field {
	if x := fpp.spool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_S{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getA() *Field {
	if x := fpp.apool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_A{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getB() *Field {
	if x := fpp.bpool.Get(); x == nil {
		fpp.kvCreated.Add(1)
		return &Field{Val: &Field_B{}}
	} else {
		fpp.kvReused.Add(1)
		return x.(*Field)
	}
}

func (fpp *fullPointPool) GetKV(k string, v any) *Field {
	var (
		kv  *Field
		arr *types.Any
		err error
	)

	fpp.kvGetCount.Add(1)

	switch x := v.(type) {
	case int8:
		kv = fpp.getI()
		kv.Val.(*Field_I).I = int64(x)
	case uint8:
		kv = fpp.getU()
		kv.Val.(*Field_U).U = uint64(x)
	case int16:
		kv = fpp.getI()
		kv.Val.(*Field_I).I = int64(x)
	case uint16:
		kv = fpp.getU()
		kv.Val.(*Field_U).U = uint64(x)
	case int32:
		kv = fpp.getI()
		kv.Val.(*Field_I).I = int64(x)
	case uint32:
		kv = fpp.getU()
		kv.Val.(*Field_U).U = uint64(x)
	case int:
		kv = fpp.getI()
		kv.Val.(*Field_I).I = int64(x)
	case uint:
		kv = fpp.getU()
		kv.Val.(*Field_U).U = uint64(x)
	case int64:
		kv = fpp.getI()
		kv.Val.(*Field_I).I = int64(x)
	case uint64:
		kv = fpp.getU()
		kv.Val.(*Field_U).U = uint64(x)
	case float64:
		kv = fpp.getF()
		kv.Val.(*Field_F).F = float64(x)
	case float32:
		kv = fpp.getF()
		kv.Val.(*Field_F).F = float64(x)
	case string:
		kv = fpp.getS()
		kv.Val.(*Field_S).S = x
	case []byte:
		kv = fpp.getD()
		kv.Val.(*Field_D).D = append(kv.Val.(*Field_D).D, x...)
	case bool:
		kv = fpp.getB()
		kv.Val.(*Field_B).B = x

	case *types.Any: // TODO
		kv = fpp.getA()
		kv.Val.(*Field_A).A = x

		// following are array types
	case []int8:
		kv = fpp.getA()
		arr, err = NewIntArray(x...)
	case []int16:
		kv = fpp.getA()
		arr, err = NewIntArray(x...)
	case []int32:
		kv = fpp.getA()
		arr, err = NewIntArray(x...)
	case []int64:
		kv = fpp.getA()
		arr, err = NewIntArray(x...)
	case []uint16:
		kv = fpp.getA()
		arr, err = NewUintArray(x...)
	case []uint32:
		kv = fpp.getA()
		arr, err = NewUintArray(x...)
	case []uint64:
		kv = fpp.getA()
		arr, err = NewUintArray(x...)

	case []string:
		kv = fpp.getA()
		arr, err = NewStringArray(x...)

	case []bool:
		kv = fpp.getA()
		arr, err = NewBoolArray(x...)

	case [][]byte:
		kv = fpp.getA()
		arr, err = NewBytesArray(x...)

	default: // for nil or other types
		return nil
	}

	// there are array types.
	if arr != nil && err == nil {
		kv.Val.(*Field_A).A = arr
	}

	if kv != nil {
		kv.Key = k
	}

	return kv
}

var (
	kvCreatedDesc = p8s.NewDesc(
		"pointpool_kv_created_total",
		"New created key-value instance",
		nil, nil,
	)

	kvReusedDesc = p8s.NewDesc(
		"pointpool_kv_reused_total",
		"Reused key-value instance count",
		nil, nil,
	)

	pointCreatedDesc = p8s.NewDesc(
		"pointpool_point_created_total",
		"New created point instance count",
		nil, nil,
	)

	pointReusedDesc = p8s.NewDesc(
		"pointpool_point_reused_total",
		"Reused point instance count",
		nil, nil,
	)

	pointGetDesc = p8s.NewDesc(
		"pointpool_point_get_total",
		"Get point count",
		nil, nil,
	)

	pointPutDesc = p8s.NewDesc(
		"pointpool_point_put_total",
		"Put point count",
		nil, nil,
	)

	kvGetDesc = p8s.NewDesc(
		"pointpool_kv_get_total",
		"Get key-value count",
		nil, nil,
	)

	kvPutDesc = p8s.NewDesc(
		"pointpool_kv_put_total",
		"Put key-value count",
		nil, nil,
	)
)
