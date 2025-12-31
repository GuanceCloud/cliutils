package aggregate

import (
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_alignNextWallTime(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		now := time.Unix(123, 0)
		wallTime := alignNextWallTime(now, time.Second*10).Unix()
		assert.Equal(t, int64(130), wallTime)

		wallTime = alignNextWallTime(now, time.Second).Unix()
		assert.Equal(t, int64(123), wallTime)
	})
}
