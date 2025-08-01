package point

import (
	T "testing"

	"github.com/stretchr/testify/assert"
)

func TestEasyBatch(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		rander := NewRander()
		pts := rander.Rand(10) // random 10 points

		enc := GetEncoder(WithEncEncoding(Protobuf))
		defer PutEncoder(enc)

		arr, err := enc.Encode(pts)
		assert.NoError(t, err)
		assert.Len(t, arr, 1)

		pbbuf := arr[0]

		easyb := NewBatchPoints()
		defer easyb.Release()

		assert.NoError(t, easyb.Unmarshal(pbbuf))

		for i, pt := range pts {
			assert.Equal(t, pt.Pretty(), easyb.Points[i].Pretty())
			t.Logf("[%d]%s", i, pt.Pretty())
		}
	})
}
