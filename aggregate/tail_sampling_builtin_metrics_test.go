package aggregate

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

func TestPacketTimePrefersPointTimestamp(t *testing.T) {
	now := time.Unix(0, 1773997434997000000)
	pt := point.NewPoint("trace", nil, point.WithTime(now))

	packet := &DataPacket{
		TraceEndTimeUnixNano: 1063559000,
		Points:               []*point.PBPoint{pt.PBPoint()},
	}

	assert.Equal(t, now.UnixNano(), packetTime(packet).UnixNano())
}
