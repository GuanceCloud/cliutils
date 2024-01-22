package point

import (
	"fmt"
	sync "sync"
	T "testing"
	"time"

	types "github.com/gogo/protobuf/types"
)

var ptpool sync.Pool

func getPt() *Point {
	if x := ptpool.Get(); x == nil {
		pt := NewPointV2("", nil, WithPrecheck(false))
		return pt
	} else {
		return x.(*Point)
	}
}

func putPt(p *Point) {
	p.Reset()
	ptpool.Put(p)
}

func TestReset(t *T.T) {

	t.Run("reset", func(t *T.T) {
		for i := 0; i < 10; i++ {
			pt := getPt()

			pt.SetName(fmt.Sprintf("m-%d", i))
			pt.SetTime(time.Now())
			pt.Add(fmt.Sprintf("f1-%d", i), 123)
			pt.Add(fmt.Sprintf("f2-%d", i), 345)
			pt.Add(fmt.Sprintf("f1-%d", i), 567)

			t.Logf("point: %s", pt.Pretty())
			putPt(pt)
		}
	})
}

func BenchmarkDefaultPool(b *T.B) {

	now := time.Now()
	pp := NewPointPool()

	b.Run("with-reset", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			pt := pp.Get()

			pt.SetName("m1")
			pt.SetTime(now)
			pt.Add("f0", 123)
			pt.Add("f1", 3.14)
			pt.Add("f2", "hello")
			pt.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"))
			pt.Add("f4", false)
			pt.Add("f5", -123)

			pp.Put(pt)
		}
	})
}

func BenchmarkNoPool(b *T.B) {
	now := time.Now()

	b.Run("without-reset", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var kvs KVs

			kvs = kvs.Add("f0", 123, false, true)
			kvs = kvs.Add("f1", 3.14, false, true)
			kvs = kvs.Add("f2", "hello", false, true)
			kvs = kvs.Add("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text"), false, true)
			kvs = kvs.Add("f4", false, false, false)
			kvs = kvs.Add("f5", -123, false, false)

			NewPointV2("m1", kvs, WithTime(now), WithPrecheck(false))
		}
	})
}

func (fpp *fullPointPool) Put(p *Point) {

	kvs := p.KVs()
	kvs.ResetFull()

	p.Reset()
	fpp.ptpool.Put(p)

	for _, kv := range kvs {
		switch kv.Val.(type) {
		case *Field_A:
			fpp.apool.Put(kv)
		case *Field_B:
			fpp.bpool.Put(kv)
		case *Field_D:
			fpp.dpool.Put(kv)
		case *Field_F:
			fpp.fpool.Put(kv)
		case *Field_I:
			fpp.ipool.Put(kv)
		case *Field_S:
			fpp.spool.Put(kv)
		case *Field_U:
			fpp.upool.Put(kv)
		default: // pass
		}
	}
}

func (fpp *fullPointPool) Get() *Point {
	if x := fpp.ptpool.Get(); x == nil {
		pt := NewPointV2("", nil, WithPrecheck(false))
		return pt
	} else {
		return x.(*Point)
	}
}

func (fpp *fullPointPool) getI() *Field {
	if x := fpp.ipool.Get(); x == nil {
		return &Field{Val: &Field_I{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getF() *Field {
	if x := fpp.fpool.Get(); x == nil {
		return &Field{Val: &Field_F{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getU() *Field {
	if x := fpp.upool.Get(); x == nil {
		return &Field{Val: &Field_U{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getD() *Field {
	if x := fpp.dpool.Get(); x == nil {
		return &Field{Val: &Field_D{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getS() *Field {
	if x := fpp.spool.Get(); x == nil {
		return &Field{Val: &Field_S{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getA() *Field {
	if x := fpp.apool.Get(); x == nil {
		return &Field{Val: &Field_A{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getB() *Field {
	if x := fpp.bpool.Get(); x == nil {
		return &Field{Val: &Field_B{}}
	} else {
		return x.(*Field)
	}
}

func (fpp *fullPointPool) getKV(k string, v any) *Field {
	var kv *Field

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
		return nil

	case nil: // pass
		return nil

	default: // value ignored
		return nil
	}

	kv.Key = k

	return kv
}

type fullPointPool struct {
	ptpool,
	fpool,
	ipool,
	upool,
	spool,
	bpool,
	dpool,
	apool sync.Pool
}

func BenchmarkFullPool(b *T.B) {
	now := time.Now()

	var fpp fullPointPool

	b.Run("full-pool", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			pt := fpp.Get()

			pt.SetName("m1")
			pt.SetTime(now)

			pt.AddKV(fpp.getKV("f0", 123))
			pt.AddKV(fpp.getKV("f1", 3.14))
			pt.AddKV(fpp.getKV("f2", "hello"))
			pt.AddKV(fpp.getKV("f3", []byte("some looooooooooooooooooooooooooooooooooooooooooooooong text")))
			pt.AddKV(fpp.getKV("f4", false))
			pt.AddKV(fpp.getKV("f5", -123))

			fpp.Put(pt)
		}
	})
}

type partialPointPool struct {
	ptpool,
	kvspool sync.Pool
}

func (ppp *partialPointPool) Get() *Point {
	return nil
}

func (ppp *partialPointPool) Put(pt *Point) {

	kvs := pt.KVs()
	kvs.Reset()

	for _, kv := range kvs {
		ppp.kvspool.Put(kv)
	}

	pt.Reset()
	ppp.ptpool.Put(pt)
}
