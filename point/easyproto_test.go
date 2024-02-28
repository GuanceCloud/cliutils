package point

import (
	T "testing"

	"github.com/stretchr/testify/assert"
)

func TestEasyproto(t *T.T) {
	t.Run("marshal", func(t *T.T) {
		var kvs KVs
		kvs = kvs.Add("f1", 123, false, false)
		kvs = kvs.Add("f2", 1.23, false, false)
		kvs = kvs.Add("f3", uint(42), false, false)
		kvs = kvs.Add("f4", false, false, false)
		kvs = kvs.Add("f5", []byte("binary-data"), false, false)
		kvs = kvs.Add("f6", "text-data", false, false)
		kvs = kvs.Add("tag-1", "value-1", true, false)

		pt := NewPointV2("p1", kvs)

		var pbpts PBPoints
		pbpts.Arr = append(pbpts.Arr, pt.pt)

		var dst []byte

		dst = pbpts.MarshalProtobuf(dst)

		dec := GetDecoder(WithDecEncoding(Protobuf))
		defer PutDecoder(dec)

		pts, err := dec.Decode(dst)
		assert.NoError(t, err)
		assert.Len(t, pts, 1)

		assert.Equal(t, pts[0].Pretty(), pt.Pretty())

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

		pt := NewPointV2("p1", kvs)

		t.Logf("pt: %s", pt.Pretty())

		var pbpts PBPoints
		pbpts.Arr = append(pbpts.Arr, pt.pt)

		var dst []byte

		dst = pbpts.MarshalProtobuf(dst)

		var pbpts2 PBPoints
		assert.NoError(t, pbpts2.UnmarshalProtobuf(dst))

		assert.Len(t, pbpts2.Arr, 1)

		pt2 := Point{pt: pbpts2.Arr[0]}
		assert.Equal(t, pt.Pretty(), pt2.Pretty())

		t.Logf("pt2: %s", pt2.Pretty())
	})
}
