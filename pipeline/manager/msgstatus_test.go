// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package manager

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

type pt4t struct {
	Fields map[string]interface{}
	Tags   map[string]string
	Drop   bool
}

func BenchmarkGTags(b *testing.B) {
	b.Run("test-list", func(b *testing.B) {
		s := [][2]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for idx := range s {
				_ = s[idx][0]
				_ = s[idx][1]
			}
		}
	})

	b.Run("test-map", func(b *testing.B) {
		s := map[string]string{
			"1": "",
			"2": "",
			"3": "",
			"4": "",
			"5": "",
			"6": "",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			for k, v := range s {
				_ = k
				_ = v
			}
		}
	})
}

func TestStatus(t *testing.T) {
	for k, v := range statusMap {
		outp := &pt4t{
			Fields: map[string]interface{}{
				FieldStatus: k,
			},
		}
		pt := ptinput.NewPlPoint(point.Logging, "", nil, outp.Fields, time.Now())
		ProcLoggingStatus(pt, false, nil)
		assert.Equal(t, v, pt.Fields()[FieldStatus])
	}

	{
		outp := &pt4t{
			Fields: map[string]interface{}{
				FieldStatus:  "x",
				FieldMessage: "1234567891011",
			},
		}
		pt := ptinput.NewPlPoint(point.Logging, "", nil, outp.Fields, time.Now())
		ProcLoggingStatus(pt, false, nil)
		assert.Equal(t, "unknown", pt.Fields()[FieldStatus])
		assert.Equal(t, "1234567891011", pt.Fields()[FieldMessage])
	}

	{
		outp := &pt4t{
			Fields: map[string]interface{}{
				FieldStatus:  "x",
				FieldMessage: "1234567891011",
			},
			Tags: map[string]string{
				"xxxqqqddd": "1234567891011",
			},
		}
		pt := ptinput.NewPlPoint(point.Logging, "", outp.Tags, outp.Fields, time.Now())
		ProcLoggingStatus(pt, false, nil)
		assert.Equal(t, map[string]interface{}{
			FieldStatus:  "unknown",
			FieldMessage: "1234567891011",
		}, pt.Fields())
		assert.Equal(t, map[string]string{
			"xxxqqqddd": "1234567891011",
		}, pt.Tags())
	}

	{
		outp := &pt4t{
			Fields: map[string]interface{}{
				FieldStatus:  "n",
				FieldMessage: "1234567891011",
			},
			Tags: map[string]string{
				"xxxqqqddd": "1234567891011",
			},
		}
		pt := ptinput.NewPlPoint(point.Logging, "", outp.Tags, outp.Fields, time.Now())
		ProcLoggingStatus(pt, false, nil)
		assert.Equal(t, map[string]interface{}{
			FieldStatus:  "notice",
			FieldMessage: "1234567891011",
		}, pt.Fields())
		assert.Equal(t, map[string]string{
			"xxxqqqddd": "1234567891011",
		}, pt.Tags())
	}
}

func TestGetSetStatus(t *testing.T) {
	out := &pt4t{
		Tags: map[string]string{
			"status": "n",
		},
		Fields: make(map[string]interface{}),
	}

	pt := ptinput.NewPlPoint(point.Logging, "", out.Tags, out.Fields, time.Now())
	ProcLoggingStatus(pt, false, nil)
	assert.Equal(t, map[string]string{
		"status": "notice",
	}, pt.Tags())
	assert.Equal(t, make(map[string]interface{}), pt.Fields())

	out.Fields = map[string]interface{}{
		"status": "n",
	}
	out.Tags = make(map[string]string)
	pt = ptinput.NewPlPoint(point.Logging, "", out.Tags, out.Fields, time.Now())

	ProcLoggingStatus(pt, false, nil)
	assert.Equal(t, map[string]interface{}{
		"status": "notice",
	}, pt.Fields())
	assert.Equal(t, make(map[string]string), pt.Tags())

	out.Fields = map[string]interface{}{
		"status": "n",
	}
	out.Tags = map[string]string{
		"status": "n",
	}

	pt = ptinput.NewPlPoint(point.Logging, "", out.Tags, out.Fields, time.Now())
	ProcLoggingStatus(pt, false, nil)
	assert.Equal(t, map[string]string{
		"status": "notice",
	}, pt.Tags())
	assert.Equal(t, map[string]interface{}{
		"status": "n",
	}, pt.Fields())

	pt = ptinput.NewPlPoint(point.Logging, "", out.Tags, out.Fields, time.Now())
	ProcLoggingStatus(pt, false, []string{"notice"})
	assert.Equal(t, map[string]string{
		"status": "notice",
	}, pt.Tags())
	assert.Equal(t, map[string]interface{}{
		"status": "n",
	}, pt.Fields())
}
