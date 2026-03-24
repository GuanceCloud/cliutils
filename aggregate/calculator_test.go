package aggregate

import (
	T "testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_alignNextWallTime(t *T.T) {
	t.Run(`basic`, func(t *T.T) {
		now := time.Unix(123, 0)
		wallTime := AlignNextWallTime(now, time.Second*10)
		assert.Equal(t, int64(130), wallTime)

		wallTime = AlignNextWallTime(now, time.Second)
		assert.Equal(t, int64(123), wallTime)
	})
}

func TestQuantile(t *T.T) {
	var q float64 = 0.95
	t.Logf("quantile %f", q)
	t.Logf("quantile int %0.0f", q*100)
}
