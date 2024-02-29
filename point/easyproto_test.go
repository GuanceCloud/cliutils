package point

import (
	T "testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkEasyproto(b *T.B) {
	r := NewRander(WithRandText(3))
	pts := r.Rand(1000)

	pbpts := &PBPoints{}

	for _, pt := range pts {
		pbpts.Arr = append(pbpts.Arr, pt.pt)
	}

	b.ResetTimer()
	b.Run("easyproto-encode", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var dst []byte
			marshalPoints(pts, dst)
		}
	})

	b.Run("gogopb-encode", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			pbpts.Marshal()
		}
	})

	src, err := pbpts.Marshal()
	assert.NoError(b, err)

	b.ResetTimer()
	b.Run("easyproto-decode", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			unmarshalPoints(src)
		}
	})

	b.Run("easyproto-decode", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			var pbpts PBPoints
			pbpts.Unmarshal(src)
		}
	})

	/*
		  goos: darwin
			goarch: arm64
			pkg: github.com/GuanceCloud/cliutils/point
			BenchmarkEasyproto
			BenchmarkEasyproto/easyproto-encode
			BenchmarkEasyproto/easyproto-encode-10         	    1161	   1034532 ns/op	 1202131 B/op	       1 allocs/op
			BenchmarkEasyproto/gogopb-encode
			BenchmarkEasyproto/gogopb-encode-10            	    3010	    401256 ns/op	 1089548 B/op	       1 allocs/op
			BenchmarkEasyproto/easyproto-decode
			BenchmarkEasyproto/easyproto-decode-10         	     560	   2119419 ns/op	 2138183 B/op	   57014 allocs/op
			BenchmarkEasyproto/easyproto-decode#01
			BenchmarkEasyproto/easyproto-decode#01-10      	     601	   1958508 ns/op	 3013150 B/op	   69012 allocs/op
			PASS
			ok  	github.com/GuanceCloud/cliutils/point	6.254s
	*/
}

func TestEasyproto(t *T.T) {
	t.Run("marshal", func(t *T.T) {
		var kvs KVs
		kvs = kvs.AddV2("f1", 123, false)
		kvs = kvs.AddV2("f2", 1.23, false)
		kvs = kvs.AddV2("f3", uint(42), false, WithKVUnit("year"), WithKVType(GAUGE))
		kvs = kvs.AddV2("f4", false, false)
		kvs = kvs.AddV2("f5", []byte("binary-data"), false)
		kvs = kvs.AddV2("f6", "text-data", false)
		kvs = kvs.AddV2("tag-1", "value-1", true, WithKVTagSet(true))

		pts := []*Point{
			NewPointV2("p1", kvs),
		}

		var dst []byte

		dst = marshalPoints(pts, dst)

		dec := GetDecoder(WithDecEncoding(Protobuf))
		defer PutDecoder(dec)

		pts2, err := dec.Decode(dst)
		assert.NoError(t, err)
		assert.Len(t, pts2, 1)

		assert.Equal(t, pts[0].Pretty(), pts2[0].Pretty())

		t.Logf("pt: %s", pts[0].Pretty())
	})

	t.Run("unmarshal", func(t *T.T) {
		var kvs KVs
		kvs = kvs.AddV2("f1", 123, false, WithKVUnit("dollar"), WithKVType(GAUGE))
		kvs = kvs.AddV2("f2", 1.23, false, WithKVUnit("byte"), WithKVType(COUNT))
		kvs = kvs.AddV2("f3", uint(42), false)
		kvs = kvs.AddV2("f4", false, false)
		kvs = kvs.AddV2("f5", []byte("binary-data"), false)
		kvs = kvs.AddV2("f6", "text-data", false)
		kvs = kvs.AddV2("tag-1", "value-1", false, WithKVTagSet(true))

		pts := []*Point{
			NewPointV2("p1", kvs),
		}
		t.Logf("pt: %s", pts[0].Pretty())

		var dst []byte
		dst = marshalPoints(pts, dst)

		pts2, err := unmarshalPoints(dst)
		assert.NoError(t, err)

		assert.Len(t, pts2, 1)

		assert.Equal(t, pts[0].Pretty(), pts2[0].Pretty())

		t.Logf("pt2: %s", pts2[0].Pretty())
	})
}
