package point

import (
	T "testing"

	"github.com/stretchr/testify/assert"
)

func TestSetVal(t *T.T) {
	fi := &Field_I{}
	setVal(fi, 123)
	assert.Equal(t, int64(123), fi.I)

	fu := &Field_U{}
	setVal(fu, uint32(123))
	assert.Equal(t, uint64(123), fu.U)

	ff := &Field_F{}
	setVal(ff, float32(123))
	assert.Equal(t, float64(123), ff.F)

	fs := &Field_S{}
	setVal(fs, "hello")
	assert.Equal(t, "hello", fs.S)

	fd := &Field_D{}
	setVal(fd, []byte("world"))
	assert.Equal(t, []byte("world"), fd.D)
}
