package funcs

import (
	"testing"
	"time"

	"github.com/GuanceCloud/cliutils/pipeline/ptinput"
	"github.com/GuanceCloud/cliutils/point"
	tu "github.com/GuanceCloud/cliutils/testutil"
)

func TestHTTPRequest(t *testing.T) {
	cases := []struct {
		name, pl, in string
		expected     interface{}
		fail         bool
		outkey       string
	}{
		{
			name: "acquire_code",
			pl: `resp = http_request("GET", "http://localhost:8080/testResp")
			add_key(abc, resp["status_code"])`,
			in:       `[]`,
			expected: int64(200),
			outkey:   "abc",
		},
		{
			name: "acquire_body_without_headers",
			pl: `resp = http_request("GET", "http://localhost:8080/testResp")
			resp_body = load_json(resp["body"])
			add_key(abc, resp_body["a"])`,
			in:       `[]`,
			expected: float64(11.1),
			outkey:   `abc`,
		},
		{
			name: "acquire_body_with_headers",
			pl: `resp = http_request("GET", "http://localhost:8080/testResp", {"extraHeader1": "1", "extraHeader2": "1"})
			resp_body = load_json(resp["body"])
			add_key(abc, resp_body["a"])`,
			in:       `[]`,
			expected: float64(11.111),
			outkey:   `abc`,
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
			pt := ptinput.NewPlPoint(
				point.Logging, "test", nil, map[string]any{"message": tc.in}, time.Now())
			errR := runScript(runner, pt)

			if errR != nil {
				t.Fatal(errR.Error())
			}

			v, _, _ := pt.Get(tc.outkey)
			// tu.Equals(t, nil, err)
			tu.Equals(t, tc.expected, v)

			t.Logf("[%d] PASS", idx)
		})
	}
}
