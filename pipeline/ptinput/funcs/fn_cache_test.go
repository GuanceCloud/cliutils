package funcs

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/pipeline/ptinput/plcache"
	"github.com/GuanceCloud/cliutils/point"
	tu "github.com/GuanceCloud/cliutils/testutil"
)

func TestCache(t *testing.T) {
	cases := []struct {
		name, pl, in string
		expected     interface{}
		fail         bool
		outkey       string
	}{
		{
			name: "test_set_get_with_exp",
			pl: `cache_set("a", "123", 5)
			a = cache_get("a")
			add_key(abc, a)`,
			in:       `[]`,
			expected: "123",
			outkey:   "abc",
		},
		{
			name: "test_set_get_without_exp",
			pl: `a = cache_set("a", "123")
			a = cache_get("a")
			add_key(abc, a)`,
			in:       `[]`,
			expected: "123",
			outkey:   "abc",
		},
	}

	for idx, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner, err := NewTestingRunner(tc.pl)
			if err != nil {
				if tc.fail {
					t.Logf("[%d]expect error: %s", idx, err)
				} else {
					t.Errorf("[%d] failed: %s", idx, err)
				}
				return
			}
			cache, _ := plcache.NewCache(time.Second, 100)
			pt := ptinput.NewPlPoint(
				point.Logging, "test", nil, map[string]any{"message": tc.in}, time.Now())
			pt.SetCache(cache)
			errR := runScript(runner, pt)

			if errR != nil {
				t.Fatal(errR.Error())
			}

			cache.Stop()

			v, _, _ := pt.Get(tc.outkey)
			// tu.Equals(t, nil, err)
			tu.Equals(t, tc.expected, v)

			t.Logf("[%d] PASS", idx)
		})
	}
}
