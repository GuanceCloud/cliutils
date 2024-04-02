// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package stats

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/point"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func TestPlChangeEvent(t *testing.T) {
	stats := NewRecStats("pl", "test", defaultLabelNames, 100)

	count := 0

	var g sync.WaitGroup
	g.Add(1)
	go func() {
		defer g.Done()
		for {
			if len(stats.event.ch) == 0 {
				time.Sleep(time.Millisecond * 10)
			} else {
				break
			}
		}
		for i := 0; i < 299; i++ {
			select {
			case <-stats.event.ReadChan():
				count++
			default:
			}
		}
	}()

	g.Add(1)
	go func() {
		defer g.Done()
		for i := 0; i < 199; i++ {
			stats.WriteEvent(&ChangeEvent{
				Name: fmt.Sprintf("%d.p", i),
				NS:   fmt.Sprintf("%d", i),
				Op:   EventOpAdd,
			}, nil)
		}
	}()
	g.Wait()

	assert.Less(t, 0, count)
	stats = NewRecStats("pl", "test", defaultLabelNames, 100)

	for i := 33; i < 256; i++ {
		tmp := []*ChangeEvent{}
		for j := 0; j < i%32; j++ {
			c := ChangeEvent{
				Name:     fmt.Sprint(i, ".p"),
				Category: point.Category(i % 32), // for testing only
				NS:       fmt.Sprint(i),
				Time:     time.Now(),
			}
			stats.WriteEvent(&c, nil)
			tmp = append(tmp, &c)
		}

		events := stats.event.Read(make([]*ChangeEvent, 0, i%32))
		assert.Equal(t, tmp, events)
	}
}

func TestCap(t *testing.T) {
	var a []string = nil
	assert.Equal(t, len(a), cap(a))
}

func TestMetric(t *testing.T) {
	lb := []string{"name", "extra1", "extra2"}

	// lb = append(lb, defaultLabelNames...)

	stats := NewRecStats("pl", "test", lb, 100)
	tags := map[string]string{
		"ns":       "test",
		"category": point.Metric.String(),
		"name":     "test",
		"extra1":   "test",
		"extra2":   "test",
	}
	stats.WriteMetric(tags, 1, 1, 1, time.Millisecond)
	stats.WriteUpdateTime(tags)

	m := stats.Metrics()
	ch := [](chan prometheus.Metric){}
	for i := 0; i < len(m); i++ {
		ch = append(ch, make(chan prometheus.Metric, 1))
	}

	for i, v := range m {
		v.Collect(ch[i])
	}

	for _, c := range ch {
		select {
		case c := <-c:
			m := dto.Metric{}
			c.Write(&m)
			assert.Equal(t, len(lb)+2, len(m.Label))
			mp := map[string]struct{}{}
			for _, v := range m.Label {
				mp[v.GetName()] = struct{}{}
			}
			mpExp := map[string]struct{}{}
			for _, v := range lb {
				mpExp[v] = struct{}{}
			}

			mpExp["ns"] = struct{}{}
			mpExp["category"] = struct{}{}

			assert.Equal(t, mpExp, mp)

			t.Log(m.String())
			t.Log(c.Desc())
		default:
		}
	}
}
