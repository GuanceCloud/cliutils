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

	b.ResetTimer()
	b.Run("gogopb-encode", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			_, err := pbpts.Marshal()
			assert.NoError(b, err)
		}
	})

	src, err := pbpts.Marshal()
	assert.NoError(b, err)

	b.ResetTimer()
	b.Run("easyproto-decode", func(b *T.B) {
		for i := 0; i < b.N; i++ {
			_, err := unmarshalPoints(src)
			assert.NoError(b, err)
		}
	})

	b.ResetTimer()
	b.Run("gogopb-decode", func(b *T.B) {
		var pbpts PBPoints
		for i := 0; i < b.N; i++ {
			err := pbpts.Unmarshal(src)
			assert.NoError(b, err)
		}
	})

	b.ResetTimer()
	b.Run("easybatch-decode", func(b *T.B) {
		bp := NewBatchPoints()
		for i := 0; i < b.N; i++ {
			bp.Reset()
			err := bp.Unmarshal(src)
			assert.NoError(b, err)
		}
	})
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
			NewPointV2("p1", kvs, WithTimestamp(123)),
		}

		var dst []byte

		// marshaled with easyproto
		dst = marshalPoints(pts, dst)

		// unmarshal with gogo-proto
		dec := GetDecoder(WithDecEncoding(Protobuf), WithDecEasyproto(false))
		defer PutDecoder(dec)

		pts2, err := dec.Decode(dst)
		assert.NoError(t, err)
		assert.Len(t, pts2, 1)

		assert.Equal(t, pts[0].Pretty(), pts2[0].Pretty())

		t.Logf("pt: %s", pts[0].Pretty())
	})

	t.Run("easyproto-unmarshal", func(t *T.T) {
		var kvs KVs
		kvs = kvs.AddV2("f1", 123, false, WithKVUnit("dollar"), WithKVType(GAUGE))
		kvs = kvs.AddV2("f2", 1.23, false, WithKVUnit("byte"), WithKVType(COUNT))
		kvs = kvs.AddV2("f3", uint(42), false)
		kvs = kvs.AddV2("f4", false, false)
		kvs = kvs.AddV2("f5", []byte("binary-data"), false)
		kvs = kvs.AddV2("f6", "text-data", false)
		kvs = kvs.AddV2("tag-1", "value-1", false, WithKVTagSet(true))

		pts := []*Point{
			NewPointV2("p1", kvs, WithTimestamp(123)),
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
