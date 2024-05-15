package funcs

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/ptwindow"
	"github.com/GuanceCloud/cliutils/point"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	name, pl string
	in       []string
	expected []string

	commTags []string
	tagsVal  [][]string
	outkey   string
}

func TestPtWindow(t *testing.T) {
	cases := []testCase{
		{
			name: "test_pt_window",
			pl: `point_window(3,3)
			drop()
			if _ == "4" {
				window_hit()
			}
			`,
			in: []string{
				"1", "2", "3", "4", "5", "6",
			},
			commTags: []string{"host", "filepath"},
			tagsVal: [][]string{
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
			},
			expected: []string{
				"1", "2", "3", "4", "5", "6",
			},
			outkey: "message",
		},
		{
			name: "test_pt_window",
			pl: `point_window(3,3)
			if _ == "4" {
				window_hit()
			}
			if _ != "6" {
				drop()
			}
			`,
			in: []string{
				"1", "2", "3", "4", "5", "6", "7",
			},
			commTags: []string{"host", "filepath"},
			tagsVal: [][]string{
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"},
			},
			expected: []string{
				"1", "2", "3", "4", "5",
			},
			outkey: "message",
		},
		{
			name: "test_pt_window",
			pl: `point_window(3,3, ["host", "filepath"])
			if _ == "4" {
				window_hit()
			}
			if _ != "3" {
				drop()
			}
			`,
			in: []string{
				"1", "2", "3", "4", "5", "6", "7",
			},
			commTags: []string{"host", "filepath"},
			tagsVal: [][]string{
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"},
			},
			expected: []string{
				"1", "2", "4", "5", "6",
			},
			outkey: "message",
		},
		{
			name: "test_pt_window",
			pl: `point_window(3,3)
			if _ == "4" {
				window_hit()
			}
			if _ != "6" {
				drop()
			}
			if _ == "7" {
				window_hit()
			}
			`,
			in: []string{
				"1", "2", "3", "4", "5", "6", "7",
			},
			commTags: []string{"host", "filepath"},
			tagsVal: [][]string{
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"},
			},
			expected: []string{
				"1", "2", "3", "4", "5", "7",
			},
			outkey: "message",
		},

		{
			name: "test_pt_window1",
			pl: `point_window(3,3)
			drop()
			if _ == "4" {
				window_hit()
			}
			`,
			in: []string{
				"0", "1", "2", "3", "4", "5", "6",
			},
			commTags: []string{"host", "filepath"},
			tagsVal: [][]string{
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"}, {"a", "b"},
				{"a", "b"},
			},
			expected: []string{
				"3", "1", "2", "4", "5", "6",
			},
			outkey: "message",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ptPool := ptwindow.NewManager()
			runner, err := NewTestingRunner(c.pl)
			assert.NoError(t, err)
			r := []string{}
			for i, x := range c.in {
				tags := map[string]string{}
				for j, v := range c.commTags {
					tags[v] = c.tagsVal[i][j]
				}
				pt := ptinput.NewPlPoint(
					point.Logging, "test", tags, map[string]any{"message": x}, time.Now())
				pt.SetPtWinPool(ptPool)
				errR := runScript(runner, pt)
				if errR != nil {
					t.Fatal(errR)
				}

				pts := pt.CallbackPtWinMove()
				for _, pt := range pts {
					val := pt.Get("message")
					r = append(r, val.(string))
				}
			}

			assert.Equal(t, c.expected, r)
		})
	}
}
