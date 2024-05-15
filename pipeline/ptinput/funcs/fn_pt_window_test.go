package funcs

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
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
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
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

				errR := runScript(runner, pt)
				if errR != nil {
					t.Fatal(errR)
				}

				v, _, _ := pt.Get("message")
				r = append(r, v.(string))
			}

			assert.Equal(t, c.expected, r)
		})
	}
}
